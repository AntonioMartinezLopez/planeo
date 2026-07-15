# Adapter-Owned Transactional Processing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `libs/outbox.Relay`/`libs/inbox.Worker` with per-service, per-topic adapters that own the poll loop directly, closing the `MarkProcessed`-after-successful-handler gap on the consumer side and fixing a single-instance ordering bug on the producer side via a `claimed_by` fast-reclaim column.

**Architecture:** Both `libs/outbox` and `libs/inbox` shrink to their Kafka-only responsibilities (`Producer`, `Consumer`) plus a minimal generic `Runner` (ticker loop). All claim/mark/transaction logic moves into new adapter packages (`services/email/internal/infra/outbox`, `services/core/internal/infra/inbox`), each declaring its own `Repository` interface at the point of use, satisfied by the existing `*postgres.Client` on each service (no new wrapper types). The producer side keeps today's batch-claim-then-separately-mark model (transactions buy no atomicity there — Kafka isn't enrolled in Postgres's transaction); the consumer side wraps its domain writes and final mark in one transaction per record, with LLM calls gathered beforehand, outside any transaction.

**Tech Stack:** Go 1.24.5, pgx/v5, `github.com/twmb/franz-go`, `github.com/google/uuid`, goose migrations, testify, testcontainers-go.

## Global Constraints

- Reference spec: `docs/superpowers/specs/2026-07-15-adapter-transactional-processing-design.md`.
- Branch: `feature/migrate-nats-to-kafka` (already checked out). Do not commit to `main`.
- Does not reduce duplicate Kafka sends on the producer side — Kafka is not enrolled in the Postgres transaction, so "produce succeeds, then a crash before commit" still redelivers on the next poll. This is unchanged, expected, and not a defect to fix in this plan.
- Does not support multiple concurrent instances of the same producer/consumer — both sides remain single-instance-per-topic. The `claimed_by` mechanism is safe if a second instance is added later, but does not restore ordering under concurrency.
- `CreateRequest`'s idempotent-upsert behavior (from the earlier inbox-pattern plan's Task 5) is unchanged and untouched by this plan.
- Repository methods stay on the existing `*postgres.Client` for each service — no new `OutboxRepository`/`InboxRepository` wrapper types. The `Repository` interface each adapter package declares is satisfied directly by `*postgres.Client`.
- The repository-level transaction method is named `WithTransaction` (matching `services/email`'s existing `mail_repository.go` convention), not `WithTx` (that name is reserved for the `libs/db` package-level helper function it wraps).
- Producer-side (`services/email`) repository methods (`FetchBatch`, `MarkProcessed`, `MarkFailed`) stay on the plain pool (`c.db`) — no `db.FromContext`, no `WithTransaction`. This is deliberate and asymmetric with the consumer side: there is no transaction spanning the produce call to participate in.
- Consumer-side (`services/core`) `MarkProcessed`/`MarkFailed` **must** resolve their `Querier` via `db.FromContext(ctx, c.db)`, not call `c.db` directly — otherwise, called with a transaction's `ctx` from inside `WithTransaction`, they would silently run outside it.
- The consumer adapter's write-only transaction never calls `MarkFailed` on its own `txCtx`: once any statement in a Postgres transaction errors, the whole transaction is aborted and every subsequent statement on it fails too. `MarkFailed` is always called on the plain `ctx`, after the transaction has already returned (committed or rolled back).

---

### Task 1: Shrink `libs/outbox` — remove `Store`/`Relay`, add generic `Runner`

**Files:**
- Modify: `libs/outbox/store.go` (keep the `Record` struct — it stays the one shared transport type across the lib/adapter boundary — remove only the `Store` interface)
- Delete: `libs/outbox/relay.go`
- Delete: `libs/outbox/relay_test.go`
- Create: `libs/outbox/runner.go`
- Create: `libs/outbox/runner_test.go`

**Interfaces:**
- Produces: `outbox.Runner`, `outbox.NewRunner(opts ...Option) *Runner`, `func (r *Runner) Run(ctx context.Context, poll func(context.Context) error) error`, `outbox.Option`, `outbox.WithPollInterval(d time.Duration) Option`, `outbox.DefaultPollInterval`. `outbox.Record` (unchanged shape, `store.go`) stays — Task 3's adapter and Task 2's repository both consume it as the shared transport type; duplicating it in the adapter package would create a second, distinct type that `*postgres.Client` couldn't satisfy the adapter's `Repository` interface with.
- `outbox.Producer`/`outbox.NewProducer` (in `producer.go`) are untouched — not part of this task's diff.
- **This task deliberately breaks `services/email/cmd/email-received-producer/main.go`'s compile** (it references the now-deleted `outbox.Relay`/`outbox.NewRelay`/`outbox.Store`). This is expected and fixed by Task 3 — the same "deliberate break, fixed by a later task" pattern already used twice in this codebase's history (Task 2 of the inbox-pattern plan; Task 1 of the outbox-architecture-cleanup plan).

- [ ] **Step 1: Modify `libs/outbox/store.go` to remove only the `Store` interface**

Change the file from:
```go
package outbox

import (
	"context"
	"time"
)

// Record is a single outbox row ready to be produced to Kafka: already
// fully serialized bytes, opaque to the relay.
type Record struct {
	ID      int64
	Topic   string
	Key     []byte
	Payload []byte
}

// Store is implemented per-service against that service's own outbox
// table. This is the extension point for future services: business-
// specific fetch/claim/mark logic all lives in a Store implementation,
// never in Relay itself.
type Store interface {
	// FetchBatch atomically claims up to limit records that are either
	// pending or whose previous claim has expired (claimed longer ago than
	// claimTTL), and returns them for producing. This must be a single
	// atomic statement (e.g. UPDATE ... WHERE id IN (SELECT ... FOR UPDATE
	// SKIP LOCKED) RETURNING ...), not a separate SELECT followed later by
	// a separate mark call — otherwise the claim provides no protection
	// against a concurrent second poller. The FOR UPDATE SKIP LOCKED must
	// live in the inner SELECT, not be assumed implicit from the outer
	// UPDATE's WHERE id IN (...) alone — the latter does not reliably
	// prevent two concurrent claims from selecting overlapping ids.
	FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]Record, error)

	// MarkProcessed marks a record as successfully sent.
	MarkProcessed(ctx context.Context, id int64) error

	// MarkFailed records a failed send attempt. If the resulting attempt
	// count is still below maxAttempts, the record must be reset so it's
	// eligible for the next FetchBatch (not left claimed). Once attempts
	// reaches maxAttempts, the record is quarantined: excluded from future
	// FetchBatch calls, retained for inspection.
	MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error
}
```
to:
```go
package outbox

// Record is a single outbox row ready to be produced to Kafka: already
// fully serialized bytes, opaque to the runner/adapter. The Store
// interface that used to live alongside this type has been removed —
// each service now declares its own Repository interface (see
// services/email/internal/infra/outbox), satisfied directly by that
// service's *postgres.Client. Record itself stays here, shared, so the
// adapter and the repository layer agree on exactly one type.
type Record struct {
	ID      int64
	Topic   string
	Key     []byte
	Payload []byte
}
```

- [ ] **Step 2: Delete `relay.go` and `relay_test.go`**

```bash
git rm libs/outbox/relay.go libs/outbox/relay_test.go
```

- [ ] **Step 3: Create `libs/outbox/runner.go`**

```go
package outbox

import (
	"context"
	"time"

	"planeo/libs/logger"
)

const DefaultPollInterval = 1 * time.Second

// Runner calls a poll function on a fixed interval until ctx is cancelled.
// It has no knowledge of Store, Producer, or any claim/mark logic — those
// now live in each service's own adapter, which supplies its own poll
// function to Run.
type Runner struct {
	pollInterval time.Duration
}

type Option func(*Runner)

func WithPollInterval(d time.Duration) Option {
	return func(r *Runner) { r.pollInterval = d }
}

func NewRunner(opts ...Option) *Runner {
	r := &Runner{pollInterval: DefaultPollInterval}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run calls poll on every tick until ctx is cancelled. It blocks the
// calling goroutine. poll's own errors are logged, not returned — a single
// failed poll must not stop the loop.
func (r *Runner) Run(ctx context.Context, poll func(context.Context) error) error {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := poll(ctx); err != nil {
				log.Error().Err(err).Msg("outbox runner poll failed")
			}
		}
	}
}
```

- [ ] **Step 4: Create `libs/outbox/runner_test.go`**

```go
package outbox_test

import (
	"context"
	"errors"
	"planeo/libs/outbox"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunner(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("calls poll on every tick until ctx is cancelled", func(t *testing.T) {
		var calls int32
		poll := func(ctx context.Context) error {
			atomic.AddInt32(&calls, 1)
			return nil
		}
		runner := outbox.NewRunner(outbox.WithPollInterval(10 * time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Millisecond)
		defer cancel()
		err := runner.Run(ctx, poll)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(4))
	})

	t.Run("a poll error does not stop the loop", func(t *testing.T) {
		var calls int32
		poll := func(ctx context.Context) error {
			atomic.AddInt32(&calls, 1)
			return errors.New("simulated poll failure")
		}
		runner := outbox.NewRunner(outbox.WithPollInterval(10 * time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
		defer cancel()
		_ = runner.Run(ctx, poll)

		assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2))
	})
}
```

- [ ] **Step 5: Run the new test**

Run: `go test ./libs/outbox/... -v -short -count=1`
Expected: PASS (both `TestRunner` subtests green). Note: `go build ./...`/`go vet ./...` for the whole workspace will fail at this point (Task 3 fixes it) — that's expected; this step only verifies `libs/outbox` itself.

Run: `go build ./libs/outbox/...`
Expected: exit 0 (confirms `libs/outbox` itself compiles cleanly, isolated from the expected downstream break).

- [ ] **Step 6: Commit**

```bash
git add libs/outbox/
git commit -m "refactor(outbox): remove Relay/Store, add generic Runner"
```

---

### Task 2: Email side — `claimed_by` column + topic/instanceID-aware `FetchBatch`

**Files:**
- Create: `services/email/internal/infra/postgres/migrations/20260715120000_add_outbox_claimed_by.sql`
- Modify: `services/email/internal/infra/postgres/outbox_repository.go`
- Modify: `services/email/internal/test/outbox/outbox_test.go`

**Interfaces:**
- Produces: `(*postgres.Client).FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]outbox.Record, error)` — signature changed from today's `FetchBatch(ctx, limit, claimTTL)`. `MarkProcessed`/`MarkFailed` keep their existing signatures and stay on the plain pool (not `db.FromContext`) — see Global Constraints.
- Consumed by Task 3's adapter.

- [ ] **Step 1: Create the migration**

```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE outbox ADD COLUMN claimed_by TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE outbox DROP COLUMN claimed_by;
-- +goose StatementEnd
```

- [ ] **Step 2: Update `FetchBatch` in `services/email/internal/infra/postgres/outbox_repository.go`**

Change:
```go
func (c *Client) FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]outbox.Record, error) {
	cutoff := time.Now().Add(-claimTTL)

	query := `
		UPDATE outbox
		SET status = 'processing', claimed_at = NOW()
		WHERE id IN (
			SELECT id FROM outbox
			WHERE status = 'pending'
			   OR (status = 'processing' AND claimed_at < @cutoff)
			ORDER BY id
			LIMIT @limit
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, topic, key, payload`
	args := pgx.NamedArgs{"cutoff": cutoff, "limit": limit}

	rows, err := c.db.Query(ctx, query, args)
	if err != nil {
		return nil, NewDatabaseError("error claiming outbox batch", err)
	}
	defer rows.Close()

	records, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (outbox.Record, error) {
		var r outbox.Record
		err := row.Scan(&r.ID, &r.Topic, &r.Key, &r.Payload)
		return r, err
	})
	if err != nil {
		return nil, NewDatabaseError("error collecting outbox batch", err)
	}

	return records, nil
}
```
to:
```go
func (c *Client) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]outbox.Record, error) {
	cutoff := time.Now().Add(-claimTTL)

	// claimed_by lets THIS instance immediately reclaim its own stuck
	// "processing" rows (e.g. produce succeeded but MarkProcessed failed)
	// on the very next poll, without waiting for claimTTL — sequential
	// single-instance polling guarantees no other in-flight attempt is
	// still touching it. claimTTL remains the fallback for the case where
	// the instance itself crashed and a new instance id needs to recover
	// orphaned rows left by the old one.
	query := `
		UPDATE outbox
		SET status = 'processing', claimed_at = NOW(), claimed_by = @instanceId
		WHERE id IN (
			SELECT id FROM outbox
			WHERE topic = @topic
			  AND (
			      status = 'pending'
			   OR (status = 'processing' AND claimed_by = @instanceId)
			   OR (status = 'processing' AND claimed_at < @cutoff)
			  )
			ORDER BY id
			LIMIT @limit
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, topic, key, payload`
	args := pgx.NamedArgs{"topic": topic, "instanceId": instanceID, "cutoff": cutoff, "limit": limit}

	rows, err := c.db.Query(ctx, query, args)
	if err != nil {
		return nil, NewDatabaseError("error claiming outbox batch", err)
	}
	defer rows.Close()

	records, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (outbox.Record, error) {
		var r outbox.Record
		err := row.Scan(&r.ID, &r.Topic, &r.Key, &r.Payload)
		return r, err
	})
	if err != nil {
		return nil, NewDatabaseError("error collecting outbox batch", err)
	}

	return records, nil
}
```

`MarkProcessed`/`MarkFailed` in this file are **not modified** — leave them exactly as they are today (plain `c.db.Exec`, no `db.FromContext`).

- [ ] **Step 3: Update existing `FetchBatch` call sites in `services/email/internal/test/outbox/outbox_test.go`**

Every existing call `env.DB.FetchBatch(context.Background(), 10, ...)` (there are 6 of them in this file) becomes `env.DB.FetchBatch(context.Background(), "email-received", "test-instance", 10, ...)` — add `"email-received"` (the topic `seedOutboxEvent` already seeds) and `"test-instance"` (an arbitrary constant instance id, unrelated to the new claimed_by test in Step 4) as the second and third arguments, keeping every other argument unchanged.

- [ ] **Step 4: Add a new test for `claimed_by` behavior**

Append to `services/email/internal/test/outbox/outbox_test.go`:

```go
func TestOutboxRepositoryClaimedBy(t *testing.T) {
	env := utils.NewIntegrationTestEnvironment(t)
	seedOutboxEvent(t, env, "outbox-test-claimed-by")

	records, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(records), "instance-a claims the only pending row")

	t.Run("the same instance immediately reclaims its own stuck row, no TTL wait", func(t *testing.T) {
		batch, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(batch), "instance-a's own processing row is reclaimable immediately, despite claimTTL not having elapsed")
	})

	t.Run("a different instance must still wait out claimTTL", func(t *testing.T) {
		batch, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-b", 10, 30*time.Second)
		assert.Nil(t, err, "instance-b sees a row claimed by instance-a, still within its TTL")
		assert.Equal(t, 0, len(batch), "instance-b must not reclaim another instance's row before its TTL expires")

		expired, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-b", 10, 0*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(expired), "a claimTTL of 0 means instance-b can now reclaim instance-a's expired claim")
	})
}
```

- [ ] **Step 5: Run the tests**

Run: `go build ./services/email/...`
Expected: exit 0.

Run: `go test ./services/email/internal/test/outbox/... -v -count=1`
Expected: PASS — all of `TestOutboxRepository`, `TestOutboxRepositoryMarkFailed`, and the new `TestOutboxRepositoryClaimedBy` (2 subtests) green. Requires Docker running locally.

- [ ] **Step 6: Commit**

```bash
git add services/email/internal/infra/postgres/migrations/20260715120000_add_outbox_claimed_by.sql services/email/internal/infra/postgres/outbox_repository.go services/email/internal/test/outbox/outbox_test.go
git commit -m "feat(email): add claimed_by fast-reclaim and topic/instanceID scoping to outbox FetchBatch"
```

---

### Task 3: `services/email`'s producer adapter

**Files:**
- Create: `services/email/internal/infra/outbox/adapter.go`
- Create: `services/email/internal/infra/outbox/adapter_test.go`
- Modify: `services/email/cmd/email-received-producer/main.go`

**Interfaces:**
- Consumes: `libs/outbox.Producer`, `libs/outbox.Record`, `libs/outbox.NewRunner`/`WithPollInterval` (Task 1); `(*postgres.Client).FetchBatch(ctx, topic, instanceID, limit, claimTTL)`/`MarkProcessed`/`MarkFailed` (Task 2); `libs/events/contracts.EmailReceivedTopic`.
- Produces: `outbox.Repository`, `outbox.EmailReceivedProducerAdapter`, `outbox.NewEmailReceivedProducerAdapter(...)`, `(*EmailReceivedProducerAdapter).PollOnce(ctx) error`. This package does **not** declare its own `Record` type — it reuses `libs/outbox.Record` (Task 1) so that `*postgres.Client`'s `FetchBatch` (which returns `[]libsoutbox.Record`) actually satisfies this package's `Repository` interface. A second, identically-shaped-but-distinct `Record` type here would NOT be interchangeable with `libs/outbox.Record` in Go's type system, even with identical fields — `*postgres.Client` would fail to satisfy `Repository` at compile time.
- **This task resolves Task 1's deliberate compile break** in `cmd/email-received-producer/main.go`.

- [ ] **Step 1: Create `services/email/internal/infra/outbox/adapter.go`**

```go
package outbox

import (
	"context"
	"time"

	libsoutbox "planeo/libs/outbox"
)

// Repository is implemented once per service (satisfied directly by
// *postgres.Client), shared by every producer adapter in the service.
// topic/instanceID are passed per call so one repository instance can
// serve any number of topic-scoped adapters.
type Repository interface {
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsoutbox.Record, error)
	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error
}

// EmailReceivedProducerAdapter owns the claim/produce/mark flow for the
// "email-received" topic. A service with more than one producer gets one
// adapter type per topic, not a single generic parameterized type.
type EmailReceivedProducerAdapter struct {
	repo        Repository
	producer    libsoutbox.Producer
	topic       string
	instanceID  string
	batchSize   int
	claimTTL    time.Duration
	maxAttempts int
}

func NewEmailReceivedProducerAdapter(
	repo Repository,
	producer libsoutbox.Producer,
	topic string,
	instanceID string,
	batchSize int,
	maxAttempts int,
	claimTTL time.Duration,
) *EmailReceivedProducerAdapter {
	return &EmailReceivedProducerAdapter{
		repo:        repo,
		producer:    producer,
		topic:       topic,
		instanceID:  instanceID,
		batchSize:   batchSize,
		claimTTL:    claimTTL,
		maxAttempts: maxAttempts,
	}
}

// PollOnce claims a batch of pending rows for this adapter's topic and
// produces each to Kafka, sequentially. No transaction wraps ProduceSync —
// Kafka isn't enrolled in Postgres's transaction, so wrapping it here would
// buy no atomicity; the known "produce succeeds, mark fails, resend on next
// poll" duplicate-send risk is unchanged from today's Relay.
func (a *EmailReceivedProducerAdapter) PollOnce(ctx context.Context) error {
	records, err := a.repo.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if err := a.producer.ProduceSync(ctx, rec.Topic, rec.Key, rec.Payload); err != nil {
			_ = a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
			continue
		}
		_ = a.repo.MarkProcessed(ctx, rec.ID)
	}

	return nil
}
```
(`libsoutbox` aliases `planeo/libs/outbox` inside this file to avoid any ambiguity with this package's own name, `outbox` — this file never actually needs to refer to its own package by name, but the alias keeps every `Record`/`Producer` reference unambiguous to a reader.)

- [ ] **Step 2: Create `services/email/internal/infra/outbox/adapter_test.go`**

```go
package outbox_test

import (
	"context"
	"errors"
	libsoutbox "planeo/libs/outbox"
	"planeo/services/email/internal/infra/outbox"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeRepository struct {
	mu         sync.Mutex
	records    []libsoutbox.Record
	processed  []int64
	failed     map[int64]int
	maxReached []int64
}

func newFakeRepository(records []libsoutbox.Record) *fakeRepository {
	return &fakeRepository{records: records, failed: map[int64]int{}}
}

func (f *fakeRepository) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsoutbox.Record, error) {
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

func (f *fakeRepository) MarkProcessed(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed = append(f.processed, id)
	return nil
}

func (f *fakeRepository) MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed[id]++
	if f.failed[id] >= maxAttempts {
		f.maxReached = append(f.maxReached, id)
	}
	return nil
}

type fakeProducer struct {
	mu        sync.Mutex
	sent      []libsoutbox.Record
	failTopic string
}

func (f *fakeProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if topic == f.failTopic {
		return errors.New("simulated produce failure")
	}
	f.sent = append(f.sent, libsoutbox.Record{Topic: topic, Key: key, Payload: value})
	return nil
}

func TestEmailReceivedProducerAdapter(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("produces a fetched record and marks it processed", func(t *testing.T) {
		repo := newFakeRepository([]libsoutbox.Record{{ID: 1, Topic: "email-received", Key: []byte("k"), Payload: []byte("v")}})
		producer := &fakeProducer{}
		adapter := outbox.NewEmailReceivedProducerAdapter(repo, producer, "email-received", "instance-a", 10, 5, 30*time.Second)

		err := adapter.PollOnce(context.Background())

		assert.Nil(t, err)
		assert.Equal(t, 1, len(producer.sent))
		assert.Equal(t, []int64{1}, repo.processed)
	})

	t.Run("marks a record failed and quarantines it after max attempts", func(t *testing.T) {
		record := libsoutbox.Record{ID: 2, Topic: "broken-topic", Key: nil, Payload: []byte("v")}
		repo := newFakeRepository([]libsoutbox.Record{record, record, record})
		producer := &fakeProducer{failTopic: "broken-topic"}
		adapter := outbox.NewEmailReceivedProducerAdapter(repo, producer, "broken-topic", "instance-a", 1, 2, 30*time.Second)

		assert.Nil(t, adapter.PollOnce(context.Background()))
		assert.Nil(t, adapter.PollOnce(context.Background()))
		assert.Nil(t, adapter.PollOnce(context.Background()))

		assert.Equal(t, 0, len(producer.sent))
		assert.GreaterOrEqual(t, repo.failed[2], 2)
		assert.Contains(t, repo.maxReached, int64(2))
	})
}
```

- [ ] **Step 3: Run the new unit tests**

Run: `go test ./services/email/internal/infra/outbox/... -v -short -count=1`
Expected: PASS (both subtests green).

- [ ] **Step 4: Update `services/email/cmd/email-received-producer/main.go`**

Change:
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
to:
```go
package main

import (
	"context"
	"os/signal"
	"planeo/libs/events/contracts"
	"planeo/libs/logger"
	"planeo/libs/outbox"
	emailoutbox "planeo/services/email/internal/infra/outbox"
	"planeo/services/email/internal/infra/postgres"
	"strings"
	"syscall"

	"github.com/google/uuid"
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

	adapter := emailoutbox.NewEmailReceivedProducerAdapter(
		db, producer, contracts.EmailReceivedTopic, uuid.NewString(),
		cfg.BatchSize, cfg.MaxAttempts, cfg.ClaimTTL,
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Email-received producer running")
	runner := outbox.NewRunner(outbox.WithPollInterval(cfg.PollInterval))
	if err := runner.Run(runCtx, adapter.PollOnce); err != nil {
		log.Info().Err(err).Msg("Email-received producer stopped")
	}
}
```
(`emailoutbox` aliases `services/email/internal/infra/outbox` because its package name, `outbox`, collides with `libs/outbox`'s — both are needed in this file.)

`*postgres.Client` (the `db` variable) is passed directly as the `Repository` argument — it already satisfies the interface via Task 2's `FetchBatch`/`MarkProcessed`/`MarkFailed`.

- [ ] **Step 5: Verify the whole module compiles**

Run: `go build ./...`
Expected: exit 0 — this is the point where Task 1's deliberate break is fully resolved.

Run: `go vet ./...`
Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add services/email/internal/infra/outbox/ services/email/cmd/email-received-producer/main.go
git commit -m "feat(email): add EmailReceivedProducerAdapter, replacing outbox.Relay"
```

---

### Task 4: Shrink `libs/inbox` — remove `Store`/`Worker`/`Handler`, add generic `Runner`

**Files:**
- Modify: `libs/inbox/store.go` (keep the `Record` struct — the shared transport type across the lib/adapter boundary, same reasoning as `libs/outbox` in Task 1 — remove only the `Store` interface)
- Modify: `libs/inbox/consumer.go` (narrow its dependency off the deleted `Store` interface)
- Delete: `libs/inbox/worker.go`
- Delete: `libs/inbox/worker_test.go`
- Create: `libs/inbox/runner.go`
- Create: `libs/inbox/runner_test.go`

**Interfaces:**
- Produces: `inbox.Runner`, `inbox.NewRunner(opts ...Option) *Runner`, `func (r *Runner) Run(ctx context.Context, poll func(context.Context) error) error`, `inbox.Option`, `inbox.WithPollInterval(d time.Duration) Option`, `inbox.DefaultPollInterval`. `inbox.Record` (unchanged shape, `store.go`) stays — Task 7's adapter and Task 5's repository both consume it as the shared transport type, for the same reason `libs/outbox.Record` stays in Task 1.
- `inbox.Consumer`/`inbox.NewConsumer` (in `consumer.go`) keep their existing behavior, but their `store` field's type narrows (see Step 1).
- **This task deliberately breaks `services/core/internal/infra/events/*.go` and `services/core/cmd/email-received-consumer/main.go`'s compile** (both reference the now-deleted `inbox.Worker`/`inbox.NewWorker`/`inbox.Handler`/`inbox.Store`). Expected; fixed by Task 7.

- [ ] **Step 1: Modify `libs/inbox/store.go` to remove only the `Store` interface**

Change the file from:
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
to:
```go
package inbox

// Record is one durably-persisted inbox row, ready for processing. The
// Store interface that used to live alongside this type has been removed
// — services/core now declares its own Repository interface (see
// services/core/internal/infra/inbox), satisfied directly by
// *postgres.Client. Record itself stays here, shared, so the adapter and
// the repository layer agree on exactly one type.
type Record struct {
	ID      int64
	Topic   string
	Payload []byte
}
```

- [ ] **Step 2: Narrow `Consumer`'s dependency in `libs/inbox/consumer.go`**

`Consumer` currently takes a `store Store` field typed against the interface just removed in Step 1. Change `libs/inbox/consumer.go`'s type declaration from:
```go
type Consumer struct {
	brokers   []string
	groupName string
	topic     string
	store     Store
}

func NewConsumer(brokers []string, groupName, topic string, store Store) *Consumer {
```
to:
```go
// saver is the one method Consumer needs from whatever persists a raw
// Kafka record. Declared here, inline, now that the shared Store interface
// (which used to live in store.go alongside Record) has been removed —
// Consumer's needs are narrower than the removed interface's full shape.
type saver interface {
	Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (inserted bool, err error)
}

type Consumer struct {
	brokers   []string
	groupName string
	topic     string
	store     saver
}

func NewConsumer(brokers []string, groupName, topic string, store saver) *Consumer {
```
No other line in `consumer.go` changes — `c.store.Save(...)` is called exactly as before.

- [ ] **Step 3: Delete `worker.go` and `worker_test.go`**

```bash
git rm libs/inbox/worker.go libs/inbox/worker_test.go
```

- [ ] **Step 4: Create `libs/inbox/runner.go`**

```go
package inbox

import (
	"context"
	"time"

	"planeo/libs/logger"
)

const DefaultPollInterval = 1 * time.Second

// Runner calls a poll function on a fixed interval until ctx is cancelled.
// It has no knowledge of Store, Handler, or any claim/mark logic — those
// now live in services/core's own adapter, which supplies its own poll
// function to Run. Implemented separately from libs/outbox.Runner (same
// shape, not shared) — preserves the existing independently-implemented
// boundary between the two libs.
type Runner struct {
	pollInterval time.Duration
}

type Option func(*Runner)

func WithPollInterval(d time.Duration) Option {
	return func(r *Runner) { r.pollInterval = d }
}

func NewRunner(opts ...Option) *Runner {
	r := &Runner{pollInterval: DefaultPollInterval}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run calls poll on every tick until ctx is cancelled. It blocks the
// calling goroutine. poll's own errors are logged, not returned — a single
// failed poll must not stop the loop.
func (r *Runner) Run(ctx context.Context, poll func(context.Context) error) error {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := poll(ctx); err != nil {
				log.Error().Err(err).Msg("inbox runner poll failed")
			}
		}
	}
}
```

- [ ] **Step 5: Create `libs/inbox/runner_test.go`**

```go
package inbox_test

import (
	"context"
	"errors"
	"planeo/libs/inbox"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunner(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("calls poll on every tick until ctx is cancelled", func(t *testing.T) {
		var calls int32
		poll := func(ctx context.Context) error {
			atomic.AddInt32(&calls, 1)
			return nil
		}
		runner := inbox.NewRunner(inbox.WithPollInterval(10 * time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Millisecond)
		defer cancel()
		err := runner.Run(ctx, poll)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(4))
	})

	t.Run("a poll error does not stop the loop", func(t *testing.T) {
		var calls int32
		poll := func(ctx context.Context) error {
			atomic.AddInt32(&calls, 1)
			return errors.New("simulated poll failure")
		}
		runner := inbox.NewRunner(inbox.WithPollInterval(10 * time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
		defer cancel()
		_ = runner.Run(ctx, poll)

		assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2))
	})
}
```

- [ ] **Step 6: Run the new test and verify `libs/inbox` compiles in isolation**

Run: `go test ./libs/inbox/... -v -short -count=1`
Expected: PASS (both `TestRunner` subtests green).

Run: `go build ./libs/inbox/...`
Expected: exit 0.

- [ ] **Step 7: Commit**

```bash
git add libs/inbox/
git commit -m "refactor(inbox): remove Worker/Store/Handler, add generic Runner"
```

---

### Task 5: Core side — `claimed_by`/`WithTransaction` + transaction-aware `MarkProcessed`/`MarkFailed`

**Files:**
- Create: `services/core/internal/infra/postgres/migrations/20260715120000_add_inbox_claimed_by.sql`
- Modify: `services/core/internal/infra/postgres/inbox_repository.go`
- Modify: `services/core/internal/test/inbox/inbox_test.go`

**Interfaces:**
- Produces: `(*postgres.Client).FetchBatch(ctx context.Context, instanceID string, limit int, claimTTL time.Duration) ([]inbox.Record, error)` — signature changed from today's `FetchBatch(ctx, limit, claimTTL)` (no topic parameter — the approved spec scopes this to instanceID only, unlike the producer side). `(*postgres.Client).WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error` — new method. `MarkProcessed`/`MarkFailed` keep their existing signatures but now resolve their `Querier` via `db.FromContext`.
- Consumed by Task 6 (via `WithTransaction`, indirectly) and Task 7's adapter.

- [ ] **Step 1: Create the migration**

```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE inbox ADD COLUMN claimed_by TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE inbox DROP COLUMN claimed_by;
-- +goose StatementEnd
```

- [ ] **Step 2: Update `services/core/internal/infra/postgres/inbox_repository.go`**

Change the whole file from:
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
to:
```go
package postgres

import (
	"context"
	"errors"
	"planeo/libs/db"
	"planeo/libs/inbox"
	"time"

	"github.com/jackc/pgx/v5"
)

// WithTransaction runs fn within a single database transaction — repository
// methods called with fn's ctx (via db.FromContext) participate in it.
func (c *Client) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return db.WithTx(ctx, c.db, fn)
}

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

func (c *Client) FetchBatch(ctx context.Context, instanceID string, limit int, claimTTL time.Duration) ([]inbox.Record, error) {
	cutoff := time.Now().Add(-claimTTL)

	// Same claimed_by fast-reclaim / claimTTL fallback shape as
	// services/email's outbox FetchBatch — see that file's comment for the
	// full rationale.
	query := `
		UPDATE inbox
		SET status = 'processing', claimed_at = NOW(), claimed_by = @instanceId
		WHERE id IN (
			SELECT id FROM inbox
			WHERE status = 'pending'
			   OR (status = 'processing' AND claimed_by = @instanceId)
			   OR (status = 'processing' AND claimed_at < @cutoff)
			ORDER BY id
			LIMIT @limit
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, topic, payload`
	args := pgx.NamedArgs{"instanceId": instanceID, "cutoff": cutoff, "limit": limit}

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

// MarkProcessed resolves its Querier via db.FromContext: when called with a
// WithTransaction-derived ctx (as the consumer adapter does), this
// participates in that transaction rather than running on a separate,
// auto-committed connection.
func (c *Client) MarkProcessed(ctx context.Context, id int64) error {
	args := pgx.NamedArgs{"id": id}
	q := db.FromContext(ctx, c.db)
	_, err := q.Exec(ctx, `UPDATE inbox SET status = 'processed', processed_at = NOW() WHERE id = @id`, args)
	if err != nil {
		return NewDatabaseError("error marking inbox record processed", err)
	}
	return nil
}

// MarkFailed is always called on a plain (non-transaction) ctx by the
// consumer adapter — see that adapter's own comment for why it must never
// be called on a ctx whose transaction has already errored.
func (c *Client) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	args := pgx.NamedArgs{"id": id, "lastError": procErr.Error(), "maxAttempts": maxAttempts}
	query := `
		UPDATE inbox
		SET attempts = attempts + 1,
		    last_error = @lastError,
		    status = CASE WHEN attempts + 1 >= @maxAttempts THEN 'failed' ELSE 'pending' END,
		    failed_at = CASE WHEN attempts + 1 >= @maxAttempts THEN NOW() ELSE failed_at END
		WHERE id = @id`
	q := db.FromContext(ctx, c.db)
	_, err := q.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error marking inbox record failed", err)
	}
	return nil
}
```

- [ ] **Step 3: Update existing `FetchBatch` call sites in `services/core/internal/test/inbox/inbox_test.go`**

Every existing call `env.DB.FetchBatch(context.Background(), 10, ...)` (there are 6 of them in this file) becomes `env.DB.FetchBatch(context.Background(), "test-instance", 10, ...)` — add `"test-instance"` as the new second argument, keeping every other argument unchanged.

- [ ] **Step 4: Add a new test for `claimed_by` behavior**

Append to `services/core/internal/test/inbox/inbox_test.go`:

```go
func TestInboxRepositoryClaimedBy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)
	_, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
	assert.Nil(t, err)

	records, err := env.DB.FetchBatch(context.Background(), "instance-a", 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(records), "instance-a claims the only pending row")

	t.Run("the same instance immediately reclaims its own stuck row, no TTL wait", func(t *testing.T) {
		batch, err := env.DB.FetchBatch(context.Background(), "instance-a", 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(batch), "instance-a's own processing row is reclaimable immediately, despite claimTTL not having elapsed")
	})

	t.Run("a different instance must still wait out claimTTL", func(t *testing.T) {
		batch, err := env.DB.FetchBatch(context.Background(), "instance-b", 10, 30*time.Second)
		assert.Nil(t, err, "instance-b sees a row claimed by instance-a, still within its TTL")
		assert.Equal(t, 0, len(batch), "instance-b must not reclaim another instance's row before its TTL expires")

		expired, err := env.DB.FetchBatch(context.Background(), "instance-b", 10, 0*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(expired), "a claimTTL of 0 means instance-b can now reclaim instance-a's expired claim")
	})
}
```

Note: `TestInboxRepositoryFetchBatch` and `TestInboxRepositoryMarkFailed` in this file already use `if testing.Short() { t.Skip() }` — `TestInboxRepositoryClaimedBy` matches that same convention (unlike `services/email`'s `outbox_test.go`, which has no such guard on its own tests — follow each file's own existing convention exactly, don't unify them).

- [ ] **Step 5: Run the tests**

Run: `go build ./services/core/internal/infra/postgres/... ./services/core/internal/test/inbox/...`
Expected: exit 0. (Whole-workspace `go build ./...` still fails at this point — Task 4's break in `internal/infra/events`/`cmd/email-received-consumer` isn't resolved until Task 7 — so scope this build to the packages this task actually touches.)

Run: `go test ./services/core/internal/test/inbox/... -v -count=1`
Expected: PASS — all of `TestInboxRepositorySave`, `TestInboxRepositoryFetchBatch`, `TestInboxRepositoryMarkFailed`, and the new `TestInboxRepositoryClaimedBy` (2 subtests) green. Requires Docker running locally.

- [ ] **Step 6: Commit**

```bash
git add services/core/internal/infra/postgres/migrations/20260715120000_add_inbox_claimed_by.sql services/core/internal/infra/postgres/inbox_repository.go services/core/internal/test/inbox/inbox_test.go
git commit -m "feat(core): add WithTransaction, claimed_by fast-reclaim, and transaction-aware Mark* to inbox repository"
```

---

### Task 6: `CreateRequest`/`UpdateRequest` join a caller-supplied transaction

**Files:**
- Modify: `services/core/internal/infra/postgres/request_repository.go`
- Modify: `services/core/internal/test/request/request_test.go`

**Interfaces:**
- No signature changes to `CreateRequest`/`UpdateRequest` — only their internal `Querier` resolution changes.
- Consumed by Task 7's adapter, which calls both inside a `WithTransaction` callback.

- [ ] **Step 1: Update `CreateRequest` and `UpdateRequest` in `services/core/internal/infra/postgres/request_repository.go`**

Add `"planeo/libs/db"` to the file's import block. Change `CreateRequest` from:
```go
func (c *Client) CreateRequest(ctx context.Context, request request.NewRequest) (int, error) {
	query := `
		INSERT INTO requests (text, name, subject, email, address, telephone, raw, closed, reference_id, organization_id, category_id)
		VALUES (@text, @name, @subject, @email, @address, @telephone, @raw, @closed, @referenceId, @organizationId, @categoryId)
		ON CONFLICT (organization_id, reference_id) WHERE reference_id <> '' DO UPDATE SET reference_id = requests.reference_id
		RETURNING id`

	args := pgx.NamedArgs{
		"text":           request.Text,
		"name":           request.Name,
		"subject":        request.Subject,
		"email":          request.Email,
		"address":        request.Address,
		"telephone":      request.Telephone,
		"raw":            request.Raw,
		"referenceId":    request.ReferenceId,
		"closed":         request.Closed,
		"organizationId": request.OrganizationId,
		"categoryId":     nil,
	}

	if request.CategoryId != 0 {
		args["categoryId"] = request.CategoryId
	}

	var id int
	err := c.db.QueryRow(ctx, query, args).Scan(&id)
	if err != nil {
		return 0, NewDatabaseError("error inserting into database", err)
	}

	return id, nil
}
```
to:
```go
func (c *Client) CreateRequest(ctx context.Context, request request.NewRequest) (int, error) {
	query := `
		INSERT INTO requests (text, name, subject, email, address, telephone, raw, closed, reference_id, organization_id, category_id)
		VALUES (@text, @name, @subject, @email, @address, @telephone, @raw, @closed, @referenceId, @organizationId, @categoryId)
		ON CONFLICT (organization_id, reference_id) WHERE reference_id <> '' DO UPDATE SET reference_id = requests.reference_id
		RETURNING id`

	args := pgx.NamedArgs{
		"text":           request.Text,
		"name":           request.Name,
		"subject":        request.Subject,
		"email":          request.Email,
		"address":        request.Address,
		"telephone":      request.Telephone,
		"raw":            request.Raw,
		"referenceId":    request.ReferenceId,
		"closed":         request.Closed,
		"organizationId": request.OrganizationId,
		"categoryId":     nil,
	}

	if request.CategoryId != 0 {
		args["categoryId"] = request.CategoryId
	}

	var id int
	q := db.FromContext(ctx, c.db)
	err := q.QueryRow(ctx, query, args).Scan(&id)
	if err != nil {
		return 0, NewDatabaseError("error inserting into database", err)
	}

	return id, nil
}
```

Change `UpdateRequest` from:
```go
func (c *Client) UpdateRequest(ctx context.Context, req request.UpdateRequest) error {
	query := `
		UPDATE requests
		SET text = @text, name = @name, email = @email, address = @address, telephone = @telephone, closed = @closed, category_id = @categoryId
		WHERE organization_id = @organizationId AND id = @requestId`

	args := pgx.NamedArgs{
		"text":           req.Text,
		"name":           req.Name,
		"email":          req.Email,
		"address":        req.Address,
		"telephone":      req.Telephone,
		"closed":         req.Closed,
		"categoryId":     req.CategoryId,
		"organizationId": req.OrganizationId,
		"requestId":      req.Id,
	}

	if req.CategoryId == 0 {
		args["categoryId"] = nil
	}

	result, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error updating request", err)
	}

	if result.RowsAffected() == 0 {
		return request.RequestNotFoundError
	}

	return nil
}
```
to:
```go
func (c *Client) UpdateRequest(ctx context.Context, req request.UpdateRequest) error {
	query := `
		UPDATE requests
		SET text = @text, name = @name, email = @email, address = @address, telephone = @telephone, closed = @closed, category_id = @categoryId
		WHERE organization_id = @organizationId AND id = @requestId`

	args := pgx.NamedArgs{
		"text":           req.Text,
		"name":           req.Name,
		"email":          req.Email,
		"address":        req.Address,
		"telephone":      req.Telephone,
		"closed":         req.Closed,
		"categoryId":     req.CategoryId,
		"organizationId": req.OrganizationId,
		"requestId":      req.Id,
	}

	if req.CategoryId == 0 {
		args["categoryId"] = nil
	}

	q := db.FromContext(ctx, c.db)
	result, err := q.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error updating request", err)
	}

	if result.RowsAffected() == 0 {
		return request.RequestNotFoundError
	}

	return nil
}
```

`GetRequests`/`GetRequest`/`DeleteRequest` are **not modified** — this migration is scoped to the write path the consumer adapter's transaction needs (`CreateRequest`/`UpdateRequest`), not a wholesale rewrite.

- [ ] **Step 2: Add a test proving both methods participate in a caller-supplied transaction**

Add `"planeo/services/core/internal/domain/category"` to the file's import block if not already present (needed to create a category to reference, so `UpdateRequest`'s `category_id` foreign key succeeds in the positive-path assertion below). Append to `services/core/internal/test/request/request_test.go`:

```go
func TestCreateAndUpdateRequestParticipateInTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)

	t.Run("CreateRequest and UpdateRequest both commit together inside WithTransaction", func(t *testing.T) {
		var requestId int
		err := env.DB.WithTransaction(context.Background(), func(ctx context.Context) error {
			id, err := env.DB.CreateRequest(ctx, request.NewRequest{
				Subject:        "Tx test",
				Text:           "body",
				Email:          "tx@example.com",
				OrganizationId: 1,
				ReferenceId:    "tx-participation-test",
			})
			if err != nil {
				return err
			}
			requestId = id
			return env.DB.UpdateRequest(ctx, request.UpdateRequest{
				Id:             id,
				Text:           "updated body",
				Subject:        "Tx test updated",
				Email:          "tx@example.com",
				OrganizationId: 1,
			})
		})
		assert.Nil(t, err)

		got, err := env.DB.GetRequest(context.Background(), 1, requestId)
		assert.Nil(t, err)
		assert.Equal(t, "updated body", got.Text, "both writes must be visible after a successful transaction")
	})

	t.Run("a failure after CreateRequest rolls back the whole transaction", func(t *testing.T) {
		err := env.DB.WithTransaction(context.Background(), func(ctx context.Context) error {
			_, err := env.DB.CreateRequest(ctx, request.NewRequest{
				Subject:        "Rollback test",
				Text:           "body",
				Email:          "rollback@example.com",
				OrganizationId: 1,
				ReferenceId:    "tx-rollback-test",
			})
			if err != nil {
				return err
			}
			// A nonexistent CategoryId violates the requests.category_id
			// foreign key, forcing a real transaction-aborting error.
			return env.DB.UpdateRequest(ctx, request.UpdateRequest{
				Id:             999999,
				OrganizationId: 1,
				CategoryId:     999999,
			})
		})
		assert.NotNil(t, err, "the forced foreign key violation must surface as an error")

		requests, err := env.DB.GetRequests(context.Background(), 1, 0, 100, false, nil)
		assert.Nil(t, err)
		for _, r := range requests {
			assert.NotEqual(t, "tx-rollback-test", r.ReferenceId, "CreateRequest's row must not survive when the transaction as a whole fails")
		}
	})
}
```

- [ ] **Step 3: Run the tests**

Run: `go build ./services/core/...`
Expected: exit 1, confined to `services/core/internal/infra/events`/`services/core/cmd/email-received-consumer` (Task 4's still-unresolved break — fixed by Task 7, not this task). Confirm the failure is exactly those two packages, nothing else.

Run: `go vet ./services/core/internal/test/request/...`
Expected: exit 0 — confirms this task's own changes compile cleanly, independent of Task 4's break.

Run: `go test ./services/core/internal/test/request/... -v -count=1`
Expected: PASS — all of `TestRequestIntegration`, `TestCreateRequestIdempotency`, and the new `TestCreateAndUpdateRequestParticipateInTransaction` (2 subtests) green. Requires Docker running locally.

- [ ] **Step 4: Commit**

```bash
git add services/core/internal/infra/postgres/request_repository.go services/core/internal/test/request/request_test.go
git commit -m "feat(core): make CreateRequest/UpdateRequest participate in a caller-supplied transaction"
```

---

### Task 7: `services/core`'s consumer adapter

**Files:**
- Delete: `services/core/internal/infra/events/email_received.go`
- Delete: `services/core/internal/infra/events/events.go`
- Create: `services/core/internal/infra/inbox/adapter.go`
- Modify: `services/core/cmd/email-received-consumer/main.go`
- Create: `services/core/internal/test/inbox/adapter_rollback_test.go`

(No unit test file for the new adapter — `adapter_test.go` was tried and retracted; see Step 3's note. `processRecord` calls the real `llm` package directly, so its only test coverage is the integration-level rollback test below, which runs against real Postgres and the real, network-reachable LLM.)

**Interfaces:**
- Consumes: `libs/inbox.Consumer`/`NewConsumer` (unchanged, Task 4), `libs/inbox.NewRunner`/`WithPollInterval` (Task 4), `libs/inbox.Record` (Task 4); `(*postgres.Client).FetchBatch(ctx, instanceID, limit, claimTTL)`/`WithTransaction`/`MarkProcessed`/`MarkFailed` (Task 5); `(*postgres.Client).CreateRequest`/`UpdateRequest` now participating in `WithTransaction` (Task 6); `services/core/internal/domain/request.Service`, `services/core/internal/domain/category.Service`; `services/core/internal/infra/llm.ExtractRequestFields`/`ClassifyRequest`/`RequestData`; `libs/events/contracts.EmailCreatedPayload`/`EmailReceivedTopic`.
- Produces: `inbox.Repository`, `inbox.EmailReceivedConsumerAdapter`, `inbox.NewEmailReceivedConsumerAdapter(...)`, `(*EmailReceivedConsumerAdapter).PollOnce(ctx) error`. This package does **not** declare its own `Record` type — it reuses `libs/inbox.Record` (Task 4), for the same reason `services/email/internal/infra/outbox` reuses `libs/outbox.Record` in Task 3: `*postgres.Client`'s `FetchBatch` (Task 5) returns `[]libsinbox.Record`, and a second, distinct `Record` type here would not satisfy this package's `Repository` interface.
- **This task resolves Task 4's deliberate compile break** in `services/core/internal/infra/events` and `cmd/email-received-consumer/main.go` — by deleting the former entirely (its logic moves into the new adapter) and rewiring the latter.

- [ ] **Step 1: Delete `services/core/internal/infra/events`**

```bash
git rm services/core/internal/infra/events/email_received.go services/core/internal/infra/events/events.go
```
(Confirmed earlier in this plan's brainstorming that `cmd/email-received-consumer/main.go` is this package's only caller anywhere in the repo — deleting it wholesale, not trimming, matches how `libs/events` was deleted wholesale in the earlier inbox-pattern plan once its only caller was rewired.)

- [ ] **Step 2: Create `services/core/internal/infra/inbox/adapter.go`**

```go
package inbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"planeo/libs/events/contracts"
	libsinbox "planeo/libs/inbox"
	"planeo/libs/logger"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
	"planeo/services/core/internal/infra/llm"
)

// Repository is implemented once per service (satisfied directly by
// *postgres.Client).
type Repository interface {
	FetchBatch(ctx context.Context, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error)
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error
}

type EmailReceivedConsumerAdapter struct {
	repo            Repository
	requestService  request.Service
	categoryService category.Service
	instanceID      string
	batchSize       int
	claimTTL        time.Duration
	maxAttempts     int
}

func NewEmailReceivedConsumerAdapter(
	repo Repository,
	requestService request.Service,
	categoryService category.Service,
	instanceID string,
	batchSize int,
	maxAttempts int,
	claimTTL time.Duration,
) *EmailReceivedConsumerAdapter {
	return &EmailReceivedConsumerAdapter{
		repo:            repo,
		requestService:  requestService,
		categoryService: categoryService,
		instanceID:      instanceID,
		batchSize:       batchSize,
		claimTTL:        claimTTL,
		maxAttempts:     maxAttempts,
	}
}

// PollOnce claims a batch of pending inbox rows and processes each in turn.
// One bad record does not stop the rest of the batch.
func (a *EmailReceivedConsumerAdapter) PollOnce(ctx context.Context) error {
	records, err := a.repo.FetchBatch(ctx, a.instanceID, a.batchSize, a.claimTTL)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	for _, rec := range records {
		if err := a.processRecord(ctx, rec); err != nil {
			log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to process inbox record")
		}
	}

	return nil
}

// processRecord gathers everything the write phase needs (categories, LLM
// extraction, LLM classification) BEFORE opening any transaction — these
// are slow, network-dependent calls, and holding a Postgres row lock and a
// pooled connection across them would be an unnecessary cost. Only the
// domain writes and the inbox row's final status are wrapped together.
func (a *EmailReceivedConsumerAdapter) processRecord(ctx context.Context, rec libsinbox.Record) error {
	var payload contracts.EmailCreatedPayload
	if err := json.Unmarshal(rec.Payload, &payload); err != nil {
		return a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
	}

	raw := fmt.Sprintf("Subject: %s\nFrom: %s\nDate: %s\nMessage-ID: %s\nBody: %s",
		payload.Subject, payload.From, payload.Date.Format(time.RFC1123), payload.MessageID, payload.Body)

	categories, err := a.categoryService.GetCategories(ctx, payload.OrganizationId)
	if err != nil {
		return a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
	}

	extractedFields, err := llm.ExtractRequestFields(ctx, raw)
	if err != nil {
		logger.FromContext(ctx).Error().Err(err).Msg("failed to extract fields from request")
		// not fatal - matches CreateInboxHandler's prior behavior exactly
	}

	requestData := llm.RequestData{Subject: payload.Subject, Text: payload.Body}
	categoryId, err := llm.ClassifyRequest(ctx, requestData, categories)
	if err != nil {
		return a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
	}

	// Write-only transaction: CreateRequest/UpdateRequest's writes and
	// MarkProcessed are atomic together. MarkFailed is deliberately NOT
	// called inside this transaction: once any statement in a Postgres
	// transaction errors, the whole transaction is aborted and every
	// subsequent statement on it fails too (SQLSTATE 25P02) — calling
	// MarkFailed(txCtx, ...) after a write error here would itself fail,
	// rolling back and losing the attempts increment entirely.
	txErr := a.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		requestId, err := a.requestService.CreateRequest(txCtx, request.NewRequest{
			Subject:        payload.Subject,
			Raw:            raw,
			Text:           payload.Body,
			Email:          payload.From,
			OrganizationId: payload.OrganizationId,
			ReferenceId:    payload.MessageID,
		})
		if err != nil {
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
		if err := a.requestService.UpdateRequest(txCtx, updatedRequest); err != nil {
			return err
		}

		return a.repo.MarkProcessed(txCtx, rec.ID)
	})
	if txErr != nil {
		return a.repo.MarkFailed(ctx, rec.ID, txErr, a.maxAttempts)
	}
	return nil
}
```

- [ ] **Step 3 (retracted after implementation — see note below): `services/core/internal/infra/inbox/adapter_test.go`**

**This step was implemented, then deliberately removed in a follow-up commit (`d758843`, after `00d6a6e`).** `processRecord` calls `llm.ExtractRequestFields`/`llm.ClassifyRequest` directly as free functions, not through an injectable port — so a unit test exercising `processRecord` makes live network calls to the real Mistral API and requires `MISTRAL_API_KEY` in the process environment to pass, which is not appropriate for a `-short` unit test (non-hermetic, environment-dependent, costs a real API call per run). This is a pre-existing property of the `llm` package's design (the original `CreateEmailReceivedCallback`/`CreateInboxHandler` had the same non-injectable dependency; it was simply never unit-tested before, so the gap never surfaced). Revisit only if a future task makes the LLM calls injectable — do not re-add this test as originally written. The code below is kept for historical reference (what Task 7 originally attempted), not as something to (re)implement:

<details>
<summary>Original Step 3 content (retracted, not to be re-implemented)</summary>


```go
package inbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"planeo/libs/events/contracts"
	libsinbox "planeo/libs/inbox"
	"planeo/services/core/internal/domain/category"
	categorymocks "planeo/services/core/internal/domain/category/mocks"
	"planeo/services/core/internal/domain/request"
	requestmocks "planeo/services/core/internal/domain/request/mocks"
	"planeo/services/core/internal/infra/inbox"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeRepository struct {
	mu          sync.Mutex
	records     []libsinbox.Record
	processed   []int64
	failed      map[int64]int
	txShouldErr bool
}

func newFakeRepository(records []libsinbox.Record) *fakeRepository {
	return &fakeRepository{records: records, failed: map[int64]int{}}
}

func (f *fakeRepository) FetchBatch(ctx context.Context, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error) {
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

func (f *fakeRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if f.txShouldErr {
		return errors.New("simulated transaction failure")
	}
	return fn(ctx)
}

func (f *fakeRepository) MarkProcessed(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed = append(f.processed, id)
	return nil
}

func (f *fakeRepository) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed[id]++
	return nil
}

func mustPayload(t *testing.T, payload contracts.EmailCreatedPayload) []byte {
	t.Helper()
	b, err := json.Marshal(payload)
	assert.Nil(t, err)
	return b
}

func TestEmailReceivedConsumerAdapter(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	payload := mustPayload(t, contracts.EmailCreatedPayload{
		Subject: "Subject", Body: "Body", From: "sender@example.com",
		MessageID: "msg-1", OrganizationId: 1,
	})

	t.Run("processes a record and marks it processed", func(t *testing.T) {
		repo := newFakeRepository([]libsinbox.Record{{ID: 1, Topic: "email-received", Payload: payload}})
		requestService := requestmocks.NewMockService(t)
		categoryService := categorymocks.NewMockService(t)

		categoryService.EXPECT().GetCategories(mock.Anything, 1).Return([]category.Category{}, nil)
		requestService.EXPECT().CreateRequest(mock.Anything, mock.Anything).Return(42, nil)
		requestService.EXPECT().UpdateRequest(mock.Anything, mock.Anything).Return(nil)

		adapter := inbox.NewEmailReceivedConsumerAdapter(repo, requestService, categoryService, "instance-a", 10, 5, 30*time.Second)
		err := adapter.PollOnce(context.Background())

		assert.Nil(t, err)
		assert.Equal(t, []int64{1}, repo.processed)
	})

	t.Run("a transaction failure calls MarkFailed on the plain ctx, after rollback", func(t *testing.T) {
		repo := newFakeRepository([]libsinbox.Record{{ID: 2, Topic: "email-received", Payload: payload}})
		repo.txShouldErr = true
		requestService := requestmocks.NewMockService(t)
		categoryService := categorymocks.NewMockService(t)

		categoryService.EXPECT().GetCategories(mock.Anything, 1).Return([]category.Category{}, nil)

		adapter := inbox.NewEmailReceivedConsumerAdapter(repo, requestService, categoryService, "instance-a", 10, 5, 30*time.Second)
		err := adapter.PollOnce(context.Background())

		assert.Nil(t, err, "PollOnce logs per-record errors, it doesn't return them")
		assert.Equal(t, 0, len(repo.processed))
		assert.Equal(t, 1, repo.failed[2])
	})
}
```

`requestmocks.NewMockService(t)` and `categorymocks.NewMockService(t)` are the confirmed, existing mockery-generated constructors for `request.Service`/`category.Service` (in `services/core/internal/domain/request/mocks/service_mock.go` and `services/core/internal/domain/category/mocks/service_mock.go`) — no name adjustment needed. The `.EXPECT().MethodName(args...).Return(...)` builder shape matches the existing convention already used throughout `services/core/internal/domain/request/service_test.go`. `libsinbox` aliases `planeo/libs/inbox` to avoid ambiguity with this file's own package under test, `planeo/services/core/internal/infra/inbox` (imported unaliased as `inbox`, used for `inbox.NewEmailReceivedConsumerAdapter`).

</details>

- [ ] **Step 4 (retracted along with Step 3 — no longer applicable): ~~Run the new unit tests~~**

Skipped — `services/core/internal/infra/inbox` has no unit test file after the Step 3 retraction. The rollback-atomicity integration test (Step 6) is what actually exercises `processRecord`'s transaction/rollback behavior, against real Postgres and the real (network-reachable) LLM calls — it remains in place and is not affected by this retraction.

- [ ] **Step 5: Update `services/core/cmd/email-received-consumer/main.go`**

Change:
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
to:
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
	coreinbox "planeo/services/core/internal/infra/inbox"
	"planeo/services/core/internal/infra/postgres"
	"strings"
	"syscall"

	"github.com/google/uuid"
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

	instanceID := uuid.NewString()

	adapter := coreinbox.NewEmailReceivedConsumerAdapter(
		db, requestService, categoryService, instanceID,
		cfg.BatchSize, cfg.MaxAttempts, cfg.ClaimTTL,
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	consumer := inbox.NewConsumer(brokers, cfg.GroupName, contracts.EmailReceivedTopic, db)
	if err := consumer.Run(runCtx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start email-received consumer")
	}

	log.Info().Msg("Email-received consumer running")
	runner := inbox.NewRunner(inbox.WithPollInterval(cfg.PollInterval))
	if err := runner.Run(runCtx, adapter.PollOnce); err != nil {
		log.Info().Err(err).Msg("Email-received consumer stopped")
	}
}
```
(`coreinbox` aliases `services/core/internal/infra/inbox` because its package name, `inbox`, collides with `libs/inbox`'s — both are needed in this file.)

- [ ] **Step 6: Add the rollback-atomicity integration test**

Create `services/core/internal/test/inbox/adapter_rollback_test.go`:

```go
package inbox_test

import (
	"context"
	"encoding/json"
	"planeo/libs/events/contracts"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
	coreinbox "planeo/services/core/internal/infra/inbox"
	"planeo/services/core/internal/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// This proves the consumer adapter's write-only transaction really rolls
// back on a real Postgres failure — not just believed correct by
// inspection. It forces UpdateRequest to fail (a nonexistent CategoryId
// violates requests.category_id's foreign key) after CreateRequest has
// already succeeded inside the same transaction, then asserts neither
// write survived and the inbox row is back to pending with attempts
// incremented — mirroring the outbox-architecture-cleanup plan's
// "CreateOutboxEvent failure rolls back CreateMail" test.
func TestEmailReceivedConsumerAdapterRollsBackOnWriteFailure(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)

	payload := contracts.EmailCreatedPayload{
		Subject: "Rollback test", Body: "Body", From: "rollback@example.com",
		MessageID: "rollback-test-message-id", OrganizationId: 1,
	}
	payloadBytes, err := json.Marshal(payload)
	assert.Nil(t, err)

	inserted, err := env.DB.Save(context.Background(), "email-received", 0, 1, payloadBytes)
	assert.Nil(t, err)
	assert.True(t, inserted)

	categoryService := category.NewService(env.DB)
	requestService := forcingUpdateFailureRequestService{Service: request.NewService(env.DB)}

	adapter := coreinbox.NewEmailReceivedConsumerAdapter(env.DB, requestService, categoryService, "instance-a", 10, 5, 30*time.Second)
	err = adapter.PollOnce(context.Background())
	assert.Nil(t, err, "PollOnce logs per-record errors, it doesn't return them")

	requests, err := env.DB.GetRequests(context.Background(), 1, 0, 100, false, nil)
	assert.Nil(t, err)
	for _, r := range requests {
		assert.NotEqual(t, "rollback-test-message-id", r.ReferenceId, "CreateRequest's row must not survive when UpdateRequest fails in the same transaction")
	}

	batch, err := env.DB.FetchBatch(context.Background(), "instance-a", 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(batch), "the inbox row must be back to pending (immediately reclaimable by the same instance) after the transaction rolled back")
}

// forcingUpdateFailureRequestService wraps the real request.Service, only
// overriding UpdateRequest to inject a foreign-key-violating CategoryId -
// forcing a genuine Postgres error inside the adapter's transaction,
// rather than a fabricated one.
type forcingUpdateFailureRequestService struct {
	request.Service
}

func (f forcingUpdateFailureRequestService) UpdateRequest(ctx context.Context, req request.UpdateRequest) error {
	req.CategoryId = 999999
	return f.Service.UpdateRequest(ctx, req)
}
```

- [ ] **Step 7: Verify the whole module compiles and run the tests**

Run: `go build ./...`
Expected: exit 0 — this is the point where Task 4's deliberate break is fully resolved.

Run: `go vet ./...`
Expected: exit 0.

Run: `go test ./services/core/internal/test/inbox/... -v -count=1`
Expected: PASS — all pre-existing inbox repository tests, plus the new `TestEmailReceivedConsumerAdapterRollsBackOnWriteFailure`, green. Requires Docker running locally. (No `go test ./services/core/internal/infra/inbox/...` step — that package has no test file after Step 3's retraction.)

- [ ] **Step 8: Commit**

```bash
git add services/core/internal/infra/events services/core/internal/infra/inbox services/core/cmd/email-received-consumer/main.go services/core/internal/test/inbox/adapter_rollback_test.go
git commit -m "feat(core): add EmailReceivedConsumerAdapter, replacing inbox.Worker and CreateInboxHandler"
```

(In the actual execution history, `adapter_test.go` was committed as part of this step and then removed in a separate follow-up commit once its non-hermetic Mistral-API dependency was identified — see Step 3's note. A future implementer following this plan fresh should simply not create `adapter_test.go` at all, skipping straight from Step 2 to Step 5.)

---

### Task 8: Full workspace verification

**Files:** none (verification only)

- [ ] **Step 1: Build, vet, and format-check the entire module**

Run: `go build ./...`
Expected: exit 0.

Run: `go vet ./...`
Expected: exit 0.

Run: `gofmt -l .`
Expected: no output, except the already-known pre-existing drift in `services/email/internal/infra/rest/api/errors.go` (untouched by this plan — leave it alone, per the precedent already established in the inbox-pattern plan's own Task 8).

- [ ] **Step 2: Confirm no stray references to removed names**

Run:
```bash
grep -rn "outbox\.Relay\|outbox\.NewRelay\|outbox\.Store\b" --include="*.go" .
```
Expected: no output (`Relay`/`Store` fully removed from `libs/outbox`).

Run:
```bash
grep -rn "inbox\.Worker\|inbox\.NewWorker\|inbox\.Handler\|inbox\.Store\b" --include="*.go" .
```
Expected: no output (`Worker`/`Handler`/`Store` fully removed from `libs/inbox`).

Run:
```bash
grep -rn "CreateInboxHandler\|coreEvents\." --include="*.go" .
```
Expected: no output (`internal/infra/events` fully deleted, nothing references it).

Run:
```bash
ls services/core/internal/infra/events 2>&1
```
Expected: `No such file or directory`.

- [ ] **Step 3: Run every affected test suite**

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

- [ ] **Step 4: Commit (only if Steps 1-2 required fixes; otherwise skip — do not create an empty commit)**

```bash
git add -A
git commit -m "chore: cleanup fixes from final verification"
```
