# NATS Job Queue Implementation - Summary

## What Was Implemented

I've transformed your email service from a single-instance cron scheduler to a **production-grade, horizontally scalable** job queue architecture using NATS JetStream (which you already have in your stack).

## The Answer to Your Question

> "How would you actually trigger the publishing? Maybe by providing an endpoint and using the environment, e.g. kubernetes, to create a cronjob that calls this endpoint?"

**My Answer:** **Hybrid approach - lightweight in-process scheduler that publishes to NATS**

This is better than K8s CronJobs because:
- ✅ No need to manage thousands of CronJob manifests
- ✅ Dynamic intervals can change in code without K8s updates
- ✅ Simple deployment - just run the service in different modes
- ✅ Publisher is stateless and lightweight (just scheduling, not doing work)

## Architecture Components

### 1. **Job Publisher** ([job_publisher.go](services/email/internal/job_publisher.go))
- Lightweight scheduler that runs one goroutine per email account
- Publishes `jobs.email.check` messages to NATS every 10 seconds
- Can be deployed with 1-2 instances for HA
- Uses minimal resources (just timing + publishing)

### 2. **Job Worker** ([job_worker.go](services/email/internal/job_worker.go))
- Subscribes to NATS `jobs.email.check` queue
- Fetches emails from IMAP servers
- Publishes `events.email.received` for found emails
- **Horizontally scalable** - deploy 10, 100, 1000 workers as needed
- NATS ensures each job goes to exactly one worker

### 3. **NATS Events** ([email_check_job.go](libs/events/email_check_job.go))
- New event type: `EmailCheckJobPayload`
- Work queue semantics (not broadcast)
- Auto-retry on failure with NAK/delay

## How to Use

### Development (Current - No Breaking Changes)

```bash
# Default mode - keeps old behavior
task run:email
```

The `MODE` env var defaults to `"both"` which runs the legacy gocron scheduler for backward compatibility.

### Testing the New Architecture

**Terminal 1 - Publisher:**
```bash
cd services/email
MODE=publisher task run:email
```

**Terminal 2 - Worker:**
```bash
MODE=worker PORT=8082 task run:email
```

**Terminal 3 - More Workers:**
```bash
MODE=worker PORT=8083 task run:email
MODE=worker PORT=8084 task run:email
```

You'll see:
- Publisher logs: `Published email check job`
- Workers logs: `Processing email check job` distributed across instances

### Production Deployment

**Option 1: Docker Compose**
```yaml
# Add to dev/docker-compose.yaml
email-publisher:
  build: ./services/email
  environment:
    MODE: publisher
  env_file: services/email/.env
  ports: ["8081:8081"]

email-worker:
  build: ./services/email
  environment:
    MODE: worker
  env_file: services/email/.env
  deploy:
    replicas: 5  # Scale this!
```

**Option 2: Kubernetes**
See [NATS_ARCHITECTURE.md](services/email/NATS_ARCHITECTURE.md) for full deployment manifests with HPA (auto-scaling).

## Benefits vs Current Implementation

| Feature | Current (gocron) | With NATS Queue |
|---------|------------------|-----------------|
| Horizontal Scaling | ❌ No (duplicate work) | ✅ Yes (auto-distributed) |
| Max Concurrent Jobs | 20 | Unlimited (add workers) |
| Job Persistence | ❌ Lost on restart | ✅ Persisted in NATS |
| Retry Logic | ❌ Wait for next cycle | ✅ Exponential backoff |
| Single Point of Failure | ❌ Yes | ✅ No (workers are stateless) |
| Zero Downtime Deploy | ❌ Hard | ✅ Rolling worker updates |
| Resource Efficiency | ❌ All-in-one | ✅ Scale workers independently |

## Migration Path (Zero Downtime)

### Phase 1: Verify (Week 1)
```bash
MODE=both  # Default - runs both old and new systems
```
Monitor that NATS jobs are being created and processed.

### Phase 2: Split Deployment (Week 2)
Deploy with:
- 1x publisher instance (`MODE=publisher`)
- 3x worker instances (`MODE=worker`)

Verify all email accounts are being checked.

### Phase 3: Production (Week 3+)
- Switch K8s/Docker to split mode
- Enable HPA for workers (auto-scale 3-20 based on load)
- Monitor NATS metrics

### Phase 4: Cleanup (Optional)
Remove old code:
- Delete [cron.go](services/email/internal/cron.go)
- Delete [email.go](services/email/internal/email.go)
- Remove `MODE=both` support

## Files Changed/Created

**New Files:**
- `libs/events/email_check_job.go` - Job event definition
- `services/email/internal/job_publisher.go` - Scheduler (publishes jobs)
- `services/email/internal/job_worker.go` - Worker (processes jobs)
- `services/email/internal/email_scheduler_service.go` - Adapter for settings service
- `services/email/NATS_ARCHITECTURE.md` - Full architecture docs
- `services/email/TESTING.md` - Local testing guide

**Modified Files:**
- `services/email/config/config.go` - Added `MODE` config + helper methods
- `libs/events/service.go` - Added job publish/subscribe interface
- `services/email/internal/email.go` - Updated interface
- `services/email/internal/setup/app_factory.go` - Wiring for new components

## Next Steps

1. **Test locally** - Follow [TESTING.md](services/email/TESTING.md)
2. **Deploy to staging** with `MODE=both` to verify
3. **Monitor NATS** queue depth and processing times
4. **Scale workers** based on actual load

## Future Enhancements (TODO)

The code includes TODOs for:
- ✨ Dynamic intervals based on email volume
- ✨ Rate limiting per provider (Gmail, Outlook limits)
- ✨ Priority queues (premium orgs get faster checks)
- ✨ Dead letter queue for persistent failures
- ✨ Prometheus metrics

## Questions?

Check the architecture docs:
- [NATS_ARCHITECTURE.md](services/email/NATS_ARCHITECTURE.md) - Full deployment guide
- [TESTING.md](services/email/TESTING.md) - Local development workflow
