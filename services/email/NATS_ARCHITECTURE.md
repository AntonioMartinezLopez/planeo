# Email Service - NATS Job Queue Architecture

## Overview

The email service now supports scalable, distributed email processing using NATS JetStream. It can run in three modes:

### Modes

1. **`publisher`** - Schedules email checks and publishes jobs to NATS (lightweight)
2. **`worker`** - Processes email check jobs from NATS (horizontally scalable)
3. **`both`** - Legacy mode (default for backward compatibility)

## Architecture

```
┌──────────────────┐
│   Database       │ ← Source of truth for email accounts
└────────┬─────────┘
         │
┌────────▼──────────┐
│  Publisher(s)     │ ← 1-2 instances, schedules checks
│  MODE=publisher   │    Publishes to NATS
└────────┬──────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│  NATS JetStream Queue                   │
│  Subject: jobs.email.check              │
│  Consumer: email-check-workers          │
└────────┬────────────────────────────────┘
         │
    ┌────┴────┬─────────┬──────────┐
    ▼         ▼         ▼          ▼
┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
│Worker 1│ │Worker 2│ │Worker 3│ │Worker N│ ← Scale these!
│MODE=   │ │MODE=   │ │MODE=   │ │MODE=   │   Process emails
│worker  │ │worker  │ │worker  │ │worker  │
└────────┘ └────────┘ └────────┘ └────────┘
```

## Configuration

Set the `MODE` environment variable:

### Publisher Instance (.env)
```env
MODE=publisher
HOST=0.0.0.0
PORT=8081
NATS_URL=nats://nats:4222
DB_HOST=postgres
DB_PORT=5432
DB_USER=planeo
DB_PASSWORD=planeo
DB_NAME=planeo_email
KC_BASE_URL=http://keycloak:8080
KC_ISSUER_REALM=planeo
KC_OAUTH_CLIENT_ID=planeo-client
```

### Worker Instance (.env.worker)
```env
MODE=worker
HOST=0.0.0.0
PORT=8082
NATS_URL=nats://nats:4222
DB_HOST=postgres
DB_PORT=5432
DB_USER=planeo
DB_PASSWORD=planeo
DB_NAME=planeo_email
KC_BASE_URL=http://keycloak:8080
KC_ISSUER_REALM=planeo
KC_OAUTH_CLIENT_ID=planeo-client
```

## Deployment

### Docker Compose (Development)

```yaml
services:
  email-publisher:
    build: ./services/email
    environment:
      MODE: publisher
    env_file:
      - .env
    ports:
      - "8081:8081"
    depends_on:
      - postgres
      - nats
    restart: unless-stopped

  email-worker:
    build: ./services/email
    environment:
      MODE: worker
    env_file:
      - .env
    deploy:
      replicas: 3  # Scale workers
    depends_on:
      - postgres
      - nats
    restart: unless-stopped
```

### Kubernetes (Production)

#### Publisher Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: email-publisher
spec:
  replicas: 2  # For HA
  selector:
    matchLabels:
      app: email-publisher
  template:
    metadata:
      labels:
        app: email-publisher
    spec:
      containers:
      - name: email-publisher
        image: planeo/email-service:latest
        env:
        - name: MODE
          value: "publisher"
        envFrom:
        - configMapRef:
            name: email-config
        - secretRef:
            name: email-secrets
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
```

#### Worker Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: email-worker
spec:
  replicas: 5  # Scale based on load
  selector:
    matchLabels:
      app: email-worker
  template:
    metadata:
      labels:
        app: email-worker
    spec:
      containers:
      - name: email-worker
        image: planeo/email-service:latest
        env:
        - name: MODE
          value: "worker"
        envFrom:
        - configMapRef:
            name: email-config
        - secretRef:
            name: email-secrets
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "512Mi"
            cpu: "500m"
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: email-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: email-worker
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## Migration Path

### Phase 1: Run Both Modes (No Downtime)
```env
MODE=both  # Default - keeps old behavior
```
This runs both the old gocron scheduler and new NATS publisher/worker.

### Phase 2: Split Deployment
1. Deploy 1 publisher instance
2. Deploy 3+ worker instances
3. Verify jobs are being processed
4. Monitor NATS queue metrics

### Phase 3: Decommission Legacy
Once confident, switch to split mode:
- Set publisher: `MODE=publisher`
- Set workers: `MODE=worker`
- Remove old gocron code (optional cleanup)

## Monitoring

### NATS Metrics
```bash
# Check stream status
nats stream info EVENTS

# Monitor consumer
nats consumer info EVENTS email-check-workers

# View pending jobs
nats stream view EVENTS --subject "jobs.email.check"
```

### Application Logs
Publishers log:
- `Published email check job` (debug level)
- `Started publishing jobs for account`
- `Stopped publishing jobs for account`

Workers log:
- `Processing email check job`
- `Fetched emails successfully`
- `Failed to fetch emails` (with retry)

## Benefits

✅ **Horizontal Scaling** - Add more workers to handle more accounts  
✅ **Resilience** - Jobs persist in NATS if workers crash  
✅ **Load Distribution** - NATS automatically distributes jobs  
✅ **Retry Logic** - Failed jobs automatically retry with backoff  
✅ **No Duplicate Processing** - Work queue semantics ensure exactly-once  
✅ **Zero Downtime Deployments** - Workers can be updated rolling  

## Future Enhancements

- [ ] Dynamic interval adjustment based on email volume
- [ ] Rate limiting per email provider (Gmail, Outlook, etc.)
- [ ] Priority queues for premium organizations
- [ ] Dead letter queue for persistent failures
- [ ] Prometheus metrics for job processing
