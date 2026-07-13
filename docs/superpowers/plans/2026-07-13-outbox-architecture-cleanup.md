# Outbox Pattern Architecture Cleanup — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the already-working transactional outbox pattern for `services/email` along cleaner architectural boundaries: extract the Kafka event contract into its own dependency-free package, move event construction out of the IMAP-fetching adapter into the domain layer, and replace the repository's hidden internal transaction with a service-orchestrated one via a new, reusable `libs/db` utility.

**Architecture:** `libs/events/contracts` becomes the single source of truth for the `email-received` event's wire shape, importable by both the Kafka pub/sub library (`libs/events`) and the domain layer (`domain/mail`) without pulling a Kafka-client dependency into the latter. `domain/mail`'s `Repository` port changes from one combined method to three (`CreateMail`, `CreateOutboxEvent`, `WithTransaction`), so the *service* — not the repository — orchestrates which writes happen atomically together, via a new `libs/db.WithTx`/`FromContext` ambient-transaction utility. `services/email/internal/infra/email/email_service.go` drops all Kafka/JSON knowledge, becoming a pure "fetch mail, hand raw data to the domain" adapter.

**Tech Stack:** Go, pgx/v5, mockery + testify, testcontainers-go.

## Global Constraints

- Reference spec: `docs/superpowers/specs/2026-07-12-outbox-architecture-cleanup-design.md`.
- No behavior change to outbox semantics (claim/TTL, poison-row quarantine, ordering) — all unchanged, already covered by existing tests from the prior plan.
- `services/core`'s subscribe side and `libs/events`'s public API (`PublishEmailReceived`/`SubscribeEmailReceived` signatures) are unaffected in shape — only the payload type's import path changes (from `events.EmailCreatedPayload` to `contracts.EmailCreatedPayload`).
- No retrofit of `setting_repository.go` or any `services/core` repository onto `WithTx`/`FromContext` — scoped to `services/email`'s mail+outbox repositories only.
- `outbox_repository.go`'s relay-facing methods (`FetchBatch`, `MarkProcessed`, `MarkFailed`) are unchanged — they keep calling `c.db` directly; only the new `CreateOutboxEvent` method goes through `db.FromContext`.
- Branch: `feature/email-outbox-pattern` (already checked out, per user instruction to stay on it for this cleanup). Do not commit to `main`.

---

### Task 1: Extract `libs/events/contracts` package

**Files:**
- Create: `libs/events/contracts/contracts.go`
- Modify: `libs/events/email_received.go`
- Modify: `services/core/internal/infra/events/email_received.go`
- Modify: `services/email/internal/infra/email/email_service.go` (stopgap import swap only, superseded by Task 6's full rewrite)

**Interfaces:**
- Produces: `contracts.EmailCreatedPayload`, `contracts.EmailReceivedTopic` — consumed by Task 4 (`domain/mail`) and this task's own updates to `libs/events`/`services/core`/`services/email`.

- [ ] **Step 1: Create the contracts package**

```go
// libs/events/contracts/contracts.go
package contracts

import "time"

// EmailCreatedPayload is the wire-format contract for the "email received"
// domain event: the shape a producer (services/email's outbox) serializes
// into an outbox row's payload, and a consumer (services/core's Kafka
// subscribe handler) deserializes back out. Kept in its own package, with
// no dependencies beyond the standard library, so a producer can depend on
// it without pulling in any Kafka client code.
type EmailCreatedPayload struct {
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	From           string    `json:"from"`
	Date           time.Time `json:"date"`
	MessageID      string    `json:"messageId"`
	OrganizationId int       `json:"organizationId"`
}

// EmailReceivedTopic is the Kafka topic both the outbox producer and the
// libs/events subscriber use for EmailCreatedPayload events.
const EmailReceivedTopic = "email-received"
```

- [ ] **Step 2: Update `libs/events/email_received.go` to use the contracts package**

Replace the whole file's content with:

```go
package events

import (
	"context"
	"encoding/json"
	"planeo/libs/events/contracts"
)

var subscriptionName = "email-receiver"

func (es *EventService) PublishEmailReceived(ctx context.Context, payload contracts.EmailCreatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return es.Publish(ctx, contracts.EmailReceivedTopic, data)
}

func (es *EventService) SubscribeEmailReceived(ctx context.Context, handler func(contracts.EmailCreatedPayload) error) error {
	return es.Subscribe(ctx, subscriptionName, contracts.EmailReceivedTopic, func(data []byte) error {
		var payload contracts.EmailCreatedPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			return err
		}

		return handler(payload)
	})
}
```

- [ ] **Step 3: Update `services/core/internal/infra/events/email_received.go`**

Change the import block (lines 3-12) from:
```go
import (
	"context"
	"fmt"
	"planeo/libs/events"
	"planeo/libs/logger"

	"planeo/services/core/internal/domain/request"
	"planeo/services/core/internal/infra/llm"
	"time"
)
```
to:
```go
import (
	"context"
	"fmt"
	"planeo/libs/events/contracts"
	"planeo/libs/logger"

	"planeo/services/core/internal/domain/request"
	"planeo/services/core/internal/infra/llm"
	"time"
)
```

Change both occurrences of `events.EmailCreatedPayload` (in the function signature on line 15 and the returned closure's parameter on line 18) to `contracts.EmailCreatedPayload`:

```go
//nolint:funlen
func CreateEmailReceivedCallback(ctx context.Context, services Services) func(payload contracts.EmailCreatedPayload) error {
	logger := logger.FromContext(ctx)

	return func(payload contracts.EmailCreatedPayload) error {
```

The rest of the function body is unchanged — it only reads fields off `payload`, which have identical names/types in `contracts.EmailCreatedPayload`.

`services/core/internal/infra/events/events.go` needs no changes — it never references the payload type by name, only `CreateEmailReceivedCallback(...)`'s return value, which still type-matches `SubscribeEmailReceived`'s parameter after this change.

- [ ] **Step 4: Stopgap-fix `services/email/internal/infra/email/email_service.go`'s two references**

This file also references `events.EmailCreatedPayload`/`events.EmailReceivedTopic` (lines 6, 100, 124) and would otherwise stop compiling the moment Task 1 lands — Task 6 doesn't rewrite this file until later. This is a temporary import swap only, superseded (import dropped entirely) by Task 6.

In the import block, replace:
```go
	"planeo/libs/events"
```
with:
```go
	"planeo/libs/events/contracts"
```

Then change the two usages inside `createTask`'s loop:
```go
			payload, err := json.Marshal(contracts.EmailCreatedPayload{
```
and:
```go
				Event: mail.OutboxEvent{
					Topic:   contracts.EmailReceivedTopic,
```

- [ ] **Step 5: Verify it compiles and core's existing tests still pass**

Run: `go build ./libs/events/... ./services/core/... ./services/email/...`
Expected: exit 0 (includes `services/email` because of the Step 4 stopgap fix).

Run: `task test:core:unit`
Expected: PASS (no test currently exercises `PublishEmailReceived`/`SubscribeEmailReceived`/`CreateEmailReceivedCallback` directly — this is a pre-existing gap, not something this task introduces — so this is a compile-correctness confirmation, not new behavioral coverage).

- [ ] **Step 6: Commit**

```bash
git add libs/events/contracts/contracts.go libs/events/email_received.go services/core/internal/infra/events/email_received.go services/email/internal/infra/email/email_service.go
git commit -m "refactor(events): extract EmailCreatedPayload/EmailReceivedTopic into libs/events/contracts"
```

---

### Task 2: Add `WithTx`/`Querier`/`FromContext` to `libs/db` + integration test

**Files:**
- Modify: `libs/db/db.go`
- Create: `libs/db/db_test.go`

**Interfaces:**
- Produces: `db.Querier`, `db.WithTx(ctx, pool, fn) error`, `db.FromContext(ctx, pool) Querier` — consumed by Task 5 (`mail_repository.go`, `outbox_repository.go`).

- [ ] **Step 1: Add `Querier`, `WithTx`, and `FromContext` to `libs/db/db.go`**

Add these imports to the existing import block (`"github.com/jackc/pgx/v5"` and `"github.com/jackc/pgx/v5/pgconn"`, alongside the existing `pgxpool` import):

```go
import (
	"context"
	"planeo/libs/logger"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool" // Standard library bindings for pgx
)
```

Append this to the end of the file (after the existing `pingDatabase` function):

```go
// Querier is satisfied by both *pgxpool.Pool and pgx.Tx, letting
// repository code run the same query methods whether or not it's
// currently inside a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// WithTx runs fn inside a single database transaction on pool, committing
// if fn returns nil and rolling back otherwise. Repository code that calls
// FromContext(ctx, pool) using the ctx passed to fn transparently
// participates in this same transaction.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FromContext returns the pgx.Tx stored in ctx by WithTx, or pool if ctx
// carries no transaction.
func FromContext(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
```

- [ ] **Step 2: Write the integration test**

`libs/db` has no existing test infrastructure — this is a self-contained integration-style test using its own throwaway testcontainer and scratch table, independent of any service's schema/migrations. It deliberately has no `-short` guard (unlike the unit tests in `services/*`), matching how integration tests elsewhere in this repo (`internal/test/...`) always run rather than being skipped — running it requires Docker.

```go
// libs/db/db_test.go
package db_test

import (
	"context"
	"errors"
	"planeo/libs/db"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:alpine3.20",
		postgres.WithDatabase("db_test"),
		postgres.WithUsername("planeo"),
		postgres.WithPassword("planeo"),
		testcontainers.WithWaitStrategyAndDeadline(5*time.Minute,
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	assert.Nil(t, err)

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	assert.Nil(t, err)

	pool, err := pgxpool.New(ctx, connString)
	assert.Nil(t, err)

	_, err = pool.Exec(ctx, `CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
	assert.Nil(t, err)

	t.Cleanup(func() {
		pool.Close()
		_ = container.Terminate(ctx)
	})

	return pool
}

func TestWithTx(t *testing.T) {
	pool := startTestPool(t)

	t.Run("commits when fn returns nil", func(t *testing.T) {
		err := db.WithTx(context.Background(), pool, func(ctx context.Context) error {
			q := db.FromContext(ctx, pool)
			_, err := q.Exec(ctx, `INSERT INTO widgets (id, name) VALUES (1, 'committed')`)
			return err
		})
		assert.Nil(t, err)

		var name string
		err = pool.QueryRow(context.Background(), `SELECT name FROM widgets WHERE id = 1`).Scan(&name)
		assert.Nil(t, err)
		assert.Equal(t, "committed", name)
	})

	t.Run("rolls back when fn returns an error", func(t *testing.T) {
		sentinel := errors.New("boom")
		err := db.WithTx(context.Background(), pool, func(ctx context.Context) error {
			q := db.FromContext(ctx, pool)
			if _, err := q.Exec(ctx, `INSERT INTO widgets (id, name) VALUES (2, 'rolled-back')`); err != nil {
				return err
			}
			return sentinel
		})
		assert.ErrorIs(t, err, sentinel)

		var count int
		err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM widgets WHERE id = 2`).Scan(&count)
		assert.Nil(t, err)
		assert.Equal(t, 0, count, "a rolled-back transaction must not leave any row behind")
	})
}

func TestFromContext(t *testing.T) {
	pool := startTestPool(t)

	t.Run("returns the pool when ctx carries no transaction", func(t *testing.T) {
		q := db.FromContext(context.Background(), pool)
		assert.Equal(t, pool, q)
	})

	t.Run("returns the transaction when ctx carries one", func(t *testing.T) {
		err := db.WithTx(context.Background(), pool, func(ctx context.Context) error {
			q := db.FromContext(ctx, pool)
			assert.NotEqual(t, pool, q, "FromContext must return the tx, not the pool, once inside WithTx")
			return nil
		})
		assert.Nil(t, err)
	})
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./libs/db/... -v -count=1`
Expected: PASS, all subtests green. (Requires Docker running locally, for testcontainers.)

- [ ] **Step 4: Verify the rest of the module still builds**

Run: `go build ./...`
Expected: exit 0 (this confirms the new imports in `db.go` don't break anything already depending on `libs/db`, e.g. `services/core`'s Postgres client).

- [ ] **Step 5: Commit**

```bash
git add libs/db/db.go libs/db/db_test.go
git commit -m "feat(db): add WithTx/Querier/FromContext ambient-transaction utility"
```

---

### Task 3: Migrate `services/email`'s Postgres `Client` onto `libs/db`

**Files:**
- Modify: `services/email/internal/infra/postgres/client.go`

**Interfaces:**
- Consumes: `db.InitializeDatabaseConnection` (already exists, used identically by `services/core`).
- Produces: no change to `Client`'s public shape (`db *pgxpool.Pool` field name/type, `NewClient`/`Close` signatures) — only the internal construction path changes, so nothing downstream needs touching in this task.

- [ ] **Step 1: Replace the file's content**

```go
package postgres

import (
	"context"
	"planeo/libs/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	db *pgxpool.Pool
}

func NewClient(ctx context.Context, connString string) *Client {
	conn := db.InitializeDatabaseConnection(ctx, connString)
	return &Client{db: conn.DB}
}

func (c *Client) Close() {
	c.db.Close()
}
```

This mirrors `services/core/internal/infra/postgres/client.go`'s existing pattern exactly. One behavior change worth noting: `db.InitializeDatabaseConnection` starts a background goroutine that pings the database every 20 seconds and `panic`s the process after 5 consecutive failures — `services/email`'s main service and its `cmd/outbox-relay` sidecar (which both call `postgres.NewClient`) now get this health-check behavior for free, matching what `services/core` already has. Previously, `services/email`'s connection-failure path called `logger.Fatal()` (clean `os.Exit`); it's now `panic()` (via `libs/db`), consistent with `services/core`'s existing behavior.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./services/email/...`
Expected: exit 0. (`setting_repository.go`, `mail_repository.go`, `outbox_repository.go` all continue to compile unchanged — `Client.db`'s type didn't change, only how it's constructed.)

- [ ] **Step 3: Run existing email tests to confirm no regression**

Run: `task test:email:unit && task test:email:integration`
Expected: both PASS (requires Docker for the integration half). This is the last point before Tasks 4-5 land, so these should behave exactly as they did before this cleanup started.

- [ ] **Step 4: Commit**

```bash
git add services/email/internal/infra/postgres/client.go
git commit -m "refactor(email): migrate postgres.Client onto libs/db.InitializeDatabaseConnection"
```

---

### Task 4: Rewrite `domain/mail` — `RawFetchedMail` input model, split `Repository` port, service-level transaction orchestration

**Files:**
- Modify: `services/email/internal/domain/mail/model.go`
- Modify: `services/email/internal/domain/mail/ports.go`
- Modify: `services/email/internal/domain/mail/service.go`
- Modify: `services/email/internal/domain/mail/service_test.go`
- Regenerate (via mockery): `services/email/internal/domain/mail/mocks/repository_mock.go`, `services/email/internal/domain/mail/mocks/service_mock.go`

**Interfaces:**
- Consumes: `contracts.EmailCreatedPayload`, `contracts.EmailReceivedTopic` from Task 1.
- Produces: `mail.RawFetchedMail`, `mail.NewMail`, `mail.OutboxEvent`, `mail.SaveResult` (unchanged), `mail.Repository{CreateMail, CreateOutboxEvent, WithTransaction}`, `mail.Service.SaveFetchedMails(ctx, raws []RawFetchedMail) ([]SaveResult, error)` — consumed by Task 5 (Postgres repository implementing the port) and Task 6 (`email_service.go` calling the service).

This task is self-contained and independently testable — the domain layer has no infrastructure imports, so it compiles and its unit tests run without Task 5's Postgres changes existing yet.

- [ ] **Step 1: Rewrite `model.go`**

```go
package mail

import "time"

type Mail struct {
	ID             int       `db:"id"`
	MessageID      string    `db:"message_id"`
	SettingID      int       `db:"setting_id"`
	OrganizationID int       `db:"organization_id"`
	Subject        string    `db:"subject"`
	Sender         string    `db:"sender"`
	Body           string    `db:"body"`
	Date           time.Time `db:"date"`
	CreatedAt      time.Time `db:"created_at"`
}

type NewMail struct {
	MessageID      string
	SettingID      int
	OrganizationID int
	Subject        string
	Sender         string
	Body           string
	Date           time.Time
}

// OutboxEvent is the Kafka event to be durably queued alongside a NewMail,
// in the same local transaction.
type OutboxEvent struct {
	Topic   string
	Key     []byte
	Payload []byte
}

// RawFetchedMail is the raw data a fetch-side adapter (e.g. IMAP) hands to
// this domain — it carries no knowledge of Kafka, topics, or event
// payloads; SaveFetchedMails builds those internally.
type RawFetchedMail struct {
	MessageID      string
	SettingID      int
	OrganizationID int
	Subject        string
	Sender         string
	Body           string
	Date           time.Time
	UID            uint32
}

// SaveResult reports, per fetched mail, its IMAP UID (so the caller can
// mark it seen) and whether this call actually inserted a new mails row
// (false means it was already recorded from a prior attempt — still safe
// to mark seen, but no new outbox event was created for it this time).
type SaveResult struct {
	UID      uint32
	Inserted bool
}
```

Note: the old `FetchedMail{Mail, Event, UID}` bundle type is removed — nothing needs it anymore, since the service now builds `NewMail`/`OutboxEvent` internally per `RawFetchedMail` rather than receiving them pre-built.

- [ ] **Step 2: Rewrite `ports.go`**

```go
package mail

import "context"

type Repository interface {
	// CreateMail inserts a mail row, returning its id, whether it was
	// newly inserted (false on a duplicate setting_id+message_id, a safe
	// no-op), and any error. Must be called within WithTransaction
	// alongside CreateOutboxEvent to keep both writes atomic.
	CreateMail(ctx context.Context, m NewMail) (mailID int, inserted bool, err error)

	// CreateOutboxEvent inserts the outbox row for a given mail.
	CreateOutboxEvent(ctx context.Context, mailID int, event OutboxEvent) error

	// WithTransaction runs fn within a single database transaction,
	// committing on success and rolling back on error. Repository calls
	// made using the ctx passed to fn participate in that same
	// transaction.
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service interface {
	SaveFetchedMails(ctx context.Context, raws []RawFetchedMail) ([]SaveResult, error)
}
```

- [ ] **Step 3: Rewrite `service.go`**

```go
package mail

import (
	"context"
	"encoding/json"
	"planeo/libs/events/contracts"
	"strconv"
)

type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{repository: repository}
}

func (s *service) SaveFetchedMails(ctx context.Context, raws []RawFetchedMail) ([]SaveResult, error) {
	if len(raws) == 0 {
		return nil, nil
	}

	var results []SaveResult
	err := s.repository.WithTransaction(ctx, func(ctx context.Context) error {
		for _, raw := range raws {
			newMail, event, err := buildMailAndEvent(raw)
			if err != nil {
				return err
			}

			mailID, inserted, err := s.repository.CreateMail(ctx, newMail)
			if err != nil {
				return err
			}

			if inserted {
				if err := s.repository.CreateOutboxEvent(ctx, mailID, event); err != nil {
					return err
				}
			}

			results = append(results, SaveResult{UID: raw.UID, Inserted: inserted})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

func buildMailAndEvent(raw RawFetchedMail) (NewMail, OutboxEvent, error) {
	newMail := NewMail{
		MessageID:      raw.MessageID,
		SettingID:      raw.SettingID,
		OrganizationID: raw.OrganizationID,
		Subject:        raw.Subject,
		Sender:         raw.Sender,
		Body:           raw.Body,
		Date:           raw.Date,
	}

	payload, err := json.Marshal(contracts.EmailCreatedPayload{
		Subject:        raw.Subject,
		Body:           raw.Body,
		From:           raw.Sender,
		Date:           raw.Date,
		MessageID:      raw.MessageID,
		OrganizationId: raw.OrganizationID,
	})
	if err != nil {
		return NewMail{}, OutboxEvent{}, err
	}

	event := OutboxEvent{
		Topic:   contracts.EmailReceivedTopic,
		Key:     []byte(strconv.Itoa(raw.OrganizationID)),
		Payload: payload,
	}

	return newMail, event, nil
}
```

Note on error handling: if `buildMailAndEvent` ever fails (in practice, `json.Marshal` on this plain struct of strings/int/time.Time cannot realistically fail — there's no cyclic reference, channel, or function value in it), the whole `WithTransaction` call aborts and rolls back, so no mail in that poll cycle gets saved or marked seen this time — the entire batch is safely retried on the next poll. This is a deliberate simplification versus the pre-cleanup code, which skipped just the one bad mail and kept processing the rest; given the failure mode is not realistically reachable for this payload shape, the simpler all-or-nothing transaction semantics were chosen over reproducing that per-mail skip logic.

- [ ] **Step 4: Regenerate mocks**

Run: `cd services/email && mockery && cd -`
Expected: `mocks/repository_mock.go` regenerates with `MockRepository` now having `CreateMail`, `CreateOutboxEvent`, `WithTransaction` methods (and their `EXPECT()`/`RunAndReturn` builders); `mocks/service_mock.go` regenerates with `MockService.SaveFetchedMails` taking `[]RawFetchedMail`.

- [ ] **Step 5: Rewrite `service_test.go`**

```go
package mail_test

import (
	"context"
	. "planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/domain/mail/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMailService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	raws := []RawFetchedMail{
		{
			MessageID:      "abc123",
			SettingID:      1,
			OrganizationID: 1,
			Subject:        "Test",
			Sender:         "sender@example.com",
			Body:           "body",
			Date:           time.Now(),
			UID:            42,
		},
	}

	t.Run("SaveFetchedMails", func(t *testing.T) {
		t.Run("creates the mail and outbox event when the mail is newly inserted", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRepository.EXPECT().CreateMail(context.Background(), mock.AnythingOfType("NewMail")).Return(1, true, nil)
			mockRepository.EXPECT().CreateOutboxEvent(context.Background(), 1, mock.AnythingOfType("OutboxEvent")).Return(nil)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), raws)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.True(t, results[0].Inserted)
			assert.Equal(t, uint32(42), results[0].UID)
		})

		t.Run("does not create an outbox event when the mail already exists", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRepository.EXPECT().CreateMail(context.Background(), mock.AnythingOfType("NewMail")).Return(0, false, nil)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), raws)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.False(t, results[0].Inserted)
			// CreateOutboxEvent has no .EXPECT() set above; mockery's generated
			// mock fails the test via its registered t.Cleanup assertion if an
			// unexpected call happens, which is what proves it was never called.
		})

		t.Run("returns error when CreateMail fails", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRepository.EXPECT().CreateMail(context.Background(), mock.AnythingOfType("NewMail")).Return(0, false, assert.AnError)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), raws)
			assert.Error(t, err)
			assert.Nil(t, results)
		})

		t.Run("returns nil without calling repository when raws is empty", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), []RawFetchedMail{})
			assert.Nil(t, err)
			assert.Nil(t, results)
		})
	})
}
```

- [ ] **Step 6: Run the test**

Run: `go test ./services/email/internal/domain/mail/... -v -short -count=1`
Expected: PASS, all 4 subtests green.

- [ ] **Step 7: Verify the package compiles standalone**

Run: `go build ./services/email/internal/domain/mail/...`
Expected: exit 0. (A full `go build ./services/email/...` will legitimately fail at this point — `mail_repository.go`, `outbox_repository.go`, and `email_service.go` all still reference the now-removed `FetchedMail`/old `Repository.SaveFetchedMails` shape. That's expected; Tasks 5-6 fix it.)

- [ ] **Step 8: Commit**

```bash
git add services/email/internal/domain/mail/
git commit -m "refactor(email): move event construction into domain/mail, split Repository into CreateMail/CreateOutboxEvent/WithTransaction"
```

---

### Task 5: Rewrite `mail_repository.go` + `outbox_repository.go` — `CreateMail`, `CreateOutboxEvent`, `WithTransaction` + integration tests

**Files:**
- Modify: `services/email/internal/infra/postgres/mail_repository.go`
- Modify: `services/email/internal/infra/postgres/outbox_repository.go`
- Modify: `services/email/internal/test/mail/mail_test.go`
- Modify: `services/email/internal/test/outbox/outbox_test.go`

**Interfaces:**
- Consumes: `db.WithTx`, `db.FromContext` (Task 2); `mail.NewMail`, `mail.OutboxEvent`, `mail.Repository` (Task 4).
- Produces: `(*postgres.Client)` satisfying `mail.Repository` via `CreateMail`, `CreateOutboxEvent`, `WithTransaction` — consumed by Task 6's `cmd/main.go` wiring (unchanged call site, since `mail.NewService(db)` still just needs `db` to satisfy `Repository`, whatever its method set).

- [ ] **Step 1: Rewrite `mail_repository.go`**

```go
package postgres

import (
	"context"
	"errors"
	"planeo/libs/db"
	"planeo/services/email/internal/domain/mail"

	"github.com/jackc/pgx/v5"
)

func (c *Client) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return db.WithTx(ctx, c.db, fn)
}

func (c *Client) CreateMail(ctx context.Context, m mail.NewMail) (int, bool, error) {
	query := `
		INSERT INTO mails (message_id, setting_id, organization_id, subject, sender, body, date)
		VALUES (@messageId, @settingId, @organizationId, @subject, @sender, @body, @date)
		ON CONFLICT (setting_id, message_id) DO NOTHING
		RETURNING id`
	args := pgx.NamedArgs{
		"messageId":      m.MessageID,
		"settingId":      m.SettingID,
		"organizationId": m.OrganizationID,
		"subject":        m.Subject,
		"sender":         m.Sender,
		"body":           m.Body,
		"date":           m.Date,
	}

	q := db.FromContext(ctx, c.db)
	row, err := q.Query(ctx, query, args)
	if err != nil {
		return 0, false, NewDatabaseError("error inserting mail", err)
	}

	mailID, err := pgx.CollectExactlyOneRow(row, pgx.RowTo[int])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, NewDatabaseError("error collecting inserted mail id", err)
	}

	return mailID, true, nil
}
```

- [ ] **Step 2: Add `CreateOutboxEvent` to `outbox_repository.go`**

Add this import to the existing import block (alongside `"planeo/libs/outbox"`):
```go
import (
	"context"
	"planeo/libs/db"
	"planeo/libs/outbox"
	"planeo/services/email/internal/domain/mail"
	"time"

	"github.com/jackc/pgx/v5"
)
```

Add this new method to the file (the existing `FetchBatch`, `MarkProcessed`, `MarkFailed` methods are unchanged — leave them exactly as they are):

```go
func (c *Client) CreateOutboxEvent(ctx context.Context, mailID int, event mail.OutboxEvent) error {
	query := `
		INSERT INTO outbox (mail_id, topic, key, payload)
		VALUES (@mailId, @topic, @key, @payload)`
	args := pgx.NamedArgs{
		"mailId":  mailID,
		"topic":   event.Topic,
		"key":     event.Key,
		"payload": event.Payload,
	}

	q := db.FromContext(ctx, c.db)
	if _, err := q.Exec(ctx, query, args); err != nil {
		return NewDatabaseError("error inserting outbox event", err)
	}
	return nil
}
```

- [ ] **Step 3: Rewrite `mail_test.go`**

```go
package mail_test

import (
	"context"
	"planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMailRepository(t *testing.T) {
	env := utils.NewIntegrationTestEnvironment(t)

	newMail := mail.NewMail{
		MessageID:      "duplicate-test-1",
		SettingID:      1,
		OrganizationID: 1,
		Subject:        "Test Subject",
		Sender:         "sender@example.com",
		Body:           "Test body",
		Date:           time.Now(),
	}
	event := mail.OutboxEvent{
		Topic:   "email-received",
		Key:     []byte("1"),
		Payload: []byte(`{"subject":"Test Subject"}`),
	}

	saveOnce := func(t *testing.T) (mailID int, inserted bool) {
		t.Helper()
		err := env.DB.WithTransaction(context.Background(), func(ctx context.Context) error {
			var err error
			mailID, inserted, err = env.DB.CreateMail(ctx, newMail)
			if err != nil {
				return err
			}
			if inserted {
				return env.DB.CreateOutboxEvent(ctx, mailID, event)
			}
			return nil
		})
		assert.Nil(t, err)
		return mailID, inserted
	}

	t.Run("CreateMail and CreateOutboxEvent within a transaction", func(t *testing.T) {
		t.Run("inserts a new mail and outbox event", func(t *testing.T) {
			mailID, inserted := saveOnce(t)
			assert.True(t, inserted)
			assert.NotZero(t, mailID)
		})

		t.Run("is idempotent on a duplicate setting_id+message_id", func(t *testing.T) {
			_, inserted := saveOnce(t)
			assert.False(t, inserted, "a conflicting mail must not create a second row, and must be reported as not-newly-inserted")
		})
	})
}
```

- [ ] **Step 4: Rewrite `outbox_test.go`**

```go
package outbox_test

import (
	"context"
	"errors"
	"planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func seedOutboxEvent(t *testing.T, env *utils.IntegrationTestEnvironment, messageID string) {
	t.Helper()

	newMail := mail.NewMail{
		MessageID:      messageID,
		SettingID:      1,
		OrganizationID: 1,
		Subject:        "Subject",
		Sender:         "sender@example.com",
		Body:           "Body",
		Date:           time.Now(),
	}
	event := mail.OutboxEvent{
		Topic:   "email-received",
		Key:     []byte("1"),
		Payload: []byte(`{"subject":"Subject"}`),
	}

	err := env.DB.WithTransaction(context.Background(), func(ctx context.Context) error {
		mailID, inserted, err := env.DB.CreateMail(ctx, newMail)
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		return env.DB.CreateOutboxEvent(ctx, mailID, event)
	})
	assert.Nil(t, err)
}

func TestOutboxRepository(t *testing.T) {
	env := utils.NewIntegrationTestEnvironment(t)
	seedOutboxEvent(t, env, "outbox-test-1")

	t.Run("FetchBatch", func(t *testing.T) {
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
	})

	t.Run("MarkProcessed", func(t *testing.T) {
		t.Run("marks the record sent and excludes it from future batches", func(t *testing.T) {
			err := env.DB.MarkProcessed(context.Background(), 1)
			assert.Nil(t, err)

			records, err := env.DB.FetchBatch(context.Background(), 10, 0*time.Second)
			assert.Nil(t, err)
			assert.Equal(t, 0, len(records))
		})
	})
}

func TestOutboxRepositoryMarkFailed(t *testing.T) {
	env := utils.NewIntegrationTestEnvironment(t)
	seedOutboxEvent(t, env, "outbox-test-2")

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

- [ ] **Step 5: Verify it compiles**

Run: `go build ./services/email/...`
Expected: exit 0 for everything EXCEPT `internal/infra/email` — `email_service.go` still references the old `FetchedMail`/`mail.FetchedMail` shape and the old `mailServiceInterface.SaveFetchedMails(ctx, mails []mail.FetchedMail)` signature. Expect a compile error there; that's resolved by Task 6.

Run: `go build ./services/email/internal/infra/postgres/... ./services/email/internal/domain/... ./services/email/cmd/outbox-relay/...`
Expected: exit 0 (everything except `internal/infra/email` and its caller in `cmd/main.go` compiles cleanly at this point).

- [ ] **Step 6: Run the integration tests**

Run: `task test:email:integration`
Expected: this will FAIL to build as a whole module command right now, because `go build ./services/email/...` (which `task test:email:integration` runs under the hood via `go test ./services/email/...`) still hits the `internal/infra/email` compile error from Step 5. Instead, run the two affected test packages directly:

Run: `go test ./services/email/internal/test/mail/... ./services/email/internal/test/outbox/... -v -count=1`
Expected: PASS, all subtests in `TestMailRepository`, `TestOutboxRepository`, and `TestOutboxRepositoryMarkFailed` green. (Requires Docker running locally, for testcontainers.)

- [ ] **Step 7: Commit**

```bash
git add services/email/internal/infra/postgres/mail_repository.go services/email/internal/infra/postgres/outbox_repository.go services/email/internal/test/mail/mail_test.go services/email/internal/test/outbox/outbox_test.go
git commit -m "refactor(email): implement CreateMail/CreateOutboxEvent/WithTransaction on postgres.Client"
```

---

### Task 6: Rewire `email_service.go` to use `RawFetchedMail` — drop Kafka/JSON knowledge from the IMAP adapter

**Files:**
- Modify: `services/email/internal/infra/email/email_service.go`

**Interfaces:**
- Consumes: `mail.RawFetchedMail`, `mail.SaveResult` (Task 4).

This is the task where the whole module finally compiles and passes end-to-end again — Tasks 4-5 deliberately left it broken. This rewrite also removes the `planeo/libs/events/contracts` import that Task 1, Step 4 added as a stopgap (that import's only purpose was to keep `services/email` compiling between Task 1 and this task) — the full rewrite below has no `contracts` import at all, since event construction no longer happens in this file.

- [ ] **Step 1: Rewrite the file**

```go
package email

import (
	"context"
	"planeo/libs/logger"
	"planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/domain/setting"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

type cronServiceInterface interface {
	AddJob(ctx context.Context, task func(), fetchInterval time.Duration, tags []string)
	RemoveJobByTag(ctx context.Context, tag string)
}

type imapServiceInterface interface {
	FetchUnseenMails(ctx context.Context, settings IMAPSettings) ([]Email, error)
	MarkSeen(ctx context.Context, settings IMAPSettings, uids []uint32) error
	TestConnection(ctx context.Context, settings IMAPSettings) error
}

type mailServiceInterface interface {
	SaveFetchedMails(ctx context.Context, mails []mail.RawFetchedMail) ([]mail.SaveResult, error)
}

type EmailService struct {
	cronService cronServiceInterface
	imapService imapServiceInterface
	mailService mailServiceInterface
	logger      zerolog.Logger
}

func NewEmailService(cron cronServiceInterface, imap imapServiceInterface, mailService mailServiceInterface) *EmailService {
	return &EmailService{
		cronService: cron,
		imapService: imap,
		mailService: mailService,
		logger:      logger.New("email-service"),
	}
}

func (s *EmailService) StartFetching(ctx context.Context, settings []setting.Setting) error {
	for _, st := range settings {
		task := s.createTask(st)
		s.cronService.AddJob(ctx, task, 10*time.Second, []string{strconv.Itoa(st.ID)})
	}
	return nil
}

func (s *EmailService) StopFetching(ctx context.Context, settingId int) {
	s.cronService.RemoveJobByTag(ctx, strconv.Itoa(settingId))
}

func (s *EmailService) TestConnection(ctx context.Context, st setting.Setting) error {
	return s.imapService.TestConnection(ctx, IMAPSettings{
		Host:     st.Host,
		Port:     st.Port,
		Username: st.Username,
		Password: st.Password,
	})
}

func (s *EmailService) createTask(st setting.Setting) func() {
	return func() {
		start := time.Now()
		emailLogger := s.logger.With().Int("setting_id", st.ID).Logger()
		ctx := logger.WithContext(context.Background(), emailLogger)

		imapSettings := IMAPSettings{
			Host:     st.Host,
			Port:     st.Port,
			Username: st.Username,
			Password: st.Password,
		}

		mails, err := s.imapService.FetchUnseenMails(ctx, imapSettings)
		duration := time.Since(start)

		if err != nil {
			emailLogger.Error().Err(err).Dur("duration_ms", duration).Msg("Error fetching emails")
			return
		}

		emailLogger.Info().
			Int("email_count", len(mails)).
			Dur("duration_ms", duration).
			Msg("Email fetch completed")

		if len(mails) == 0 {
			return
		}

		raws := make([]mail.RawFetchedMail, 0, len(mails))
		for _, m := range mails {
			raws = append(raws, mail.RawFetchedMail{
				MessageID:      m.MessageID,
				SettingID:      st.ID,
				OrganizationID: st.OrganizationID,
				Subject:        m.Subject,
				Sender:         m.From,
				Body:           m.Body,
				Date:           m.Date,
				UID:            m.UID,
			})
		}

		emailLogger.Info().Int("batch_size", len(raws)).Msg("Saving fetched mails to outbox")

		results, err := s.mailService.SaveFetchedMails(ctx, raws)
		if err != nil {
			emailLogger.Error().Err(err).Int("batch_size", len(raws)).Msg("Error saving fetched mails to outbox")
			return
		}

		inserted := 0
		uids := make([]uint32, 0, len(results))
		for _, r := range results {
			uids = append(uids, r.UID)
			if r.Inserted {
				inserted++
			}
		}

		emailLogger.Info().
			Int("results_count", len(results)).
			Int("inserted_count", inserted).
			Msg("SaveFetchedMails completed")

		if err := s.imapService.MarkSeen(ctx, imapSettings, uids); err != nil {
			emailLogger.Error().Err(err).Msg("Error marking emails as seen")
			return
		}

		emailLogger.Info().Int("marked_seen_count", len(uids)).Msg("Marked mails as seen on IMAP")
	}
}
```

This drops the `encoding/json` and `planeo/libs/events` imports entirely, and the old "no mails to save after building outbox batch" warning branch (there's no longer a marshal step at this layer that could skip entries — every fetched `Email` maps 1:1 to a `RawFetchedMail`).

- [ ] **Step 2: Verify the whole module compiles**

Run: `go build ./...`
Expected: exit 0 — this is the point where the deliberate breaks from Tasks 4-5 are fully resolved.

Run: `go vet ./...`
Expected: exit 0.

- [ ] **Step 3: Run email's full test suite**

Run: `task test:email:unit && task test:email:integration`
Expected: both PASS (integration requires Docker).

- [ ] **Step 4: Commit**

```bash
git add services/email/internal/infra/email/email_service.go
git commit -m "refactor(email): drop Kafka/JSON knowledge from email_service.go, use RawFetchedMail"
```

---

### Task 7: Full workspace verification

**Files:** none (verification only)

- [ ] **Step 1: Build and vet the entire module**

Run: `go build ./...`
Expected: exit 0.

Run: `go vet ./...`
Expected: exit 0.

- [ ] **Step 2: Format check**

Run: `gofmt -l .`
Expected: no output. If any file is listed, run `gofmt -w <file>` and re-check. If a listed file is untouched by this plan's tasks (e.g. a pre-existing, unrelated drift inherited from `main`), do not fix it here — that has happened twice before in this branch's history (both times reverted) and is out of scope for this cleanup.

- [ ] **Step 3: Confirm no stray references to removed names**

Run:
```bash
grep -rn "FetchedMail\b" --include="*.go" . | grep -v RawFetchedMail
```
Expected: no output (the old `FetchedMail` bundle type should be fully gone; only `RawFetchedMail` should remain).

Run:
```bash
grep -rn "events\.EmailCreatedPayload\|events\.EmailReceivedTopic" --include="*.go" .
```
Expected: no output (everything should reference `contracts.EmailCreatedPayload`/`contracts.EmailReceivedTopic` now).

- [ ] **Step 4: Run every affected test suite**

Run:
```bash
task test:core:unit
task test:core:integration
task test:email:unit
task test:email:integration
go test ./libs/db/... -v -count=1
go test ./libs/outbox/... -v -short -count=1
```
Expected: all PASS. (`libs/db` and `libs/outbox` aren't covered by any Taskfile target's scope, so they're run explicitly here — the same gap the prior outbox-pattern plan already flagged for `libs/outbox`, now also true for `libs/db`.)

- [ ] **Step 5: Commit (only if steps 2-3 required fixes; otherwise skip — do not create an empty commit)**

```bash
git add -A
git commit -m "chore: cleanup fixes from final verification"
```

---

## Explicitly deferred (do not do in this plan)

Per the approved spec: retrofitting `setting_repository.go` or any `services/core` repository onto `WithTx`/`FromContext`; routing `outbox_repository.go`'s `FetchBatch`/`MarkProcessed`/`MarkFailed` through `db.FromContext` for stylistic consistency; schema-registry serialization logic and where it lives.
