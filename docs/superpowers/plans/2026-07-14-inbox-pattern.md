# Inbox Pattern for services/core's Kafka Consumer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `services/core`'s Kafka consumer at-least-once delivery with idempotency by durably persisting each `email-received` message before acking it, then processing it asynchronously — the consumer-side mirror of `services/email`'s already-shipped outbox pattern.

**Architecture:** A new reusable `libs/inbox` engine (mirroring `libs/outbox`) provides a `Consumer` (persists Kafka records into an `inbox` table, commits offset only after persist succeeds) and a `Worker` (polls the table, claims rows atomically, invokes an injected `Handler`, retries/quarantines on failure). Both run as goroutines inside one new binary, `services/core/cmd/email-received-consumer`, which fully replaces core's current inline, non-idempotent Kafka subscription. As a preliminary step, the existing `outbox-relay` sidecar is renamed to `email-received-producer` to establish an explicit `<topic-name>-producer`/`<topic-name>-consumer` naming convention before any inbox code lands.

**Tech Stack:** Go, franz-go (`kgo`), pgx/v5, goose, testify.

## Global Constraints

- Reference spec: `docs/superpowers/specs/2026-07-14-inbox-pattern-design.md`.
- Branch: `feature/migrate-nats-to-kafka` (already checked out). Do not commit to `main`.
- Task 1's rename is naming/identity only — no functional change to `services/email`'s outbox pattern, schema, or relay logic.
- No redesign of Kafka consumer-group semantics beyond per-binary configurable, collision-safe-by-default group names (see Task 7).
- No generic multi-topic router — `email-received-consumer` consumes exactly one topic. A future service consuming a different topic gets its own dedicated binary.
- No shared claim/retry engine, and no shared `Record` type, between `libs/outbox` and `libs/inbox` — kept as two independently-implemented packages.
- Section 9 of the spec (per-key ordering in `Worker`) is explicitly out of scope for this plan — not designed, not implemented here.
- `libs/events` (the `service.go`/`email_received.go` pair — not the separate `libs/events/contracts` subpackage) is deleted wholesale in Task 2 as dead code (verified zero callers anywhere in the repo once Task 6 lands — `services/email`'s outbox relay never used it, and `services/core` is its only other caller). `inbox.Consumer` (Task 3) owns its own `kgo` consumer-group client directly, mirroring `outbox.Producer`'s existing independence from `libs/events`.
- `services/core`'s `ApplicationConfiguration.KafkaBrokers` field is removed as dead code in Task 6 (its only caller, `cmd/main.go`'s `InitializeEvents` call, is removed in that same task).
- `requests.reference_id` gets a partial unique index in Task 5 (`WHERE reference_id <> ''`), and `CreateRequest` becomes idempotent on `(organization_id, reference_id)` — closing a duplicate-`Request` risk if `MarkProcessed` fails after a successful handler run and the record gets reprocessed after `claimTTL` expires. Requests without a `reference_id` (created via the REST API) are unaffected.

---

### Task 1: Rename `outbox-relay` → `email-received-producer`

**Files:**
- Move: `services/email/cmd/outbox-relay/main.go` → `services/email/cmd/email-received-producer/main.go`
- Move: `services/email/cmd/outbox-relay/config.go` → `services/email/cmd/email-received-producer/config.go`
- Move: `services/email/Dockerfile.outbox-relay` → `services/email/Dockerfile.email-received-producer`
- Modify: `Taskfile.yml`
- Modify: `dev/docker-compose.yaml`

**Interfaces:**
- No Go types change shape. `outbox.Relay`, `outbox.NewProducer`, `postgres.NewClient` are all used identically to before — only file paths, env var names, and identifiers change.

- [ ] **Step 1: Move the folder and update its contents**

```bash
git mv services/email/cmd/outbox-relay services/email/cmd/email-received-producer
git mv services/email/Dockerfile.outbox-relay services/email/Dockerfile.email-received-producer
```

Replace `services/email/cmd/email-received-producer/main.go` with:

```go
package main

import (
	"context"
	"os/signal"
	"planeo/libs/logger"
	"planeo/libs/outbox"
	"planeo/services/email/internal/infra/postgres"
	"strings"
	"syscall"
)

func main() {
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	log := logger.New("email-received-producer")
	ctx := logger.WithContext(context.Background(), log)

	log.Info().Msg("Loading environment variables")
	cfg := LoadConfig(ctx)

	db := postgres.NewClient(ctx, cfg.DatabaseConfig())
	defer db.Close()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	producer, kafkaClient, err := outbox.NewProducer(brokers)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}
	defer kafkaClient.Close()

	relay := outbox.NewRelay(db, producer,
		outbox.WithPollInterval(cfg.PollInterval),
		outbox.WithBatchSize(cfg.BatchSize),
		outbox.WithMaxAttempts(cfg.MaxAttempts),
		outbox.WithClaimTTL(cfg.ClaimTTL),
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Outbox relay running")
	if err := relay.Run(runCtx); err != nil {
		log.Info().Err(err).Msg("Outbox relay stopped")
	}
}
```

(Only the `logger.New(...)` argument changed, from `"outbox-relay"` to `"email-received-producer"` — this is the structured-logger service tag. Log message text is unchanged, per the spec's scope: naming covers deployable identity, not narrative log strings.)

Replace `services/email/cmd/email-received-producer/config.go` with:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"planeo/libs/logger"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DbHost       string
	DbPort       string
	DbUser       string
	DbPassword   string
	DbName       string
	KafkaBrokers string
	PollInterval time.Duration
	BatchSize    int
	MaxAttempts  int
	ClaimTTL     time.Duration
}

func (c *Config) DatabaseConfig() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DbUser, c.DbPassword, c.DbHost, c.DbPort, c.DbName)
}

func LoadConfig(ctx context.Context, filenames ...string) *Config {
	if err := godotenv.Load(filenames...); err != nil {
		l := logger.FromContext(ctx)
		l.Warn().Err(err).Msg("Error loading .env file")
	}

	return &Config{
		DbHost:       readEnvVariable(ctx, "DB_HOST"),
		DbPort:       readEnvVariable(ctx, "DB_PORT"),
		DbUser:       readEnvVariable(ctx, "DB_USER"),
		DbPassword:   readEnvVariable(ctx, "DB_PASSWORD"),
		DbName:       readEnvVariable(ctx, "DB_NAME"),
		KafkaBrokers: readEnvVariable(ctx, "KAFKA_BROKERS"),
		PollInterval: readDurationEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_POLL_INTERVAL", 1*time.Second),
		BatchSize:    readIntEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_BATCH_SIZE", 100),
		MaxAttempts:  readIntEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_MAX_ATTEMPTS", 5),
		ClaimTTL:     readDurationEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_CLAIM_TTL", 30*time.Second),
	}
}

func readEnvVariable(ctx context.Context, name string) string {
	v, ok := os.LookupEnv(name)
	if !ok {
		l := logger.FromContext(ctx)
		l.Fatal().Msgf("Missing env variable '%s'. Aborting...\n", name)
	}
	return v
}

func readDurationEnvVariable(ctx context.Context, name string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Fatal().Err(err).Msgf("Invalid duration for env variable '%s'", name)
	}
	return d
}

func readIntEnvVariable(ctx context.Context, name string, def int) int {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Fatal().Err(err).Msgf("Invalid integer for env variable '%s'", name)
	}
	return n
}
```

(Only the four `OUTBOX_*` env var name strings changed to `EMAIL_RECEIVED_PRODUCER_*` — struct field names, function names, and all logic are unchanged.)

Replace `services/email/Dockerfile.email-received-producer` with:

```dockerfile
# syntax=docker/dockerfile:1

FROM golang:1.26.5-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/email-received-producer ./services/email/cmd/email-received-producer

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/email-received-producer /email-received-producer
ENTRYPOINT ["/email-received-producer"]
```

- [ ] **Step 2: Update `Taskfile.yml`**

Change:
```yaml
  build:email:outbox-relay:
    desc: Build Docker image for email outbox-relay sidecar
    cmds:
      - echo "Building Docker image for email-outbox-relay with tag {{.VERSION}}..."
      - docker build -t {{.DOCKER_REGISTRY}}/email-outbox-relay:{{.VERSION}} -f services/email/Dockerfile.outbox-relay .
```
to:
```yaml
  build:email:email-received-producer:
    desc: Build Docker image for email-received-producer sidecar
    cmds:
      - echo "Building Docker image for email-received-producer with tag {{.VERSION}}..."
      - docker build -t {{.DOCKER_REGISTRY}}/email-received-producer:{{.VERSION}} -f services/email/Dockerfile.email-received-producer .
```

Change the `build:all` task's reference from `build:email:outbox-relay` to `build:email:email-received-producer`:
```yaml
  build:all:
    desc: Build Docker images for all services
    cmds:
      - task: build:core
      - task: build:email
      - task: build:email:email-received-producer
```

- [ ] **Step 3: Update `dev/docker-compose.yaml`**

Change:
```yaml
  # email outbox relay sidecar - drains services/email's outbox table to Kafka
  # NOTE: On first `task up`, this container will log query errors for a few seconds
  # while waiting for the outbox table to be created by migrations. This is expected
  # and not a regression — the poll loop continues and self-recovers once the table exists.
  email-outbox-relay:
    container_name: email-outbox-relay
    build:
      context: ..
      dockerfile: services/email/Dockerfile.outbox-relay
    depends_on:
      - postgres
      - kafka
    environment:
      DB_HOST: postgres
      DB_PORT: "5432"
      DB_USER: planeo
      DB_PASSWORD: planeo
      DB_NAME: mail
      KAFKA_BROKERS: kafka:19092
      OUTBOX_POLL_INTERVAL: 1s
      OUTBOX_BATCH_SIZE: "100"
      OUTBOX_MAX_ATTEMPTS: "5"
      OUTBOX_CLAIM_TTL: 30s
```
to:
```yaml
  # email-received producer sidecar - drains services/email's outbox table to Kafka
  # NOTE: On first `task up`, this container will log query errors for a few seconds
  # while waiting for the outbox table to be created by migrations. This is expected
  # and not a regression — the poll loop continues and self-recovers once the table exists.
  email-received-producer:
    container_name: email-received-producer
    build:
      context: ..
      dockerfile: services/email/Dockerfile.email-received-producer
    depends_on:
      - postgres
      - kafka
    environment:
      DB_HOST: postgres
      DB_PORT: "5432"
      DB_USER: planeo
      DB_PASSWORD: planeo
      DB_NAME: mail
      KAFKA_BROKERS: kafka:19092
      EMAIL_RECEIVED_PRODUCER_POLL_INTERVAL: 1s
      EMAIL_RECEIVED_PRODUCER_BATCH_SIZE: "100"
      EMAIL_RECEIVED_PRODUCER_MAX_ATTEMPTS: "5"
      EMAIL_RECEIVED_PRODUCER_CLAIM_TTL: 30s
```

- [ ] **Step 4: Verify it builds**

Run: `go build ./services/email/...`
Expected: exit 0.

Run: `gofmt -l services/email/cmd/email-received-producer/`
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add services/email/cmd/email-received-producer services/email/Dockerfile.email-received-producer Taskfile.yml dev/docker-compose.yaml
git commit -m "refactor(email): rename outbox-relay to email-received-producer"
```

---

### Task 2: Delete `libs/events` wholesale (dead code)

**Files:**
- Delete: `libs/events/service.go`
- Delete: `libs/events/email_received.go`

**Interfaces:**
- Produces: nothing — this task only removes code. `libs/events/contracts` (a separate subpackage) is untouched and unaffected.
- **This task deliberately breaks `services/core/internal/infra/events/events.go`**, which calls `events.NewEventService(...)` and `eventService.SubscribeEmailReceived(...)` — both now gone. That break is expected and resolved by Task 6 — do not attempt to fix `services/core` here.

`libs/events.Publish`/`PublishEmailReceived` already have zero callers anywhere in the repo (`services/email`'s outbox relay never used `libs/events` — `outbox.NewProducer` owns its own `kgo.Client` directly). Once Task 6 removes `services/core`'s only remaining call into this package (`InitializeEvents`/`SubscribeEmailReceived`), `Subscribe`/`IsConnected`/`NewEventService`/`Close` lose their last caller too. Rather than trim dead functions and leave an empty-purpose package behind, the whole package is deleted here — `inbox.Consumer` (Task 3) implements its own Kafka consumer-group client directly, the same way `outbox.Producer` already does for producing.

- [ ] **Step 1: Delete both files**

```bash
git rm libs/events/service.go libs/events/email_received.go
```

- [ ] **Step 2: Verify the expected, isolated compile break**

Run: `go build ./libs/events/...`
Expected: exit 0 — only `libs/events/contracts` remains, and it has no dependency on the deleted files, so it compiles standalone. (Verify with `ls libs/events/` — only the `contracts/` subdirectory should remain at this level.)

Run: `go build ./services/core/...`
Expected: exit 1, with the error confined to `services/core/internal/infra/events`:
```
services/core/internal/infra/events/events.go:XX: package planeo/libs/events is not in std
```
(or similar — the import no longer resolves to anything, since no `.go` files remain directly in `libs/events/`). This is the deliberate break described above — resolved by Task 6. Do not modify any file in `services/core` to work around it in this task.

Run: `go build ./services/email/...`
Expected: exit 0 — `services/email` never imported `libs/events` (confirmed: only `services/core/internal/infra/events/events.go` imports `planeo/libs/events` anywhere in the repo), so it's unaffected by this task.

- [ ] **Step 3: Commit**

```bash
git rm libs/events/service.go libs/events/email_received.go
git commit -m "refactor(events): delete libs/events - dead code, inbox.Consumer owns its own kgo client"
```

---

### Task 3: `libs/inbox` package

**Files:**
- Create: `libs/inbox/store.go`
- Create: `libs/inbox/consumer.go`
- Create: `libs/inbox/worker.go`
- Create: `libs/inbox/worker_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks — this task is fully self-contained (Task 2 only deletes code, it doesn't produce anything `libs/inbox` needs).
- Produces: `inbox.Record{ID int64, Topic string, Payload []byte}`, `inbox.Store{Save, FetchBatch, MarkProcessed, MarkFailed}`, `inbox.Handler func(ctx context.Context, record Record) error`, `inbox.NewConsumer(brokers []string, groupName, topic string, store Store) *Consumer`, `inbox.NewWorker(store Store, handler Handler, opts ...Option) *Worker` — consumed by Task 4 (`services/core`'s `Store` implementation) and Task 7 (`email-received-consumer`'s wiring).
- This task is self-contained and independently testable — it does not depend on Task 4's Postgres implementation, and its own unit tests use a fake `Store`. `Consumer` owns its own `kgo.Client` directly (mirroring `outbox.Producer`'s independence from `libs/events`), so this package has no dependency on any other `planeo`-internal package besides `libs/logger`.

- [ ] **Step 1: Create `libs/inbox/store.go`**

```go
package inbox

import (
	"context"
	"time"
)

// Record is one durably-persisted inbox row, ready for processing.
type Record struct {
	ID      int64
	Topic   string
	Payload []byte
}

// Store is implemented per-service against that service's own inbox table.
// Mirrors outbox.Store's shape, for the opposite direction.
type Store interface {
	// Save durably persists one raw Kafka record, deduped on
	// (topic, partition, offset). inserted=false on a duplicate — a safe
	// no-op, not an error. The caller commits the Kafka offset regardless
	// of inserted's value, since either way this offset is now durably
	// known.
	Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (inserted bool, err error)

	// FetchBatch atomically claims up to limit pending (or expired-claim)
	// records in insertion order. Same FOR UPDATE SKIP LOCKED
	// atomic-UPDATE-with-inner-SELECT requirement as outbox.Store.
	FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]Record, error)

	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error
}
```

- [ ] **Step 2: Create `libs/inbox/consumer.go`**

```go
package inbox

import (
	"context"

	"planeo/libs/logger"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Consumer reads from Kafka and persists into the inbox, committing the
// offset only after Save succeeds. No Handler is invoked here. Owns its
// own kgo consumer-group client directly — mirrors outbox.Producer's
// independence from any other planeo-internal package.
type Consumer struct {
	brokers   []string
	groupName string
	topic     string
	store     Store
}

func NewConsumer(brokers []string, groupName, topic string, store Store) *Consumer {
	return &Consumer{
		brokers:   brokers,
		groupName: groupName,
		topic:     topic,
		store:     store,
	}
}

// Run subscribes to the configured topic and persists each fetched record
// into the inbox, deduped on the Kafka coordinate, committing the offset
// only after a successful Save. Non-blocking — starts its own background
// goroutine and returns once the subscription is established (or fails to
// start).
func (c *Consumer) Run(ctx context.Context) error {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(c.brokers...),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup(c.groupName),
		kgo.ConsumeTopics(c.topic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)

	go func() {
		defer client.Close()

		for {
			if ctx.Err() != nil {
				return
			}

			fetches := client.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return
			}

			fetches.EachError(func(_ string, _ int32, err error) {
				log.Error().Err(err).Msg("kafka fetch error")
			})

			// Kafka offset commits are a single cumulative per-partition
			// marker, not a per-record acknowledgement: if record N's Save
			// fails but record N+1's Save (and commit) succeeds, the
			// committed offset would advance past N, and N would never be
			// redelivered — a silent, permanent loss of exactly the message
			// this pattern exists to protect. To prevent that, once a
			// partition has a failure in this poll, stop committing further
			// records on that partition for the rest of the poll (Save's
			// (topic, partition, offset) dedup makes it safe to reprocess
			// already-saved records on the next poll/redelivery).
			failedPartitions := map[int32]bool{}
			fetches.EachRecord(func(record *kgo.Record) {
				if failedPartitions[record.Partition] {
					return
				}

				if _, err := c.store.Save(ctx, c.topic, record.Partition, record.Offset, record.Value); err != nil {
					log.Error().Err(err).Int32("partition", record.Partition).Int64("offset", record.Offset).Msg("failed to persist inbox record, halting commits on this partition until next poll")
					failedPartitions[record.Partition] = true
					return
				}

				if err := client.CommitRecords(ctx, record); err != nil {
					log.Error().Err(err).Int32("partition", record.Partition).Int64("offset", record.Offset).Msg("failed to commit kafka offset, halting further commits on this partition")
					failedPartitions[record.Partition] = true
				}
			})
		}
	}()

	return nil
}
```

- [ ] **Step 3: Create `libs/inbox/worker.go`**

```go
package inbox

import (
	"context"
	"time"

	"planeo/libs/logger"

	"github.com/rs/zerolog"
)

const (
	DefaultPollInterval = 1 * time.Second
	DefaultBatchSize    = 100
	DefaultMaxAttempts  = 5
	DefaultClaimTTL     = 30 * time.Second
)

// Handler processes one durably-persisted inbox record. Injected by the
// consuming service — this is where business logic (calling domain
// services, LLM, etc.) lives. Never called until the record is already
// safely persisted.
type Handler func(ctx context.Context, record Record) error

type Worker struct {
	store        Store
	handler      Handler
	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	claimTTL     time.Duration
}

type Option func(*Worker)

func WithPollInterval(d time.Duration) Option {
	return func(w *Worker) { w.pollInterval = d }
}

func WithBatchSize(n int) Option {
	return func(w *Worker) { w.batchSize = n }
}

func WithMaxAttempts(n int) Option {
	return func(w *Worker) { w.maxAttempts = n }
}

func WithClaimTTL(d time.Duration) Option {
	return func(w *Worker) { w.claimTTL = d }
}

func NewWorker(store Store, handler Handler, opts ...Option) *Worker {
	w := &Worker{
		store:        store,
		handler:      handler,
		pollInterval: DefaultPollInterval,
		batchSize:    DefaultBatchSize,
		maxAttempts:  DefaultMaxAttempts,
		claimTTL:     DefaultClaimTTL,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Run polls the inbox and invokes Handler for each claimed record,
// sequentially, until ctx is cancelled. It blocks the calling goroutine.
func (w *Worker) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.pollOnce(ctx, log); err != nil {
				log.Error().Err(err).Msg("inbox worker poll failed")
			}
		}
	}
}

func (w *Worker) pollOnce(ctx context.Context, log zerolog.Logger) error {
	records, err := w.store.FetchBatch(ctx, w.batchSize, w.claimTTL)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if err := w.handler(ctx, rec); err != nil {
			log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to process inbox record")
			if markErr := w.store.MarkFailed(ctx, rec.ID, err, w.maxAttempts); markErr != nil {
				log.Error().Err(markErr).Int64("inbox_id", rec.ID).Msg("failed to mark inbox record as failed")
			}
			continue
		}

		if err := w.store.MarkProcessed(ctx, rec.ID); err != nil {
			log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to mark inbox record as processed")
		}
	}

	return nil
}
```

- [ ] **Step 4: Create `libs/inbox/worker_test.go`**

```go
package inbox_test

import (
	"context"
	"errors"
	"planeo/libs/inbox"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeStore struct {
	mu         sync.Mutex
	records    []inbox.Record
	processed  []int64
	failed     map[int64]int
	maxReached []int64
}

func newFakeStore(records []inbox.Record) *fakeStore {
	return &fakeStore{records: records, failed: map[int64]int{}}
}

func (f *fakeStore) Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error) {
	return true, nil
}

func (f *fakeStore) FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]inbox.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.records) == 0 {
		return nil, nil
	}
	n := limit
	if n > len(f.records) {
		n = len(f.records)
	}
	batch := f.records[:n]
	f.records = f.records[n:]
	return batch, nil
}

func (f *fakeStore) MarkProcessed(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed = append(f.processed, id)
	return nil
}

func (f *fakeStore) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed[id]++
	if f.failed[id] >= maxAttempts {
		f.maxReached = append(f.maxReached, id)
	}
	return nil
}

func TestWorker(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("processes a fetched record and marks it processed", func(t *testing.T) {
		store := newFakeStore([]inbox.Record{{ID: 1, Topic: "t", Payload: []byte("v")}})
		var handled []int64
		handler := func(ctx context.Context, rec inbox.Record) error {
			handled = append(handled, rec.ID)
			return nil
		}
		worker := inbox.NewWorker(store, handler, inbox.WithPollInterval(10*time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = worker.Run(ctx)

		assert.Equal(t, []int64{1}, handled)
		assert.Equal(t, []int64{1}, store.processed)
	})

	t.Run("marks a record failed and quarantines it after max attempts", func(t *testing.T) {
		record := inbox.Record{ID: 2, Topic: "broken-topic", Payload: []byte("v")}
		store := newFakeStore([]inbox.Record{record, record, record})
		handler := func(ctx context.Context, rec inbox.Record) error {
			return errors.New("simulated handler failure")
		}
		worker := inbox.NewWorker(store, handler, inbox.WithPollInterval(10*time.Millisecond), inbox.WithMaxAttempts(2))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = worker.Run(ctx)

		assert.GreaterOrEqual(t, store.failed[2], 2)
		assert.Contains(t, store.maxReached, int64(2))
	})
}
```

- [ ] **Step 5: Run the test**

Run: `go test ./libs/inbox/... -v -short -count=1`
Expected: PASS, both subtests green.

- [ ] **Step 6: Verify the package builds standalone**

Run: `go build ./libs/inbox/...`
Expected: exit 0.

- [ ] **Step 7: Commit**

```bash
git add libs/inbox/
git commit -m "feat(inbox): add libs/inbox package - Store, Consumer, Worker"
```

---

### Task 4: Postgres schema + `services/core`'s `Store` implementation

**Files:**
- Create: `services/core/internal/infra/postgres/migrations/20260714120000_add_inbox_table.sql`
- Create: `services/core/internal/infra/postgres/inbox_repository.go`
- Create: `services/core/internal/test/inbox/inbox_test.go`

**Interfaces:**
- Consumes: `inbox.Record`, `inbox.Store` (Task 3).
- Produces: `(*postgres.Client)` satisfying `inbox.Store` via `Save`, `FetchBatch`, `MarkProcessed`, `MarkFailed` — consumed by Task 7's `email-received-consumer` wiring.
- This task is independently testable — it does not depend on Task 2's, Task 5's, or Task 6's changes (this package doesn't import `libs/events`, `services/core/internal/infra/events`, or touch the `requests` table at all).

- [ ] **Step 1: Create the migration**

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE inbox (
    id           BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    topic        TEXT NOT NULL,
    partition    INTEGER NOT NULL,
    "offset"     BIGINT NOT NULL,
    payload      BYTEA NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    claimed_at   TIMESTAMPTZ,
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT,
    received_at  TIMESTAMPTZ DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    failed_at    TIMESTAMPTZ,
    UNIQUE (topic, partition, "offset")
);

CREATE INDEX inbox_pending_idx ON inbox (id) WHERE status IN ('pending', 'processing');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS inbox_pending_idx;
DROP TABLE IF EXISTS inbox;
-- +goose StatementEnd
```

- [ ] **Step 2: Create `services/core/internal/infra/postgres/inbox_repository.go`**

```go
package postgres

import (
	"context"
	"errors"
	"planeo/libs/inbox"
	"time"

	"github.com/jackc/pgx/v5"
)

func (c *Client) Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error) {
	query := `
		INSERT INTO inbox (topic, partition, "offset", payload)
		VALUES (@topic, @partition, @offset, @payload)
		ON CONFLICT (topic, partition, "offset") DO NOTHING
		RETURNING id`
	args := pgx.NamedArgs{
		"topic":     topic,
		"partition": partition,
		"offset":    offset,
		"payload":   payload,
	}

	row, err := c.db.Query(ctx, query, args)
	if err != nil {
		return false, NewDatabaseError("error inserting inbox record", err)
	}

	_, err = pgx.CollectExactlyOneRow(row, pgx.RowTo[int64])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, NewDatabaseError("error collecting inserted inbox id", err)
	}

	return true, nil
}

func (c *Client) FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]inbox.Record, error) {
	cutoff := time.Now().Add(-claimTTL)

	query := `
		UPDATE inbox
		SET status = 'processing', claimed_at = NOW()
		WHERE id IN (
			SELECT id FROM inbox
			WHERE status = 'pending'
			   OR (status = 'processing' AND claimed_at < @cutoff)
			ORDER BY id
			LIMIT @limit
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, topic, payload`
	args := pgx.NamedArgs{"cutoff": cutoff, "limit": limit}

	rows, err := c.db.Query(ctx, query, args)
	if err != nil {
		return nil, NewDatabaseError("error claiming inbox batch", err)
	}
	defer rows.Close()

	records, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (inbox.Record, error) {
		var r inbox.Record
		err := row.Scan(&r.ID, &r.Topic, &r.Payload)
		return r, err
	})
	if err != nil {
		return nil, NewDatabaseError("error collecting inbox batch", err)
	}

	return records, nil
}

func (c *Client) MarkProcessed(ctx context.Context, id int64) error {
	args := pgx.NamedArgs{"id": id}
	_, err := c.db.Exec(ctx, `UPDATE inbox SET status = 'processed', processed_at = NOW() WHERE id = @id`, args)
	if err != nil {
		return NewDatabaseError("error marking inbox record processed", err)
	}
	return nil
}

func (c *Client) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	args := pgx.NamedArgs{"id": id, "lastError": procErr.Error(), "maxAttempts": maxAttempts}
	query := `
		UPDATE inbox
		SET attempts = attempts + 1,
		    last_error = @lastError,
		    status = CASE WHEN attempts + 1 >= @maxAttempts THEN 'failed' ELSE 'pending' END,
		    failed_at = CASE WHEN attempts + 1 >= @maxAttempts THEN NOW() ELSE failed_at END
		WHERE id = @id`
	_, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error marking inbox record failed", err)
	}
	return nil
}
```

- [ ] **Step 3: Create the integration test**

```go
// services/core/internal/test/inbox/inbox_test.go
package inbox_test

import (
	"context"
	"errors"
	"planeo/services/core/internal/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInboxRepositorySave(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)

	t.Run("inserts a new record", func(t *testing.T) {
		inserted, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
		assert.Nil(t, err)
		assert.True(t, inserted)
	})

	t.Run("is idempotent on a duplicate topic+partition+offset", func(t *testing.T) {
		inserted, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
		assert.Nil(t, err)
		assert.False(t, inserted, "a conflicting (topic, partition, offset) must not create a second row")
	})
}

func TestInboxRepositoryFetchBatch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)
	_, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
	assert.Nil(t, err)

	t.Run("claims a pending record", func(t *testing.T) {
		records, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(records))
		assert.Equal(t, "email-received", records[0].Topic)
	})

	t.Run("does not reclaim a record still within its claim TTL", func(t *testing.T) {
		records, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(records), "the record claimed in the previous poll is still within its TTL")
	})

	t.Run("reclaims a record whose claim has expired", func(t *testing.T) {
		records, err := env.DB.FetchBatch(context.Background(), 10, 0*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(records), "a claimTTL of 0 means any processing record is immediately reclaimable")
	})

	t.Run("MarkProcessed marks the record processed and excludes it from future batches", func(t *testing.T) {
		err := env.DB.MarkProcessed(context.Background(), 1)
		assert.Nil(t, err)

		records, err := env.DB.FetchBatch(context.Background(), 10, 0*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(records))
	})
}

func TestInboxRepositoryMarkFailed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)
	_, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
	assert.Nil(t, err)

	records, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(records))
	recordID := records[0].ID

	t.Run("resets to pending when attempts is still below maxAttempts", func(t *testing.T) {
		err := env.DB.MarkFailed(context.Background(), recordID, errors.New("boom"), 3)
		assert.Nil(t, err)

		batch, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(batch), "a not-yet-exhausted failure must be reset to pending, not left claimed")
	})

	t.Run("quarantines the record once maxAttempts is reached", func(t *testing.T) {
		err := env.DB.MarkFailed(context.Background(), recordID, errors.New("boom"), 2)
		assert.Nil(t, err)

		batch, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(batch), "a record that reached maxAttempts must be quarantined, excluded from future batches")
	})
}
```

`utils.NewIntegrationTestEnvironment(t)` is core's existing testcontainer harness (`services/core/internal/test/utils/setup.go`) — it spins up Postgres and Keycloak and exposes `env.DB *postgres.Client`. This test only needs the Postgres side (`env.DB`), but reuses the existing harness rather than building a lighter one, matching how every other integration test in `services/core` already does this.

- [ ] **Step 4: Verify it compiles and run the test**

Run: `go build ./services/core/internal/infra/postgres/... ./services/core/internal/test/inbox/...`
Expected: exit 0.

Run: `go test ./services/core/internal/test/inbox/... -v -count=1`
Expected: PASS, all subtests in `TestInboxRepositorySave`, `TestInboxRepositoryFetchBatch`, `TestInboxRepositoryMarkFailed` green. (Requires Docker running locally, for testcontainers.)

- [ ] **Step 5: Commit**

```bash
git add services/core/internal/infra/postgres/migrations/20260714120000_add_inbox_table.sql services/core/internal/infra/postgres/inbox_repository.go services/core/internal/test/inbox/
git commit -m "feat(core): add inbox table migration and Postgres Store implementation"
```

---

### Task 5: Make `CreateRequest` idempotent on `(organization_id, reference_id)`

**Files:**
- Modify: `services/core/internal/infra/postgres/migrations/20241101135140_initialize_database.sql` (fix pre-existing colliding seed data — see Step 1)
- Create: `services/core/internal/infra/postgres/migrations/20260714130000_add_requests_reference_id_unique_index.sql`
- Modify: `services/core/internal/infra/postgres/request_repository.go`
- Modify: `services/core/internal/test/request/request_test.go`

**Interfaces:**
- No signature changes — `CreateRequest(ctx context.Context, request request.NewRequest) (int, error)` keeps its exact shape; only its internal SQL changes. Consumed identically by Task 6's `CreateInboxHandler`.

**Why this task exists:** `Worker.pollOnce` (Task 3) marks a row `'processed'` only *after* the handler succeeds. If `MarkProcessed` itself then fails (e.g. a transient Postgres blip right after the handler completed), the row is left at status `'processing'` with nothing to retry the mark — once `claimTTL` elapses, `FetchBatch` reclaims it as an expired-processing row and the handler runs again. Today, `CreateRequest` is a bare `INSERT ... RETURNING id` with no conflict handling, and `requests.reference_id` (verified: `services/core/internal/infra/postgres/migrations/20241101135140_initialize_database.sql:38-53`) has no uniqueness constraint at all — so that re-run creates a second, duplicate `Request` row for the same source email. This closes that gap the same way the outbox's `CreateMail` already closes the equivalent gap on the producer side (`ON CONFLICT (setting_id, message_id) DO NOTHING`).

`reference_id` is set exactly once, in `services/core/internal/infra/events/email_received.go` (`ReferenceId: payload.MessageID` — the source email's RFC822 Message-ID). Requests created via the REST API never set it (verified: `CreateRequestInputBody` — `services/core/internal/infra/rest/api/v1/requests/dto_create_request.go` — has no `ReferenceId` field at all, and the handler's `request.NewRequest{...}` construction — `services/core/internal/infra/rest/api/v1/requests/handler.go:44-53` — never sets it either), so it defaults to Go's zero value `""` for every manually-created request. **The fix must not constrain those** — a naive `UNIQUE (organization_id, reference_id)` would allow at most one manually-created request per organization, a severe regression. The constraint is scoped with a partial index (`WHERE reference_id <> ''`) so only rows with a real reference_id are deduplicated; empty-reference-id rows are completely unconstrained.

- [ ] **Step 1: Fix pre-existing seed data that would collide with the new constraint**

`services/core/internal/infra/postgres/migrations/20241101135140_initialize_database.sql:96-101` seeds 5 `requests` rows, all with `organization_id = 1` and all with the identical placeholder `reference_id = '1234'` — this predates this task entirely and is unrelated demo/fixture data (untouched since the file was created), but it collides with the new unique index Step 2 adds: Postgres cannot build a unique index over data that already violates it. Confirmed (per project decision) there is no real deployed environment where this migration has already run against real data — this is dev/testcontainers-only so far, so fixing the seed fixture file directly is sufficient; no separate data-migration/cleanup step is needed.

Change:
```sql
INSERT INTO requests (text, subject, name, email, address, telephone, category_id, organization_id, reference_id, raw) VALUES
('Install new electrical outlets in the conference room', 'Installation electrics in conference room', 'Emily Clark', 'emily.clark@example.com', '123 Main St, Springfield', '555-1234', 1, 1, '1234', ''),
('Routine maintenance of the electrical wiring in the main office', 'Request: Maintenance electrical wiring' ,'Michael Scott', 'michael.scott@example.com', '456 Elm St, Scranton', '555-5678', 2, 1, '1234', ''),
('Repair the broken light fixtures in the hallway', 'Request for fixing broken light fixtures in hallway' ,'Sarah Lee', 'sarah.lee@example.com', '789 Oak St, Metropolis', '555-8765', 3, 1, '1234', ''),
('Order new circuit breakers for the electrical panel', 'Order: Circuit breakers No.PW-44021' ,'David Wilson', 'david.wilson@example.com', '101 Pine St, Gotham', '555-4321', 4, 1, '1234', ''),
('Customer support for troubleshooting a power outage issue', 'Customer support needed for outage problem' ,'Laura Martinez', 'laura.martinez@example.com', '202 Maple St, Star City', '555-6789', 5, 1, '1234', '');
```
to (each row gets its own distinct `reference_id`, preserving the "came from an email" semantics the field implies, per project decision):
```sql
INSERT INTO requests (text, subject, name, email, address, telephone, category_id, organization_id, reference_id, raw) VALUES
('Install new electrical outlets in the conference room', 'Installation electrics in conference room', 'Emily Clark', 'emily.clark@example.com', '123 Main St, Springfield', '555-1234', 1, 1, '1234-1', ''),
('Routine maintenance of the electrical wiring in the main office', 'Request: Maintenance electrical wiring' ,'Michael Scott', 'michael.scott@example.com', '456 Elm St, Scranton', '555-5678', 2, 1, '1234-2', ''),
('Repair the broken light fixtures in the hallway', 'Request for fixing broken light fixtures in hallway' ,'Sarah Lee', 'sarah.lee@example.com', '789 Oak St, Metropolis', '555-8765', 3, 1, '1234-3', ''),
('Order new circuit breakers for the electrical panel', 'Order: Circuit breakers No.PW-44021' ,'David Wilson', 'david.wilson@example.com', '101 Pine St, Gotham', '555-4321', 4, 1, '1234-4', ''),
('Customer support for troubleshooting a power outage issue', 'Customer support needed for outage problem' ,'Laura Martinez', 'laura.martinez@example.com', '202 Maple St, Star City', '555-6789', 5, 1, '1234-5', '');
```

This is an edit to an already-existing migration file, not a new one — safe here specifically because it's dev/testcontainer-only fixture data that has never run against a real environment (confirmed above). Do not use this as precedent for editing other already-applied migrations in general.

- [ ] **Step 2: Create the new migration**

```sql
-- +goose Up
-- +goose StatementBegin

-- Two requests in the same organization must not share the same
-- reference_id (the source email's Message-ID) — this is what makes
-- re-processing an inbox record after a failed MarkProcessed safe:
-- CreateRequest resolves to the existing row instead of creating a
-- duplicate. Requests without a reference_id (e.g. created manually via
-- the REST API, where it's never set and defaults to '') are excluded
-- by the WHERE clause and remain completely unconstrained.
CREATE UNIQUE INDEX requests_org_reference_id_idx ON requests (organization_id, reference_id) WHERE reference_id <> '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS requests_org_reference_id_idx;
-- +goose StatementEnd
```

- [ ] **Step 3: Update `CreateRequest` in `services/core/internal/infra/postgres/request_repository.go`**

Change the query (and only the query — nothing else in this function changes) from:
```go
	query := `
		INSERT INTO requests (text, name, subject, email, address, telephone, raw, closed, reference_id, organization_id, category_id)
		VALUES (@text, @name, @subject, @email, @address, @telephone, @raw, @closed, @referenceId, @organizationId, @categoryId)
		RETURNING id`
```
to:
```go
	query := `
		INSERT INTO requests (text, name, subject, email, address, telephone, raw, closed, reference_id, organization_id, category_id)
		VALUES (@text, @name, @subject, @email, @address, @telephone, @raw, @closed, @referenceId, @organizationId, @categoryId)
		ON CONFLICT (organization_id, reference_id) WHERE reference_id <> '' DO UPDATE SET id = requests.id
		RETURNING id`
```

`ON CONFLICT (...) WHERE reference_id <> '' DO UPDATE SET id = requests.id` targets the partial index from Step 2 exactly (Postgres requires the `ON CONFLICT` clause's predicate to match the partial index's predicate for conflict detection to use it). The `DO UPDATE SET id = requests.id` is a no-op update — its only purpose is to make `RETURNING id` fire on the conflicting row too, so callers always get an id back regardless of whether this call inserted a new row or matched an existing one. For rows where `reference_id = ''` (manual creation), the partial index doesn't cover them at all, so no conflict is ever detected and the insert always succeeds as a fresh row, exactly as today.

- [ ] **Step 4: Add the idempotency test to `services/core/internal/test/request/request_test.go`**

Add `"context"` to the file's existing import block (not currently imported), then add this new top-level test function (its own `NewIntegrationTestEnvironment(t)`, matching the existing file's one-container-per-top-level-test convention):

```go
func TestCreateRequestIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)

	t.Run("creating a request twice with the same organization+referenceId returns the same id", func(t *testing.T) {
		newRequest := request.NewRequest{
			Subject:        "Idempotency test",
			Text:           "body",
			Email:          "sender@example.com",
			OrganizationId: 1,
			ReferenceId:    "duplicate-message-id",
		}

		firstId, err := env.DB.CreateRequest(context.Background(), newRequest)
		assert.Nil(t, err)
		assert.NotZero(t, firstId)

		secondId, err := env.DB.CreateRequest(context.Background(), newRequest)
		assert.Nil(t, err)
		assert.Equal(t, firstId, secondId, "reprocessing the same source email must resolve to the same Request row, not create a duplicate")
	})

	t.Run("requests without a referenceId remain unconstrained", func(t *testing.T) {
		manualRequest := request.NewRequest{
			Subject:        "Manually created",
			Text:           "body",
			Email:          "operator@example.com",
			OrganizationId: 1,
			ReferenceId:    "",
		}

		firstId, err := env.DB.CreateRequest(context.Background(), manualRequest)
		assert.Nil(t, err)

		secondId, err := env.DB.CreateRequest(context.Background(), manualRequest)
		assert.Nil(t, err)
		assert.NotEqual(t, firstId, secondId, "requests with an empty referenceId (e.g. created manually) must not be deduplicated")
	})
}
```

- [ ] **Step 5: Verify it compiles and run the tests**

Run: `go build ./services/core/...`
Expected: exit 1, with the error confined to `services/core/internal/infra/events` (`package planeo/libs/events is not in std`) — this is Task 2's already-deliberate, still-unresolved break (fixed by Task 6, not this task). Confirm the failure is confined to that one package (e.g. via `git stash` and re-running the same build on the unmodified tip, to verify the failure is identical/pre-existing, not something this task's changes caused).

Run: `go vet ./services/core/internal/test/request/...`
Expected: exit 0 — this compiles the test files themselves (unlike `go build`, which skips `_test.go` files) without needing Docker, and confirms `internal/test/utils` doesn't transitively import the still-broken `internal/infra/events` package, so this task's actual changes are unaffected by Task 2's break.

Run: `go test ./services/core/internal/test/request/... -v -count=1 -run TestCreateRequestIdempotency`
Expected: PASS, both subtests green. (Requires Docker running locally, for testcontainers.)

Run: `go test ./services/core/internal/test/request/... -v -count=1`
Expected: PASS — the existing `TestRequestIntegration` suite (unrelated to this change) still passes, confirming no regression.

- [ ] **Step 6: Commit**

```bash
git add services/core/internal/infra/postgres/migrations/20241101135140_initialize_database.sql services/core/internal/infra/postgres/migrations/20260714130000_add_requests_reference_id_unique_index.sql services/core/internal/infra/postgres/request_repository.go services/core/internal/test/request/request_test.go
git commit -m "fix(core): make CreateRequest idempotent on (organization_id, reference_id)"
```

---

### Task 6: Rewire `services/core`'s event handling — `CreateInboxHandler`, drop `InitializeEvents`

**Files:**
- Modify: `services/core/internal/infra/events/email_received.go`
- Modify: `services/core/internal/infra/events/events.go`
- Modify: `services/core/internal/config/config.go`
- Modify: `services/core/cmd/main.go`

**Interfaces:**
- Consumes: `inbox.Handler`, `inbox.Record` (Task 3).
- Produces: `coreEvents.CreateInboxHandler(services Services) inbox.Handler`, `coreEvents.Services{RequestService, CategoryService}` (unchanged shape) — consumed by Task 7.
- **This task resolves Task 2's deliberate compile break** in `services/core/internal/infra/events/events.go`.

- [ ] **Step 1: Rewrite `services/core/internal/infra/events/email_received.go`**

```go
package core_events

import (
	"context"
	"encoding/json"
	"fmt"
	"planeo/libs/events/contracts"
	"planeo/libs/inbox"
	"planeo/libs/logger"
	"time"

	"planeo/services/core/internal/domain/request"
	"planeo/services/core/internal/infra/llm"
)

//nolint:funlen
func CreateInboxHandler(services Services) inbox.Handler {
	return func(ctx context.Context, record inbox.Record) error {
		log := logger.FromContext(ctx)

		var payload contracts.EmailCreatedPayload
		if err := json.Unmarshal(record.Payload, &payload); err != nil {
			log.Error().Err(err).Int64("inbox_id", record.ID).Msg("Failed to unmarshal inbox payload")
			return err
		}

		log.Info().Str("message_id", payload.MessageID).Int("organization_id", payload.OrganizationId).Msg("Received email")

		raw := fmt.Sprintf("Subject: %s\nFrom: %s\nDate: %s\nMessage-ID: %s\nBody: %s",
			payload.Subject, payload.From, payload.Date.Format(time.RFC1123), payload.MessageID, payload.Body)

		requestId, err := services.RequestService.CreateRequest(ctx, request.NewRequest{
			Subject:        payload.Subject,
			Raw:            raw,
			Text:           payload.Body,
			Email:          payload.From,
			OrganizationId: payload.OrganizationId,
			ReferenceId:    payload.MessageID,
		})

		if err != nil {
			log.Error().Err(err).Msg("Failed to create request from email")
			return err
		}

		categories, err := services.CategoryService.GetCategories(ctx, payload.OrganizationId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get categories")
			return err
		}

		// Extract fields from the request
		extractedFields, err := llm.ExtractRequestFields(ctx, raw)
		if err != nil {
			log.Error().Err(err).Msg("Failed to extract fields from request")
		}

		// Classify the request
		requestData := llm.RequestData{
			Subject: payload.Subject,
			Text:    payload.Body,
		}
		categoryId, err := llm.ClassifyRequest(ctx, requestData, categories)

		if err != nil {
			log.Error().Err(err).Msg("Failed to classify request")
			return err
		}

		updatedRequest := request.UpdateRequest{
			Id:             requestId,
			Text:           payload.Body,
			Subject:        payload.Subject,
			Email:          payload.From,
			Name:           extractedFields.Name,
			Address:        extractedFields.Address,
			Telephone:      extractedFields.Phone,
			CategoryId:     categoryId,
			OrganizationId: payload.OrganizationId,
		}

		err = services.RequestService.UpdateRequest(ctx, updatedRequest)

		if err != nil {
			log.Error().Err(err).Msg("Failed to update request")
			return err
		}

		return nil
	}
}
```

The only behavioral change from the old `CreateEmailReceivedCallback`: it now unmarshals `record.Payload` itself (the caller hands it raw bytes, not a pre-parsed struct), and it derives its logger from the per-invocation `ctx` parameter (which `inbox.Worker.Run` passes per call) rather than a `ctx` closed over once at construction time. Everything from `CreateRequest` onward is identical to before.

- [ ] **Step 2: Rewrite `services/core/internal/infra/events/events.go`**

```go
package core_events

import (
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
)

type Services struct {
	RequestService  request.Service
	CategoryService category.Service
}
```

`InitializeEvents` is removed entirely — `services/core` no longer subscribes to Kafka itself; that responsibility moves to `email-received-consumer` (Task 7). The `Services` struct is unchanged and still used by `CreateInboxHandler`.

- [ ] **Step 3: Remove the now-dead `KafkaBrokers` field from `services/core/internal/config/config.go`**

Change the `ApplicationConfiguration` struct from:
```go
type ApplicationConfiguration struct {
	Host                string
	Port                string
	KafkaBrokers        string
	DbHost              string
	...
```
to:
```go
type ApplicationConfiguration struct {
	Host                string
	Port                string
	DbHost              string
	...
```

Remove the corresponding line from `LoadConfig`:
```go
		KafkaBrokers:        readEnvVariable(ctx, "KAFKA_BROKERS"),
```

(Verified this is the field's only use anywhere in `services/core` — its sole caller was `cmd/main.go`'s `InitializeEvents` call, removed in the next step.)

- [ ] **Step 4: Update `services/core/cmd/main.go`**

Remove the `coreEvents "planeo/services/core/internal/infra/events"` line from the import block.

Remove this block entirely:
```go
	// initialize event service
	err := coreEvents.InitializeEvents(ctx, config.KafkaBrokers, coreEvents.Services{RequestService: requestService, CategoryService: categoryService})

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}
```

`categoryService`/`requestService` remain (still used by the REST server wiring below this block) — only the event-subscription block and its now-unused import are removed.

- [ ] **Step 5: Verify the whole module compiles**

Run: `go build ./...`
Expected: exit 0 — this is the point where Task 2's deliberate break is fully resolved.

Run: `go vet ./...`
Expected: exit 0.

- [ ] **Step 6: Run core's existing test suite to confirm no regression**

Run: `task test:core:unit`
Expected: PASS (no test currently exercises `CreateEmailReceivedCallback`/`CreateInboxHandler`/`InitializeEvents` directly — this is a pre-existing gap, not something this task introduces — so this is a compile-correctness confirmation, not new behavioral coverage).

- [ ] **Step 7: Commit**

```bash
git add services/core/internal/infra/events/ services/core/internal/config/config.go services/core/cmd/main.go
git commit -m "refactor(core): replace inline Kafka subscription with CreateInboxHandler, drop InitializeEvents"
```

---

### Task 7: `services/core/cmd/email-received-consumer` binary + deployment wiring

**Files:**
- Create: `services/core/cmd/email-received-consumer/main.go`
- Create: `services/core/cmd/email-received-consumer/config.go`
- Create: `services/core/Dockerfile.email-received-consumer`
- Modify: `Taskfile.yml`
- Modify: `dev/docker-compose.yaml`
- Modify: `dev/.env.template`

**Interfaces:**
- Consumes: `inbox.NewConsumer`, `inbox.NewWorker` (Task 3); `(*postgres.Client)` satisfying `inbox.Store` (Task 4); `coreEvents.CreateInboxHandler`, `coreEvents.Services` (Task 6).
- This is the final integration point — it depends on Tasks 2, 3, 4, 5, and 6 all being complete.

- [ ] **Step 1: Create `services/core/cmd/email-received-consumer/config.go`**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"planeo/libs/logger"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DbHost       string
	DbPort       string
	DbUser       string
	DbPassword   string
	DbName       string
	KafkaBrokers string
	GroupName    string
	PollInterval time.Duration
	BatchSize    int
	MaxAttempts  int
	ClaimTTL     time.Duration
}

func (c *Config) DatabaseConfig() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DbUser, c.DbPassword, c.DbHost, c.DbPort, c.DbName)
}

func LoadConfig(ctx context.Context, filenames ...string) *Config {
	if err := godotenv.Load(filenames...); err != nil {
		l := logger.FromContext(ctx)
		l.Warn().Err(err).Msg("Error loading .env file")
	}

	return &Config{
		DbHost:       readEnvVariable(ctx, "DB_HOST"),
		DbPort:       readEnvVariable(ctx, "DB_PORT"),
		DbUser:       readEnvVariable(ctx, "DB_USER"),
		DbPassword:   readEnvVariable(ctx, "DB_PASSWORD"),
		DbName:       readEnvVariable(ctx, "DB_NAME"),
		KafkaBrokers: readEnvVariable(ctx, "KAFKA_BROKERS"),
		GroupName:    readStringEnvVariableWithDefault("EMAIL_RECEIVED_CONSUMER_GROUP_NAME", "core-email-received-consumer"),
		PollInterval: readDurationEnvVariable(ctx, "EMAIL_RECEIVED_CONSUMER_POLL_INTERVAL", 1*time.Second),
		BatchSize:    readIntEnvVariable(ctx, "EMAIL_RECEIVED_CONSUMER_BATCH_SIZE", 100),
		MaxAttempts:  readIntEnvVariable(ctx, "EMAIL_RECEIVED_CONSUMER_MAX_ATTEMPTS", 5),
		ClaimTTL:     readDurationEnvVariable(ctx, "EMAIL_RECEIVED_CONSUMER_CLAIM_TTL", 30*time.Second),
	}
}

func readEnvVariable(ctx context.Context, name string) string {
	v, ok := os.LookupEnv(name)
	if !ok {
		l := logger.FromContext(ctx)
		l.Fatal().Msgf("Missing env variable '%s'. Aborting...\n", name)
	}
	return v
}

func readStringEnvVariableWithDefault(name string, def string) string {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	return v
}

func readDurationEnvVariable(ctx context.Context, name string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Fatal().Err(err).Msgf("Invalid duration for env variable '%s'", name)
	}
	return d
}

func readIntEnvVariable(ctx context.Context, name string, def int) int {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Fatal().Err(err).Msgf("Invalid integer for env variable '%s'", name)
	}
	return n
}
```

`EMAIL_RECEIVED_CONSUMER_GROUP_NAME` uses `readStringEnvVariableWithDefault` (not `readEnvVariable`) — unlike the other required vars, it has a collision-safe default (`core-email-received-consumer`, prefixed by service name) rather than failing hard if unset, per the spec's Section 7.

- [ ] **Step 2: Create `services/core/cmd/email-received-consumer/main.go`**

```go
package main

import (
	"context"
	"os/signal"
	"planeo/libs/events/contracts"
	"planeo/libs/inbox"
	"planeo/libs/logger"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
	coreEvents "planeo/services/core/internal/infra/events"
	"planeo/services/core/internal/infra/postgres"
	"strings"
	"syscall"
)

func main() {
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	log := logger.New("email-received-consumer")
	ctx := logger.WithContext(context.Background(), log)

	log.Info().Msg("Loading environment variables")
	cfg := LoadConfig(ctx)

	db := postgres.NewClient(ctx, cfg.DatabaseConfig())
	defer db.Close()

	categoryService := category.NewService(db)
	requestService := request.NewService(db)

	handler := coreEvents.CreateInboxHandler(coreEvents.Services{
		RequestService:  requestService,
		CategoryService: categoryService,
	})

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	consumer := inbox.NewConsumer(brokers, cfg.GroupName, contracts.EmailReceivedTopic, db)
	if err := consumer.Run(runCtx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start email-received consumer")
	}

	worker := inbox.NewWorker(db, handler,
		inbox.WithPollInterval(cfg.PollInterval),
		inbox.WithBatchSize(cfg.BatchSize),
		inbox.WithMaxAttempts(cfg.MaxAttempts),
		inbox.WithClaimTTL(cfg.ClaimTTL),
	)

	log.Info().Msg("Email-received consumer running")
	if err := worker.Run(runCtx); err != nil {
		log.Info().Err(err).Msg("Email-received consumer stopped")
	}
}
```

`consumer.Run(runCtx)` is called before `worker.Run(runCtx)` and is non-blocking (it starts its own background goroutine and returns immediately once the Kafka consumer-group client connects, only erroring on setup failure — matching how the old `InitializeEvents`/`SubscribeEmailReceived` call worked). `worker.Run(runCtx)` is the blocking call that keeps this process alive, exactly mirroring `email-received-producer/main.go`'s `relay.Run(runCtx)`.

`db` (a `*postgres.Client`) is passed as the `Store` argument to both `inbox.NewConsumer` and `inbox.NewWorker` — it satisfies `inbox.Store` via Task 4's `inbox_repository.go`.

- [ ] **Step 3: Create `services/core/Dockerfile.email-received-consumer`**

```dockerfile
# syntax=docker/dockerfile:1

FROM golang:1.26.5-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/email-received-consumer ./services/core/cmd/email-received-consumer

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/email-received-consumer /email-received-consumer
ENTRYPOINT ["/email-received-consumer"]
```

- [ ] **Step 4: Add the Taskfile target**

Add, alongside `build:email:email-received-producer`:
```yaml
  build:core:email-received-consumer:
    desc: Build Docker image for email-received-consumer sidecar
    cmds:
      - echo "Building Docker image for email-received-consumer with tag {{.VERSION}}..."
      - docker build -t {{.DOCKER_REGISTRY}}/email-received-consumer:{{.VERSION}} -f services/core/Dockerfile.email-received-consumer .
```

Update `build:all` to include it:
```yaml
  build:all:
    desc: Build Docker images for all services
    cmds:
      - task: build:core
      - task: build:email
      - task: build:email:email-received-producer
      - task: build:core:email-received-consumer
```

- [ ] **Step 5: Add `MISTRAL_API_KEY` to `dev/.env.template`**

`llm.ExtractRequestFields`/`llm.ClassifyRequest` read `MISTRAL_API_KEY` from the process environment internally (via `mistral.New(...)`), and this is the first binary that needs it while running inside `docker compose` (core's main HTTP server currently only runs locally via `task run:core`, never in Docker). `docker compose` auto-loads a `.env` file from the same directory as `docker-compose.yaml` (`dev/.env`, which `task setup` creates from this template) for `${VAR}`-style substitution in the compose file itself.

Add this line to `dev/.env.template`:
```
MISTRAL_API_KEY=your-mistral-api-key
```

- [ ] **Step 6: Add the docker-compose service**

Add, after the `email-received-producer` service block:
```yaml
  # email-received consumer sidecar for services/core - durably persists
  # Kafka email-received messages into core's inbox table, then processes
  # them asynchronously (creates/updates a Request via LLM extraction and
  # classification).
  # NOTE: On first `task up`, this container will log query errors for a few
  # seconds while waiting for the inbox table to be created by migrations.
  # This is expected and not a regression.
  email-received-consumer:
    container_name: email-received-consumer
    build:
      context: ..
      dockerfile: services/core/Dockerfile.email-received-consumer
    depends_on:
      - postgres
      - kafka
    environment:
      DB_HOST: postgres
      DB_PORT: "5432"
      DB_USER: planeo
      DB_PASSWORD: planeo
      DB_NAME: planeo
      KAFKA_BROKERS: kafka:19092
      EMAIL_RECEIVED_CONSUMER_GROUP_NAME: core-email-received-consumer
      EMAIL_RECEIVED_CONSUMER_POLL_INTERVAL: 1s
      EMAIL_RECEIVED_CONSUMER_BATCH_SIZE: "100"
      EMAIL_RECEIVED_CONSUMER_MAX_ATTEMPTS: "5"
      EMAIL_RECEIVED_CONSUMER_CLAIM_TTL: 30s
      MISTRAL_API_KEY: ${MISTRAL_API_KEY}
```

(`DB_NAME: planeo` — core's database, not email's `mail` database.)

- [ ] **Step 7: Verify the whole module compiles**

Run: `go build ./...`
Expected: exit 0.

Run: `go vet ./...`
Expected: exit 0.

Run: `gofmt -l services/core/cmd/email-received-consumer/`
Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add services/core/cmd/email-received-consumer services/core/Dockerfile.email-received-consumer Taskfile.yml dev/docker-compose.yaml dev/.env.template
git commit -m "feat(core): add email-received-consumer binary and deployment wiring"
```

---

### Task 8: Full workspace verification

**Files:** none (verification only)

- [ ] **Step 1: Build and vet the entire module**

Run: `go build ./...`
Expected: exit 0.

Run: `go vet ./...`
Expected: exit 0.

- [ ] **Step 2: Format check**

Run: `gofmt -l .`
Expected: no output. If any file is listed, run `gofmt -w <file>` and re-check — unless that file is untouched by this plan's tasks. `services/email/internal/infra/rest/api/errors.go` has a pre-existing gofmt drift, tracing to a `main` commit predating this branch, that has been erroneously "fixed" and reverted twice before in this repo's history. If `gofmt -l .` lists it (or any other file this plan didn't touch), leave it alone.

- [ ] **Step 3: Confirm no stray references to removed/renamed names**

Run:
```bash
grep -rn "outbox-relay" --include="*.go" --include="*.yaml" --include="*.yml" --include="Dockerfile*" .
```
Expected: no output (folder path, Dockerfile name, Taskfile task/image tag, docker-compose service/container name all renamed to `email-received-producer`).

Run:
```bash
grep -rn "OUTBOX_POLL_INTERVAL\|OUTBOX_BATCH_SIZE\|OUTBOX_MAX_ATTEMPTS\|OUTBOX_CLAIM_TTL" .
```
Expected: no output (all four renamed to `EMAIL_RECEIVED_PRODUCER_*`).

Run:
```bash
grep -rn "SubscribeEmailReceived\|PublishEmailReceived\|CreateEmailReceivedCallback\|InitializeEvents" --include="*.go" .
```
Expected: no output (all four removed/renamed: `SubscribeEmailReceived`/`PublishEmailReceived` deleted, `CreateEmailReceivedCallback` renamed to `CreateInboxHandler`, `InitializeEvents` removed).

Run:
```bash
grep -rn "config\.KafkaBrokers\|KafkaBrokers ..*string" services/core/internal/config/
```
Expected: no output (field removed from `ApplicationConfiguration`).

Run:
```bash
ls libs/events/
```
Expected: only `contracts` listed (`service.go`/`email_received.go` deleted in Task 2; `libs/events/contracts` is unaffected and still used by `services/email/internal/domain/mail` and `services/core/internal/infra/events/email_received.go`).

Run:
```bash
grep -rln "planeo/libs/events\"" --include="*.go" .
```
Expected: no output (nothing imports the bare `libs/events` package anymore — only `planeo/libs/events/contracts`, a different import path, is still imported).

Run:
```bash
grep -n "ON CONFLICT (organization_id, reference_id)" services/core/internal/infra/postgres/request_repository.go
```
Expected: one match (`CreateRequest`'s idempotency fix from Task 5 is in place).

- [ ] **Step 4: Run every affected test suite**

Run:
```bash
task test:core:unit
task test:core:integration
task test:email:unit
task test:email:integration
task test:libs:unit
task test:libs:integration
```
Expected: all PASS. (Requires Docker running locally for the integration/testcontainer portions.)

- [ ] **Step 5: Commit (only if steps 2-3 required fixes; otherwise skip — do not create an empty commit)**

```bash
git add -A
git commit -m "chore: cleanup fixes from final verification"
```

---

## Explicitly deferred / out of scope

Per the approved spec: Section 9's per-key ordering in `Worker` (not designed, not scheduled — ideas only, captured in the spec for whoever picks it up later); any deeper Kafka consumer-group redesign beyond per-binary configurable group names; a generic multi-topic router; a shared claim/retry engine or shared `Record` type between `libs/outbox` and `libs/inbox`.
