# Local Testing Guide

## Testing the NATS Job Queue

### 1. Start Infrastructure

```bash
# From planeo root
task up
```

### 2. Publisher Mode (Terminal 1)

```bash
cd services/email
MODE=publisher task run:email
```

This instance will:
- Load email account settings from DB
- Schedule checks every 10 seconds
- Publish jobs to NATS `jobs.email.check`
- Expose API on port 8081 for managing settings

### 3. Worker Mode (Terminal 2)

```bash
cd services/email
MODE=worker PORT=8082 task run:email
```

This instance will:
- Subscribe to NATS `jobs.email.check`
- Process email check jobs
- Publish `events.email.received` for found emails
- No API endpoints (worker only)

### 4. Add More Workers (Terminal 3)

```bash
cd services/email
MODE=worker PORT=8083 task run:email
```

You'll see jobs distributed between workers!

### 5. Monitor NATS

```bash
# Install NATS CLI if not already installed
brew install nats-io/nats-tools/nats

# View stream
nats stream ls

# Monitor jobs
nats stream view EVENTS --subject "jobs.email.check"

# Check consumer
nats consumer info EVENTS email-check-workers
```

### 6. Create Test Email Account

```bash
# Get admin token
task login

# Create email setting via API (publisher instance)
curl -X POST http://localhost:8081/api/organizations/1/settings \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "host": "imap.gmail.com",
    "port": 993,
    "username": "test@example.com",
    "password": "app-password"
  }'
```

### Expected Behavior

**Publisher logs:**
```
INFO  Started publishing jobs for account setting_id=1 organization_id=1 interval=10s
DEBUG Published email check job setting_id=1 organization_id=1
```

**Worker logs:**
```
DEBUG Processing email check job setting_id=1 organization_id=1 host=imap.gmail.com
INFO  Fetched emails successfully email_count=3 duration_ms=1234
DEBUG Publishing email received event message_id=abc123
```

## Performance Testing

### Load Test with Multiple Accounts

```bash
# Create 100 test accounts
for i in {1..100}; do
  curl -X POST http://localhost:8081/api/organizations/1/settings \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"host\": \"imap.test$i.com\",
      \"port\": 993,
      \"username\": \"test$i@example.com\",
      \"password\": \"password\"
    }"
done
```

### Monitor Job Processing

```bash
# Watch NATS queue depth
watch -n 1 'nats stream info EVENTS | grep -A 5 "State"'

# Watch worker logs
tail -f /tmp/email-worker-*.log
```

### Scale Workers

Start 5 workers in different terminals:
```bash
for port in {8082..8086}; do
  MODE=worker PORT=$port task run:email &
done
```

## Troubleshooting

### Jobs Not Being Processed

1. Check NATS connection:
```bash
nats stream ls
```

2. Verify consumer is created:
```bash
nats consumer ls EVENTS
```

3. Check for pending messages:
```bash
nats stream view EVENTS --subject "jobs.email.check"
```

### Jobs Stuck in Queue

Check worker logs for errors:
```bash
grep -i error /tmp/email-worker-*.log
```

### Publisher Not Sending Jobs

Verify database has email settings:
```sql
psql -U planeo -d planeo_email -c "SELECT * FROM settings;"
```

## Cleanup

```bash
# Stop all background workers
killall email

# Clear NATS stream
nats stream purge EVENTS --subject "jobs.email.check" --force

# Reset database
task down
task up
```
