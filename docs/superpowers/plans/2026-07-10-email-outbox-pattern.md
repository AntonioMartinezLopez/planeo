# Transactional Outbox Pattern for services/email — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `services/email`'s fire-and-forget Kafka publish (which today can silently lose an email forever) with a transactional outbox: durably record every fetched mail and its outbox event in one Postgres transaction before marking anything `\Seen` on IMAP, then drain the outbox to Kafka via an independent sidecar with retries.

**Architecture:** New `mails` and `outbox` tables in email's Postgres database. A new `domain/mail` package performs the atomic dual-insert. `imap_service.go` splits fetch and mark-seen into separate UID-based steps. A new reusable `libs/outbox` library implements the generic poll → produce → mark relay engine (a `Store` interface is the per-service extension point; a `Producer` interface decouples it from a concrete Kafka client for testability). `services/email/cmd/outbox-relay` is a new, separate binary wiring this library to email's Postgres outbox table and Kafka, with its own Dockerfile and docker-compose service.

**Tech Stack:** Go, pgx/v5, goose migrations, `github.com/twmb/franz-go/pkg/kgo`, `github.com/emersion/go-imap/v2`, Docker, mockery + testify for unit tests, testcontainers-go for integration tests.

## Global Constraints

- Reference spec: `docs/superpowers/specs/2026-07-10-email-outbox-pattern-design.md`.
- `libs/outbox`'s `Relay` never inspects payload content — topic/key/payload are decided entirely by the writing service at insert time. No pre-produce hook.
- The outbox dequeue (`FetchBatch`) must be a single atomic SQL statement (`UPDATE ... WHERE ... RETURNING`) — never a separate `SELECT ... FOR UPDATE SKIP LOCKED` followed later by a separate mark call. See the spec's "Concurrency safety" section for why the latter provides no real protection.
- Outbox row states: `pending` → `processing` → `sent` (or, after `OUTBOX_MAX_ATTEMPTS` failures, `failed` — a quarantine, not a Kafka-side dead-letter queue). A failed-but-not-exhausted attempt must reset to `pending`, not stay at `processing`.
- No worker-pool concurrency inside the relay in this pass — sequential processing, one record at a time.
- No new Kafka-side dead-letter topic, no message headers/trace-context columns, no outbox retention/cleanup job, no schema-registry integration — all explicitly deferred per the spec.
- Branch: `feature/email-outbox-pattern` (already checked out, based on `feature/migrate-nats-to-kafka`). Do not commit to `main`.
- No new tests for the `services/email/cmd/outbox-relay` binary itself beyond what the library/repository tests already cover — it is wiring only.

---

### Task 1: Add `mails` and `outbox` tables migration

**Files:**
- Create: `services/email/internal/infra/postgres/migrations/20260710120000_add_mails_and_outbox_tables.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE mails (
    id              INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    message_id      TEXT NOT NULL,
    setting_id      INTEGER NOT NULL REFERENCES settings(id),
    organization_id INTEGER NOT NULL,
    subject         TEXT NOT NULL,
    sender          TEXT NOT NULL,
    body            TEXT NOT NULL,
    date            TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (setting_id, message_id)
);

CREATE TABLE outbox (
    id           BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    mail_id      INTEGER NOT NULL REFERENCES mails(id),
    topic        TEXT NOT NULL,
    key          BYTEA,
    payload      BYTEA NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    claimed_at   TIMESTAMPTZ,
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT,
    processed_at TIMESTAMPTZ,
    failed_at    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX outbox_pending_idx ON outbox (id) WHERE status IN ('pending', 'processing');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS outbox_pending_idx;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS mails;
-- +goose StatementEnd
```

- [ ] **Step 2: Run the migration**

Run: `task migrate:email`
Expected: output shows the new migration applied (e.g. `OK   20260710120000_add_mails_and_outbox_tables.sql`).

- [ ] **Step 3: Verify migration status**

Run: `task migrate:email:status`
Expected: the new migration listed with a non-empty "Applied At" timestamp.

- [ ] **Step 4: Commit**

```bash
git add services/email/internal/infra/postgres/migrations/20260710120000_add_mails_and_outbox_tables.sql
git commit -m "feat(email): add mails and outbox tables migration"
```

---

### Task 2: `domain/mail` package — model and ports

**Files:**
- Create: `services/email/internal/domain/mail/model.go`
- Create: `services/email/internal/domain/mail/ports.go`

**Interfaces:**
- Produces: `mail.NewMail`, `mail.OutboxEvent`, `mail.FetchedMail`, `mail.SaveResult`, `mail.Repository`, `mail.Service` — all consumed by Tasks 3, 4, 5, 6.

Note: this domain deliberately has no `errors.go`, unlike `domain/setting`. Unlike settings (which have a "not found" concept surfaced to a REST handler), this domain has no REST exposure and no domain-specific error conditions — repository errors bubble up as-is via `postgres.NewDatabaseError` (already defined in `services/email/internal/infra/postgres/errors.go`). Adding an unused `errors.go` here would be dead code.

- [ ] **Step 1: Write `model.go`**

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

// FetchedMail pairs a mail fetched from IMAP with the outbox event it
// should produce, and the IMAP UID needed to mark it seen afterward.
type FetchedMail struct {
	Mail  NewMail
	Event OutboxEvent
	UID   uint32
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

- [ ] **Step 2: Write `ports.go`**

```go
package mail

import "context"

type Repository interface {
	// SaveFetchedMails durably records each fetched mail and its outbox
	// event in one Postgres transaction. A mail that already exists
	// (duplicate setting_id+message_id) is a no-op for both tables; it is
	// still reported in the result (Inserted: false) so the caller can
	// safely mark it seen on IMAP.
	SaveFetchedMails(ctx context.Context, mails []FetchedMail) ([]SaveResult, error)
}

type Service interface {
	SaveFetchedMails(ctx context.Context, mails []FetchedMail) ([]SaveResult, error)
}
```

- [ ] **Step 3: Verify the package compiles standalone**

Run: `go build ./services/email/internal/domain/mail/...`
Expected: exit 0 (no output — there's no implementation yet, only types and interfaces, which is a valid Go package on its own).

- [ ] **Step 4: Commit**

```bash
git add services/email/internal/domain/mail/model.go services/email/internal/domain/mail/ports.go
git commit -m "feat(email): add domain/mail model and ports"
```

---

### Task 3: `domain/mail` service implementation + unit tests

**Files:**
- Create: `services/email/.mockery.yml`
- Create: `services/email/internal/domain/mail/service.go`
- Create: `services/email/internal/domain/mail/service_test.go`
- Create (generated by mockery, not hand-written): `services/email/internal/domain/mail/mocks/repository_mock.go`

**Interfaces:**
- Consumes: `mail.NewMail`, `mail.OutboxEvent`, `mail.FetchedMail`, `mail.SaveResult`, `mail.Repository` from Task 2.
- Produces: `mail.NewService(repository Repository) Service`, consumed by Task 6.

- [ ] **Step 1: Create the mockery config for the email service**

`services/email` has no mockery config today (unlike `services/core/.mockery.yml`). Create `services/email/.mockery.yml`:

```yaml
all: true
dir: '{{.InterfaceDir}}/mocks'
structname: Mock{{.InterfaceName}}
filename: '{{.InterfaceName | snakecase}}_mock.go'
pkgname: mocks
template: testify
template-data:
  unroll-variadic: true
packages:
  planeo/services/email/internal/domain/mail:
    interfaces:
      Repository: {}
```

- [ ] **Step 2: Generate the mock**

Run: `cd services/email && mockery && cd -`
Expected: `services/email/internal/domain/mail/mocks/repository_mock.go` is created, containing a `MockRepository` type with an `EXPECT()` builder (matching the pattern already used by `services/core`'s generated mocks).

- [ ] **Step 3: Write `service.go`**

```go
package mail

import "context"

type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{repository: repository}
}

func (s *service) SaveFetchedMails(ctx context.Context, mails []FetchedMail) ([]SaveResult, error) {
	if len(mails) == 0 {
		return nil, nil
	}
	return s.repository.SaveFetchedMails(ctx, mails)
}
```

- [ ] **Step 4: Write `service_test.go`**

```go
package mail_test

import (
	"context"
	. "planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/domain/mail/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMailService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	fetched := []FetchedMail{
		{
			Mail: NewMail{
				MessageID:      "abc123",
				SettingID:      1,
				OrganizationID: 1,
				Subject:        "Test",
				Sender:         "sender@example.com",
				Body:           "body",
				Date:           time.Now(),
			},
			Event: OutboxEvent{
				Topic:   "email-received",
				Key:     []byte("1"),
				Payload: []byte(`{"subject":"Test"}`),
			},
			UID: 42,
		},
	}

	t.Run("SaveFetchedMails", func(t *testing.T) {
		t.Run("returns results when mails are saved successfully", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().SaveFetchedMails(context.Background(), fetched).Return([]SaveResult{{UID: 42, Inserted: true}}, nil)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), fetched)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.True(t, results[0].Inserted)
		})

		t.Run("returns error when saving fails", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().SaveFetchedMails(context.Background(), fetched).Return(nil, assert.AnError)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), fetched)
			assert.Error(t, err)
			assert.Nil(t, results)
		})

		t.Run("returns nil without calling repository when mails is empty", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), []FetchedMail{})
			assert.Nil(t, err)
			assert.Nil(t, results)
		})
	})
}
```

- [ ] **Step 5: Run the test**

Run: `go test ./services/email/internal/domain/mail/... -v -short -count=1`
Expected: PASS, all 3 subtests green.

- [ ] **Step 6: Commit**

```bash
git add services/email/.mockery.yml services/email/internal/domain/mail/service.go services/email/internal/domain/mail/service_test.go services/email/internal/domain/mail/mocks/
git commit -m "feat(email): implement domain/mail service with unit tests"
```

---

### Task 4: Postgres repository for mail+outbox dual insert + testcontainer harness + integration test

**Files:**
- Create: `services/email/internal/infra/postgres/mail_repository.go`
- Create: `services/email/internal/test/utils/testcontainer_postgres.go`
- Create: `services/email/internal/test/utils/setup.go`
- Create: `services/email/internal/test/mail/mail_test.go`

**Interfaces:**
- Consumes: `mail.FetchedMail`, `mail.SaveResult`, `mail.Repository` from Task 2. `postgres.Client`, `postgres.NewClient`, `postgres.NewDatabaseError` (all already exist).
- Produces: `(*postgres.Client).SaveFetchedMails(...)`, satisfying `mail.Repository` structurally, consumed by Task 6's `cmd/main.go` wiring. `utils.NewIntegrationTestEnvironment(t)`, consumed by Task 8.

Note: `services/email/.env.test.template` already exists (with `DB_NAME=mail`) and needs no changes for this task — it still has a stale `NATS_URL` line left over from an earlier migration, which Task 6 removes as part of a related cleanup; leave it as-is here.

- [ ] **Step 1: Write `mail_repository.go`**

```go
package postgres

import (
	"context"
	"errors"
	"planeo/services/email/internal/domain/mail"

	"github.com/jackc/pgx/v5"
)

func (c *Client) SaveFetchedMails(ctx context.Context, mails []mail.FetchedMail) ([]mail.SaveResult, error) {
	results := make([]mail.SaveResult, 0, len(mails))

	tx, err := c.db.Begin(ctx)
	if err != nil {
		return nil, NewDatabaseError("error starting transaction", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, fetched := range mails {
		insertMailQuery := `
			INSERT INTO mails (message_id, setting_id, organization_id, subject, sender, body, date)
			VALUES (@messageId, @settingId, @organizationId, @subject, @sender, @body, @date)
			ON CONFLICT (setting_id, message_id) DO NOTHING
			RETURNING id`
		args := pgx.NamedArgs{
			"messageId":      fetched.Mail.MessageID,
			"settingId":      fetched.Mail.SettingID,
			"organizationId": fetched.Mail.OrganizationID,
			"subject":        fetched.Mail.Subject,
			"sender":         fetched.Mail.Sender,
			"body":           fetched.Mail.Body,
			"date":           fetched.Mail.Date,
		}

		row, err := tx.Query(ctx, insertMailQuery, args)
		if err != nil {
			return nil, NewDatabaseError("error inserting mail", err)
		}

		mailID, err := pgx.CollectExactlyOneRow(row, pgx.RowTo[int])
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				results = append(results, mail.SaveResult{UID: fetched.UID, Inserted: false})
				continue
			}
			return nil, NewDatabaseError("error collecting inserted mail id", err)
		}

		insertOutboxQuery := `
			INSERT INTO outbox (mail_id, topic, key, payload)
			VALUES (@mailId, @topic, @key, @payload)`
		outboxArgs := pgx.NamedArgs{
			"mailId":  mailID,
			"topic":   fetched.Event.Topic,
			"key":     fetched.Event.Key,
			"payload": fetched.Event.Payload,
		}

		if _, err := tx.Exec(ctx, insertOutboxQuery, outboxArgs); err != nil {
			return nil, NewDatabaseError("error inserting outbox event", err)
		}

		results = append(results, mail.SaveResult{UID: fetched.UID, Inserted: true})
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, NewDatabaseError("error committing transaction", err)
	}

	return results, nil
}
```

- [ ] **Step 2: Write the testcontainer helper**

`services/email/internal/test/utils/testcontainer_postgres.go`:

```go
package utils

import (
	"context"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func StartPostgresContainer(ctx context.Context) (*postgres.PostgresContainer, error) {
	return postgres.Run(ctx,
		"postgres:alpine3.20",
		postgres.WithDatabase("mail"),
		postgres.WithUsername("planeo"),
		postgres.WithPassword("planeo"),
		testcontainers.WithWaitStrategyAndDeadline(5*time.Minute,
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
}
```

- [ ] **Step 3: Write the integration test environment setup**

`services/email/internal/test/utils/setup.go`:

```go
package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"planeo/services/email/internal/config"
	"planeo/services/email/internal/infra/postgres"
	"testing"

	postgresContainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type IntegrationTestEnvironment struct {
	PostgresContainer *postgresContainer.PostgresContainer
	Configuration     *config.ApplicationConfiguration
	DB                *postgres.Client
}

func NewIntegrationTestEnvironment(t *testing.T) *IntegrationTestEnvironment {
	postgresCont, err := StartPostgresContainer(context.Background())
	if err != nil {
		panic(err)
	}

	postgresPort, err := postgresCont.MappedPort(context.Background(), "5432")
	if err != nil {
		t.Error(err)
	}

	cfg := config.LoadConfig(context.Background(), "../../../.env.test.template")
	cfg.DbPort = postgresPort.Port()

	env := &IntegrationTestEnvironment{
		PostgresContainer: postgresCont,
		Configuration:     cfg,
	}

	if err := env.MigrateDatabase(false); err != nil {
		t.Error(err.Error())
		panic(err)
	}

	db := postgres.NewClient(context.Background(), cfg.DatabaseConfig())
	env.DB = db

	t.Cleanup(func() {
		ctx := context.Background()
		if err := env.PostgresContainer.Terminate(ctx); err != nil {
			panic(err)
		}
		env.DB.Close()
	})

	return env
}

func (env *IntegrationTestEnvironment) MigrateDatabase(tearDown bool) error {
	operation := "up"
	if tearDown {
		operation = "down"
	}

	migrationsDir, _ := filepath.Abs(filepath.Join("..", "..", "..", "internal", "infra", "postgres", "migrations"))
	cmd := exec.Command("goose", "-dir", migrationsDir, "postgres", fmt.Sprintf("postgres://planeo:planeo@127.0.0.1:%s/mail?sslmode=disable",
		env.Configuration.DbPort), operation)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}
	return nil
}
```

Note: `config.LoadConfig(ctx, "../../../.env.test.template")` is relative to this file's location (`internal/test/utils/`), three levels up to the service root — matching `services/core`'s equivalent path depth exactly. At this point in the plan (before Task 6 removes it), `config.ApplicationConfiguration` still has a `KafkaBrokers` field read from `KAFKA_BROKERS`, which `services/email/.env.test.template` already provides via its (currently misnamed) `NATS_URL` line — this will read fine since `.env.test.template`'s exact env var name doesn't matter here as long as `KAFKA_BROKERS` isn't required yet; double check: `config.LoadConfig` calls `readEnvVariable(ctx, "KAFKA_BROKERS")`, which will `Fatal()` if unset. Since `services/email/.env.test.template` currently has `NATS_URL=...` and NOT `KAFKA_BROKERS=...`, this WILL fail today. Add one line to `services/email/.env.test.template` in this step to unblock it:

- [ ] **Step 3b: Add the missing `KAFKA_BROKERS` line to the test env template (temporary — Task 6 removes it again)**

In `services/email/.env.test.template`, add a new line anywhere after the `PORT` line:
```
KAFKA_BROKERS=localhost:9092
```

- [ ] **Step 4: Write the integration test**

`services/email/internal/test/mail/mail_test.go`:

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

	t.Run("SaveFetchedMails", func(t *testing.T) {
		t.Run("inserts a new mail and outbox event", func(t *testing.T) {
			fetched := []mail.FetchedMail{{Mail: newMail, Event: event, UID: 1}}

			results, err := env.DB.SaveFetchedMails(context.Background(), fetched)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.True(t, results[0].Inserted)
			assert.Equal(t, uint32(1), results[0].UID)
		})

		t.Run("is idempotent on a duplicate setting_id+message_id", func(t *testing.T) {
			fetched := []mail.FetchedMail{{Mail: newMail, Event: event, UID: 2}}

			results, err := env.DB.SaveFetchedMails(context.Background(), fetched)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.False(t, results[0].Inserted, "a conflicting mail must not create a second row, and must be reported as not-newly-inserted")
			assert.Equal(t, uint32(2), results[0].UID, "the UID from THIS fetch must still be returned so the caller can mark it seen")
		})
	})
}
```

This relies on `setting_id = 1` existing, which it does: the very first email service migration (`20241101135140_initialize_database.sql`) seeds a default settings row with `id = 1` via `GENERATED ALWAYS AS IDENTITY`, and `MigrateDatabase(false)` runs every migration including that seed.

- [ ] **Step 5: Run the integration test**

Run: `task test:email:integration`
Expected: PASS, both subtests green. (Requires Docker running locally, for testcontainers.)

- [ ] **Step 6: Commit**

```bash
git add services/email/internal/infra/postgres/mail_repository.go services/email/internal/test/utils/testcontainer_postgres.go services/email/internal/test/utils/setup.go services/email/internal/test/mail/mail_test.go services/email/.env.test.template
git commit -m "feat(email): add mail+outbox repository with transactional dual insert and integration test"
```

---

### Task 5: Split `imap_service.go` to UID-based fetch/mark-seen

**Files:**
- Modify: `services/email/internal/infra/email/imap_service.go`

**Interfaces:**
- Produces: `(*IMAPService).FetchUnseenMails(ctx, settings) ([]Email, error)`, `(*IMAPService).MarkSeen(ctx, settings, uids []uint32) error`, both consumed by Task 6. `Email.UID uint32` (replaces the removed `Email.SeqNum uint32` field).

- [ ] **Step 1: Replace the `Email` struct's `SeqNum` field with `UID`**

Change lines 23-30 from:
```go
type Email struct {
	Subject   string
	Body      string
	From      string
	Date      time.Time
	MessageID string
	SeqNum    uint32
}
```
to:
```go
type Email struct {
	Subject   string
	Body      string
	From      string
	Date      time.Time
	MessageID string
	UID       uint32
}
```

- [ ] **Step 2: Replace `FetchAllUnseenMails` with `FetchUnseenMails` (UID-based, no seen-marking)**

Replace the whole `FetchAllUnseenMails` function (lines 47-101) with:

```go
func (s *IMAPService) FetchUnseenMails(ctx context.Context, settings IMAPSettings) ([]Email, error) {
	l := logger.FromContext(ctx)
	c, err := s.login(ctx, settings)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	sc := imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}
	e, err := c.UIDSearch(&sc, nil).Wait()
	if err != nil {
		l.Error().Err(err).Msg("failed to search for unseen messages")
		return nil, err
	}

	emails := []Email{}
	uids := e.AllUIDs()
	if len(uids) == 0 {
		return emails, nil
	}

	uidSet := imap.UIDSet{}
	uidSet.AddNum(uids...)
	fetchOptions := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{{}},
		UID:         true,
	}

	fetchCmd := c.Fetch(uidSet, fetchOptions)
	defer fetchCmd.Close()

	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}
		email, err := s.extractMailData(ctx, msg)
		if err != nil {
			l.Error().Err(err).Msg("failed to extract mail data")
			return nil, err
		}
		emails = append(emails, email)
	}

	if err := fetchCmd.Close(); err != nil {
		l.Error().Err(err).Msg("failed to close FETCH command")
		return nil, err
	}

	return emails, nil
}

func (s *IMAPService) MarkSeen(ctx context.Context, settings IMAPSettings, uids []uint32) error {
	if len(uids) == 0 {
		return nil
	}

	l := logger.FromContext(ctx)
	c, err := s.login(ctx, settings)
	if err != nil {
		return err
	}
	defer c.Logout()

	uidSet := imap.UIDSet{}
	for _, u := range uids {
		uidSet.AddNum(imap.UID(u))
	}

	storeFlags := imap.StoreFlags{Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagSeen}, Silent: true}
	if err := c.Store(uidSet, &storeFlags, nil).Close(); err != nil {
		l.Error().Err(err).Msg("failed to mark fetched mails as seen")
		return err
	}

	return nil
}
```

This is a required behavior change, not a rename: the two steps now run in separate IMAP sessions (each opens its own connection via `s.login`/`defer c.Logout()`), and message *sequence numbers* are only stable within a single session — they get renumbered if any message is expunged between sessions. UIDs are stable within a mailbox's `UIDVALIDITY` epoch, so marking by UID in a later session is safe where marking by stale sequence number would not be. `go-imap/v2`'s `Client.Fetch`/`Client.Store` both accept the same `imap.NumSet` interface satisfied by either `imap.SeqSet` or `imap.UIDSet` (confirmed in `imapclient/fetch.go:22` and `imapclient/store.go:15` — passing a `UIDSet` is what makes these issue `UID FETCH`/`UID STORE` on the wire).

- [ ] **Step 3: Update `extractMailData` to capture the UID alongside the body section**

Replace the function body (lines 142-180) with:

```go
func (s *IMAPService) extractMailData(ctx context.Context, msg *imapclient.FetchMessageData) (Email, error) {
	l := logger.FromContext(ctx)
	var bodySection imapclient.FetchItemDataBodySection
	var uid imap.UID
	hasBodySection := false

	for {
		item := msg.Next()
		if item == nil {
			break
		}
		switch data := item.(type) {
		case imapclient.FetchItemDataBodySection:
			bodySection = data
			hasBodySection = true
		case imapclient.FetchItemDataUID:
			uid = data.UID
		}
	}

	if !hasBodySection {
		return Email{}, fmt.Errorf("FETCH command did not return body section")
	}

	mr, err := mail.CreateReader(bodySection.Literal)
	if err != nil {
		l.Error().Err(err).Msg("failed to create mail reader")
		return Email{}, err
	}

	email, err := s.extractHeaderFields(ctx, mr.Header, Email{})
	if err != nil {
		l.Error().Err(err).Msg("failed to extract header fields")
		return Email{}, err
	}

	email, err = s.extractEmailBody(ctx, mr, email)
	if err != nil {
		l.Error().Err(err).Msg("failed to extract email body")
		return Email{}, err
	}

	email.UID = uint32(uid)
	return email, nil
}
```

This must not `break` on finding the body section (unlike the previous version), because the library sends `UID` as the first data item for a UID `FETCH` — breaking early on the first matched item type would skip the UID before it's read. Looping until `msg.Next()` returns nil, using a type switch, captures both.

- [ ] **Step 4: Verify the package compiles**

Run: `go build ./services/email/internal/infra/email/...`
Expected: exit 0.

Note: this will only compile once callers of the old `FetchAllUnseenMails` are also updated — that happens in Task 6. If this task is reviewed standalone before Task 6 lands, expect a compile error in `email_service.go` referencing the removed function name; that's expected and resolved by the next task, not a defect in this one.

- [ ] **Step 5: Commit**

```bash
git add services/email/internal/infra/email/imap_service.go
git commit -m "feat(email): switch IMAP fetch/mark-seen to UID-based, separate sessions"
```

---

### Task 6: Rewire `email_service.go` + `cmd/main.go` + `config.go` to use `domain/mail` instead of direct Kafka publish

**Files:**
- Modify: `services/email/internal/infra/email/email_service.go`
- Modify: `services/email/cmd/main.go`
- Modify: `services/email/internal/config/config.go`
- Modify: `libs/events/email_received.go`
- Modify: `services/email/.env.template`
- Modify: `services/email/.env.test.template`

**Interfaces:**
- Consumes: `mail.NewService`, `mail.FetchedMail`, `mail.OutboxEvent`, `mail.NewMail`, `mail.SaveResult` (Tasks 2-3); `(*IMAPService).FetchUnseenMails`, `(*IMAPService).MarkSeen` (Task 5); `(*postgres.Client).SaveFetchedMails` (Task 4).
- Produces: `events.EmailReceivedTopic` (exported constant, replacing the unexported `topic` var), used by this task's own `email_service.go` change.

- [ ] **Step 1: Export the topic constant in `libs/events/email_received.go`**

Replace the whole file's content with:

```go
package events

import (
	"context"
	"encoding/json"
	"time"
)

type EmailCreatedPayload struct {
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	From           string    `json:"from"`
	Date           time.Time `json:"date"`
	MessageID      string    `json:"messageId"`
	OrganizationId int       `json:"organizationId"`
}

// EmailReceivedTopic is exported so services that write directly to this
// topic (e.g. services/email's transactional outbox) share a single source
// of truth with the publish/subscribe helpers below.
const EmailReceivedTopic = "email-received"

var subscriptionName = "email-receiver"

func (es *EventService) PublishEmailReceived(ctx context.Context, payload EmailCreatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return es.Publish(ctx, EmailReceivedTopic, data)
}

func (es *EventService) SubscribeEmailReceived(ctx context.Context, handler func(EmailCreatedPayload) error) error {
	return es.Subscribe(ctx, subscriptionName, EmailReceivedTopic, func(data []byte) error {
		var payload EmailCreatedPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			return err
		}

		return handler(payload)
	})
}
```

- [ ] **Step 2: Rewrite `email_service.go`**

```go
package email

import (
	"context"
	"encoding/json"
	"planeo/libs/events"
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
	SaveFetchedMails(ctx context.Context, mails []mail.FetchedMail) ([]mail.SaveResult, error)
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

		fetched := make([]mail.FetchedMail, 0, len(mails))
		for _, m := range mails {
			payload, err := json.Marshal(events.EmailCreatedPayload{
				Subject:        m.Subject,
				Body:           m.Body,
				From:           m.From,
				Date:           m.Date,
				MessageID:      m.MessageID,
				OrganizationId: st.OrganizationID,
			})
			if err != nil {
				emailLogger.Error().Err(err).Str("message_id", m.MessageID).Msg("Error marshaling email event payload")
				continue
			}

			fetched = append(fetched, mail.FetchedMail{
				Mail: mail.NewMail{
					MessageID:      m.MessageID,
					SettingID:      st.ID,
					OrganizationID: st.OrganizationID,
					Subject:        m.Subject,
					Sender:         m.From,
					Body:           m.Body,
					Date:           m.Date,
				},
				Event: mail.OutboxEvent{
					Topic:   events.EmailReceivedTopic,
					Key:     []byte(strconv.Itoa(st.OrganizationID)),
					Payload: payload,
				},
				UID: m.UID,
			})
		}

		if len(fetched) == 0 {
			return
		}

		results, err := s.mailService.SaveFetchedMails(ctx, fetched)
		if err != nil {
			emailLogger.Error().Err(err).Msg("Error saving fetched mails to outbox")
			return
		}

		uids := make([]uint32, 0, len(results))
		for _, r := range results {
			uids = append(uids, r.UID)
		}

		if err := s.imapService.MarkSeen(ctx, imapSettings, uids); err != nil {
			emailLogger.Error().Err(err).Msg("Error marking emails as seen")
		}
	}
}
```

Note the ordering: mark-seen is only called after `SaveFetchedMails` returns successfully, and covers every UID in the batch (whether newly inserted or a safe duplicate) — this is the crash-safety property the whole spec is built around.

- [ ] **Step 3: Remove `KafkaBrokers` from `config.go`**

Replace the whole file's content with:

```go
package config

import (
	"context"
	"fmt"
	"os"
	"planeo/libs/logger"

	"github.com/joho/godotenv"
)

type ApplicationConfiguration struct {
	Host            string
	Port            string
	DbHost          string
	DbPort          string
	DbUser          string
	DbPassword      string
	DbName          string
	KcBaseUrl       string
	KcIssuer        string
	KcOauthClientID string
}

func (c *ApplicationConfiguration) ServerConfig() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

func (c *ApplicationConfiguration) DatabaseConfig() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DbUser, c.DbPassword, c.DbHost, c.DbPort, c.DbName)
}

func (c *ApplicationConfiguration) OauthIssuerUrl() string {
	return fmt.Sprintf("%s/realms/%s", c.KcBaseUrl, c.KcIssuer)
}

func LoadConfig(ctx context.Context, filenames ...string) *ApplicationConfiguration {
	err := godotenv.Load(filenames...)
	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Warn().Err(err).Msg("Error loading .env file")
	}

	return &ApplicationConfiguration{
		Host:            readEnvVariable(ctx, "HOST"),
		Port:            readEnvVariable(ctx, "PORT"),
		DbHost:          readEnvVariable(ctx, "DB_HOST"),
		DbPort:          readEnvVariable(ctx, "DB_PORT"),
		DbUser:          readEnvVariable(ctx, "DB_USER"),
		DbPassword:      readEnvVariable(ctx, "DB_PASSWORD"),
		DbName:          readEnvVariable(ctx, "DB_NAME"),
		KcBaseUrl:       readEnvVariable(ctx, "KC_BASE_URL"),
		KcIssuer:        readEnvVariable(ctx, "KC_ISSUER_REALM"),
		KcOauthClientID: readEnvVariable(ctx, "KC_OAUTH_CLIENT_ID"),
	}
}

func readEnvVariable(ctx context.Context, name string) string {
	v, ok := os.LookupEnv(name)
	if !ok {
		logger := logger.FromContext(ctx)
		logger.Fatal().Msgf("Missing env variable '%s'. Aborting...\n", name)
	}
	return v
}
```

`services/email`'s main service no longer talks to Kafka at all — only the new `outbox-relay` sidecar (Task 9) does, with its own separate, minimal config.

- [ ] **Step 4: Rewrite `cmd/main.go`**

```go
package main

import (
	"context"
	"net/http"
	"planeo/libs/logger"
	"planeo/services/email/internal/config"
	"planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/domain/setting"
	emailInfra "planeo/services/email/internal/infra/email"
	"planeo/services/email/internal/infra/postgres"
	"planeo/services/email/internal/infra/rest"
	"time"
)

func main() {
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	log := logger.New("main")
	ctx := logger.WithContext(context.Background(), log)

	log.Info().Msg("Loading environment variables")
	cfg := config.LoadConfig(ctx)

	db := postgres.NewClient(ctx, cfg.DatabaseConfig())
	defer db.Close()

	mailService := mail.NewService(db)

	cronService := emailInfra.NewCronService()
	cronService.Start()

	imapService := emailInfra.NewIMAPService()
	emailService := emailInfra.NewEmailService(cronService, imapService, mailService)

	settingService, err := setting.NewService(db, emailService)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize setting service")
	}

	server := rest.New(cfg, rest.Services{SettingService: settingService})

	httpServer := http.Server{
		Addr:              cfg.ServerConfig(),
		Handler:           server.Router,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	log.Info().Msgf("Server Running at %s", cfg.ServerConfig())
	log.Fatal().Msgf("%v", httpServer.ListenAndServe())
}
```

`db` (`*postgres.Client`) satisfies `mail.Repository` structurally via the `SaveFetchedMails` method added in Task 4 — no explicit interface assertion needed, matching how `db` already satisfies `setting.Repository`.

- [ ] **Step 5: Remove the now-unused `KAFKA_BROKERS` line from `services/email/.env.template`**

Remove this line (added during the earlier NATS-to-Kafka migration, now unused since the main email service no longer connects to Kafka directly):
```
KAFKA_BROKERS=localhost:9092
```

- [ ] **Step 6: Fix `services/email/.env.test.template`**

Remove the line added in Task 4 Step 3b (`KAFKA_BROKERS=localhost:9092` — no longer needed since `config.go` no longer reads it) and the stale, never-updated line left over from before the NATS-to-Kafka migration:
```
NATS_URL=nats://localhost:4222
```
Both lines should be gone; the file should otherwise be unchanged (`HOST`, `PORT`, `CONTAINER_ENV`, `DB_*`, `KC_*` remain).

- [ ] **Step 7: Verify it compiles**

Run: `go build ./services/email/... ./libs/events/...`
Expected: exit 0.

- [ ] **Step 8: Run email unit tests**

Run: `task test:email:unit`
Expected: PASS (includes Task 3's `domain/mail` unit tests).

- [ ] **Step 9: Commit**

```bash
git add libs/events/email_received.go services/email/internal/infra/email/email_service.go services/email/cmd/main.go services/email/internal/config/config.go services/email/.env.template services/email/.env.test.template
git commit -m "feat(email): write to outbox instead of publishing directly to Kafka"
```

---

### Task 7: `libs/outbox` reusable relay library + unit tests

**Files:**
- Create: `libs/outbox/store.go`
- Create: `libs/outbox/producer.go`
- Create: `libs/outbox/relay.go`
- Create: `libs/outbox/relay_test.go`

**Interfaces:**
- Produces: `outbox.Record`, `outbox.Store`, `outbox.Producer`, `outbox.NewProducer(brokers []string) (Producer, *kgo.Client, error)`, `outbox.Relay`, `outbox.NewRelay(store Store, producer Producer, opts ...Option) *Relay`, `outbox.WithPollInterval/WithBatchSize/WithMaxAttempts/WithClaimTTL`, `(*Relay).Run(ctx) error` — all consumed by Task 8 (Store implementation) and Task 9 (sidecar wiring).

- [ ] **Step 1: Write `store.go`**

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

- [ ] **Step 2: Write `producer.go`**

```go
package outbox

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer sends a single record to Kafka. Implemented by kafkaProducer in
// production; test code can supply a fake so Relay's poll/mark logic is
// unit-testable without a live broker.
type Producer interface {
	ProduceSync(ctx context.Context, topic string, key, value []byte) error
}

type kafkaProducer struct {
	client *kgo.Client
}

func (p *kafkaProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	result := p.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: key, Value: value})
	return result.FirstErr()
}

// NewProducer creates a Producer backed by a new franz-go client connected
// to brokers, with broker-side auto-topic-creation enabled (without this,
// franz-go disables it client-side by default regardless of the broker's
// own auto.create.topics.enable setting). The returned *kgo.Client is also
// returned so the caller can Close it on shutdown.
func NewProducer(brokers []string) (Producer, *kgo.Client, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, nil, err
	}
	return &kafkaProducer{client: client}, client, nil
}
```

- [ ] **Step 3: Write `relay.go`**

```go
package outbox

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

type Relay struct {
	store        Store
	producer     Producer
	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	claimTTL     time.Duration
}

type Option func(*Relay)

func WithPollInterval(d time.Duration) Option {
	return func(r *Relay) { r.pollInterval = d }
}

func WithBatchSize(n int) Option {
	return func(r *Relay) { r.batchSize = n }
}

func WithMaxAttempts(n int) Option {
	return func(r *Relay) { r.maxAttempts = n }
}

func WithClaimTTL(d time.Duration) Option {
	return func(r *Relay) { r.claimTTL = d }
}

func NewRelay(store Store, producer Producer, opts ...Option) *Relay {
	r := &Relay{
		store:        store,
		producer:     producer,
		pollInterval: DefaultPollInterval,
		batchSize:    DefaultBatchSize,
		maxAttempts:  DefaultMaxAttempts,
		claimTTL:     DefaultClaimTTL,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run polls the store and produces each claimed record to Kafka,
// sequentially, until ctx is cancelled. It blocks the calling goroutine.
func (r *Relay) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.pollOnce(ctx, log); err != nil {
				log.Error().Err(err).Msg("outbox relay poll failed")
			}
		}
	}
}

func (r *Relay) pollOnce(ctx context.Context, log zerolog.Logger) error {
	records, err := r.store.FetchBatch(ctx, r.batchSize, r.claimTTL)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if err := r.producer.ProduceSync(ctx, rec.Topic, rec.Key, rec.Payload); err != nil {
			log.Error().Err(err).Int64("outbox_id", rec.ID).Msg("failed to produce outbox record")
			if markErr := r.store.MarkFailed(ctx, rec.ID, err, r.maxAttempts); markErr != nil {
				log.Error().Err(markErr).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record as failed")
			}
			continue
		}

		if err := r.store.MarkProcessed(ctx, rec.ID); err != nil {
			log.Error().Err(err).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record as processed")
		}
	}

	return nil
}
```

- [ ] **Step 4: Write `relay_test.go`**

```go
package outbox_test

import (
	"context"
	"errors"
	"planeo/libs/outbox"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeStore struct {
	mu         sync.Mutex
	records    []outbox.Record
	processed  []int64
	failed     map[int64]int
	maxReached []int64
}

func newFakeStore(records []outbox.Record) *fakeStore {
	return &fakeStore{records: records, failed: map[int64]int{}}
}

func (f *fakeStore) FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]outbox.Record, error) {
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

func (f *fakeStore) MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error {
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
	sent      []outbox.Record
	failTopic string
}

func (f *fakeProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if topic == f.failTopic {
		return errors.New("simulated produce failure")
	}
	f.sent = append(f.sent, outbox.Record{Topic: topic, Key: key, Payload: value})
	return nil
}

func TestRelay(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("produces a fetched record and marks it processed", func(t *testing.T) {
		store := newFakeStore([]outbox.Record{{ID: 1, Topic: "t", Key: []byte("k"), Payload: []byte("v")}})
		producer := &fakeProducer{}
		relay := outbox.NewRelay(store, producer, outbox.WithPollInterval(10*time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = relay.Run(ctx)

		assert.Equal(t, 1, len(producer.sent))
		assert.Equal(t, []int64{1}, store.processed)
	})

	t.Run("marks a record failed and quarantines it after max attempts", func(t *testing.T) {
		// Simulates 3 poll cycles' worth of fetches for the same still-
		// unprocessed row by queuing 3 copies of it up front — a
		// simplification of a real Store, which would keep returning the
		// same unprocessed row across polls until it succeeds or is
		// quarantined.
		record := outbox.Record{ID: 2, Topic: "broken-topic", Key: nil, Payload: []byte("v")}
		store := newFakeStore([]outbox.Record{record, record, record})
		producer := &fakeProducer{failTopic: "broken-topic"}
		relay := outbox.NewRelay(store, producer, outbox.WithPollInterval(10*time.Millisecond), outbox.WithMaxAttempts(2))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = relay.Run(ctx)

		assert.Equal(t, 0, len(producer.sent))
		assert.GreaterOrEqual(t, store.failed[2], 2)
		assert.Contains(t, store.maxReached, int64(2))
	})
}
```

- [ ] **Step 5: Run the test**

Run: `go test ./libs/outbox/... -v -short -count=1`
Expected: PASS, both subtests green.

- [ ] **Step 6: Commit**

```bash
git add libs/outbox/
git commit -m "feat(outbox): add reusable Store/Producer/Relay engine with unit tests"
```

---

### Task 8: `services/email` outbox Postgres `Store` implementation + integration test

**Files:**
- Create: `services/email/internal/infra/postgres/outbox_repository.go`
- Create: `services/email/internal/test/outbox/outbox_test.go`

**Interfaces:**
- Consumes: `outbox.Record`, `outbox.Store` from Task 7. `postgres.Client`, `postgres.NewDatabaseError`. `utils.NewIntegrationTestEnvironment` from Task 4.
- Produces: `(*postgres.Client).FetchBatch/MarkProcessed/MarkFailed`, satisfying `outbox.Store` structurally, consumed by Task 9's sidecar wiring.

- [ ] **Step 1: Write `outbox_repository.go`**

```go
package postgres

import (
	"context"
	"planeo/libs/outbox"
	"time"

	"github.com/jackc/pgx/v5"
)

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

func (c *Client) MarkProcessed(ctx context.Context, id int64) error {
	args := pgx.NamedArgs{"id": id}
	_, err := c.db.Exec(ctx, `UPDATE outbox SET status = 'sent', processed_at = NOW() WHERE id = @id`, args)
	if err != nil {
		return NewDatabaseError("error marking outbox record processed", err)
	}
	return nil
}

func (c *Client) MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error {
	args := pgx.NamedArgs{"id": id, "lastError": sendErr.Error(), "maxAttempts": maxAttempts}
	query := `
		UPDATE outbox
		SET attempts = attempts + 1,
		    last_error = @lastError,
		    status = CASE WHEN attempts + 1 >= @maxAttempts THEN 'failed' ELSE 'pending' END,
		    failed_at = CASE WHEN attempts + 1 >= @maxAttempts THEN NOW() ELSE failed_at END
		WHERE id = @id`
	_, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error marking outbox record failed", err)
	}
	return nil
}
```

- [ ] **Step 2: Write the integration test**

`services/email/internal/test/outbox/outbox_test.go`:

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

func TestOutboxRepository(t *testing.T) {
	env := utils.NewIntegrationTestEnvironment(t)

	fetched := []mail.FetchedMail{{
		Mail: mail.NewMail{
			MessageID:      "outbox-test-1",
			SettingID:      1,
			OrganizationID: 1,
			Subject:        "Subject",
			Sender:         "sender@example.com",
			Body:           "Body",
			Date:           time.Now(),
		},
		Event: mail.OutboxEvent{
			Topic:   "email-received",
			Key:     []byte("1"),
			Payload: []byte(`{"subject":"Subject"}`),
		},
		UID: 1,
	}}

	results, err := env.DB.SaveFetchedMails(context.Background(), fetched)
	assert.Nil(t, err)
	assert.True(t, results[0].Inserted)

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

	fetched := []mail.FetchedMail{{
		Mail: mail.NewMail{
			MessageID:      "outbox-test-2",
			SettingID:      1,
			OrganizationID: 1,
			Subject:        "Subject",
			Sender:         "sender@example.com",
			Body:           "Body",
			Date:           time.Now(),
		},
		Event: mail.OutboxEvent{
			Topic:   "email-received",
			Key:     []byte("1"),
			Payload: []byte(`{"subject":"Subject"}`),
		},
		UID: 2,
	}}
	_, err := env.DB.SaveFetchedMails(context.Background(), fetched)
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

The two subtests in `TestOutboxRepositoryMarkFailed` run sequentially (default Go `t.Run` behavior, no `t.Parallel()`): the first leaves `recordID` re-claimed (`status = 'processing'`, `attempts = 1`) after its own `FetchBatch` check; the second's `MarkFailed(..., 2)` then pushes `attempts` to 2, crossing the threshold.

- [ ] **Step 3: Run the integration tests**

Run: `task test:email:integration`
Expected: PASS, all subtests in both `TestMailRepository` (Task 4), `TestOutboxRepository`, and `TestOutboxRepositoryMarkFailed` green. (Requires Docker running locally.)

- [ ] **Step 4: Commit**

```bash
git add services/email/internal/infra/postgres/outbox_repository.go services/email/internal/test/outbox/outbox_test.go
git commit -m "feat(email): implement Postgres outbox.Store with atomic claim+TTL, integration tests"
```

---

### Task 9: `outbox-relay` sidecar binary

**Files:**
- Create: `services/email/cmd/outbox-relay/config.go`
- Create: `services/email/cmd/outbox-relay/main.go`

**Interfaces:**
- Consumes: `outbox.NewProducer`, `outbox.NewRelay`, `outbox.WithPollInterval/WithBatchSize/WithMaxAttempts/WithClaimTTL` (Task 7); `postgres.NewClient`, and `(*postgres.Client)` satisfying `outbox.Store` (Task 8).

- [ ] **Step 1: Write `config.go`**

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
		PollInterval: readDurationEnvVariable(ctx, "OUTBOX_POLL_INTERVAL", 1*time.Second),
		BatchSize:    readIntEnvVariable(ctx, "OUTBOX_BATCH_SIZE", 100),
		MaxAttempts:  readIntEnvVariable(ctx, "OUTBOX_MAX_ATTEMPTS", 5),
		ClaimTTL:     readDurationEnvVariable(ctx, "OUTBOX_CLAIM_TTL", 30*time.Second),
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

- [ ] **Step 2: Write `main.go`**

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
	log := logger.New("outbox-relay")
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

`db` (`*postgres.Client`) satisfies `outbox.Store` structurally via the three methods added in Task 8.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./services/email/cmd/outbox-relay/...`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add services/email/cmd/outbox-relay/
git commit -m "feat(email): add outbox-relay sidecar binary"
```

---

### Task 10: Dockerfile, Air config, and Taskfile targets for the sidecar

**Files:**
- Create: `services/email/Dockerfile.outbox-relay`
- Create: `services/email/air.outbox-relay.toml`
- Modify: `Taskfile.yml`

- [ ] **Step 1: Write the Dockerfile**

`services/email/Dockerfile.outbox-relay` (at the service root, not nested under `cmd/`, per your preference — distinct from the main service's own future `services/email/Dockerfile`):

```dockerfile
# syntax=docker/dockerfile:1

FROM golang:1.26.5-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/outbox-relay ./services/email/cmd/outbox-relay

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/outbox-relay /outbox-relay
ENTRYPOINT ["/outbox-relay"]
```

Built from the monorepo root as context: `docker build -f services/email/Dockerfile.outbox-relay .`

- [ ] **Step 2: Write the Air config for local hot-reload**

`services/email/air.outbox-relay.toml`:

```toml
root = "."
tmp_dir = "tmp/outbox-relay"

[build]
cmd = "go build -o ./tmp/outbox-relay/app ./cmd/outbox-relay"
bin = "./tmp/outbox-relay/app"
full_bin = "./tmp/outbox-relay/app"
log = "air_errors.log"
include_ext = ["go", "yaml"]
exclude_dir = ["tmp"]
delay = 500

[log]
time = true
[color]

[misc]
clean_on_exit = true
```

- [ ] **Step 3: Add Taskfile targets**

In `Taskfile.yml`, add a new task right after the existing `run:email` task (in the "Service Management" section):

```yaml
  run:email:outbox-relay:
    desc: Run email outbox-relay sidecar with Air hot-reload
    dir: services/email
    cmds:
      - air -c air.outbox-relay.toml
```

Add a new task right after the existing `build:email` task (in the "Building" section):

```yaml
  build:email:outbox-relay:
    desc: Build Docker image for email outbox-relay sidecar
    cmds:
      - echo "Building Docker image for email-outbox-relay with tag {{.VERSION}}..."
      - docker build -t {{.DOCKER_REGISTRY}}/email-outbox-relay:{{.VERSION}} -f services/email/Dockerfile.outbox-relay .
```

Update the existing `build:all` task to include it:

```yaml
  build:all:
    desc: Build Docker images for all services
    cmds:
      - task: build:core
      - task: build:email
      - task: build:email:outbox-relay
```

- [ ] **Step 4: Verify the Taskfile is syntactically valid**

Run: `task --list-all`
Expected: exit 0, output includes `run:email:outbox-relay` and `build:email:outbox-relay` in the listing.

- [ ] **Step 5: Commit**

```bash
git add services/email/Dockerfile.outbox-relay services/email/air.outbox-relay.toml Taskfile.yml
git commit -m "feat(email): add Dockerfile, Air config, and Taskfile targets for outbox-relay"
```

---

### Task 11: Docker Compose wiring for `email-outbox-relay`

**Files:**
- Modify: `dev/docker-compose.yaml`

- [ ] **Step 1: Add the `email-outbox-relay` service**

Add this new service to `dev/docker-compose.yaml`'s `services:` block (after the `kafka-ui` service):

```yaml
  # email outbox relay sidecar - drains services/email's outbox table to Kafka
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

Note the difference from `services/core`/`services/email`'s own `.env` files: this container runs *inside* the Docker network (unlike core/email, which run on the host via Air), so it reaches Postgres via the service name `postgres` (not `localhost`) and Kafka via the internal listener `kafka:19092` (not `localhost:9092`) — the same internal address `kafka-ui` already uses.

Expected on first `task up`: this container starts before `task migrate:core`/`task migrate:email` run, so it will poll a Postgres database whose `outbox` table doesn't exist yet and log query errors for a few seconds until migrations land. This is expected, not a regression — the poll loop logs and continues (per `Relay.Run`'s error handling), and it self-recovers as soon as the `outbox` table exists.

- [ ] **Step 2: Validate the compose file syntax**

Run: `docker compose -f dev/docker-compose.yaml config --quiet`
Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add dev/docker-compose.yaml
git commit -m "feat(dev): wire email-outbox-relay into docker-compose"
```

---

### Task 12: Full workspace verification

**Files:** none (verification only)

- [ ] **Step 1: Build the entire module**

Run: `go build ./...`
Expected: exit 0, no output.

- [ ] **Step 2: Format check**

Run: `gofmt -l .`
Expected: no output. If any file is listed, run `gofmt -w <file>` and re-check.

- [ ] **Step 3: Vet**

Run: `go vet ./...`
Expected: exit 0, no output.

- [ ] **Step 4: Confirm no stray references to the removed `SeqNum` field or old `FetchAllUnseenMails` name**

Run:
```bash
grep -rn "SeqNum\|FetchAllUnseenMails" --include="*.go" .
```
Expected: no output.

- [ ] **Step 5: Confirm `services/email`'s main service no longer imports Kafka client code directly**

Run:
```bash
grep -rn "twmb/franz-go" services/email/internal/config services/email/cmd/main.go services/email/internal/infra/email
```
Expected: no output (only `libs/outbox`, `services/email/cmd/outbox-relay`, and `libs/events` should reference franz-go — confirm with `grep -rln "twmb/franz-go" --include="*.go" .` that the only hits are in `libs/events/`, `libs/outbox/`, and `services/email/cmd/outbox-relay/`).

- [ ] **Step 6: Run all email unit and integration tests**

Run: `task test:email:unit && task test:email:integration`
Expected: both PASS (integration requires Docker running).

- [ ] **Step 7: Run `libs/outbox`'s tests explicitly**

Neither `test:email:unit` (scoped to `./services/email/...`) nor `test:core:unit` (scoped to `./services/core/...`) covers `libs/`, so Task 7's tests are never re-run by the Taskfile targets — only this step catches a regression here going forward.

Run: `go test ./libs/outbox/... -v -short -count=1`
Expected: PASS.

- [ ] **Step 8: Run core unit tests to confirm nothing else broke**

Run: `task test:core:unit`
Expected: PASS (this work doesn't touch `services/core`, so this is a no-op confirmation).

- [ ] **Step 9: Commit (only if step 2 required fixes; otherwise skip — do not create an empty commit)**

```bash
git add -A
git commit -m "chore: gofmt fixes"
```

---

## Explicitly Deferred (do not do in this plan)

Per the approved spec (`docs/superpowers/specs/2026-07-10-email-outbox-pattern-design.md`): message headers/trace-context propagation (documented there with the exact column/approach for later), outbox table retention/cleanup job, schema-registry integration, reuse of `libs/outbox` by any service other than `services/email`, in-process worker-pool concurrency within the relay, and scaling the relay to multiple replicas while preserving per-key order (a key-hash partitioning approach is noted as the promising future direction, not designed further here). Also unrelated and out of scope: the pre-existing missing `services/core/Dockerfile` and `services/email/Dockerfile` (the main services' own images, not the sidecar's).
