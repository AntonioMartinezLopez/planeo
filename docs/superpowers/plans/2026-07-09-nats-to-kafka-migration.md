# NATS to Kafka Migration (franz-go) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace NATS/JetStream with Kafka (via the `franz-go` client) as the transport for the `email.received` event flow between `services/email` and `services/core`, keeping the public shape of `libs/events` unchanged so call sites barely move.

**Architecture:** `libs/events.EventService` currently wraps `*nats.Conn` + a JetStream stream and exposes `PublishEmailReceived` / `SubscribeEmailReceived` / `IsConnected`. It is rewritten to wrap a `*kgo.Client` (franz-go). `Publish` becomes a synchronous `ProduceSync` call. `Subscribe` creates a second, call-scoped `kgo.Client` configured with a consumer group and runs a `PollFetches` loop in a goroutine, committing the offset only after the handler succeeds (on failure it logs and skips the commit — no retry/DLQ infrastructure, per the approved spec). Topic name changes from `events.email.received` to `email-received` (Kafka naming convention); consumer group name stays `email-receiver` (known placeholder, intentionally unchanged). `NATS_URL` becomes `KAFKA_BROKERS` in both services. The dev Kafka broker runs single-node KRaft mode (no Zookeeper) via the `apache/kafka` image, plus a `kafbat-ui` container for inspecting topics/consumers/messages.

**Tech Stack:** Go 1.24.5+ (module targets 1.26.5 per `go.mod`), `github.com/twmb/franz-go/pkg/kgo`, Docker Compose, `apache/kafka` image (KRaft), `ghcr.io/kafbat/kafka-ui` image.

## Global Constraints

- Keep `libs/events`'s public function names and signatures on `EventServiceInterface` unchanged: `PublishEmailReceived`, `SubscribeEmailReceived`, `IsConnected`. Only internals change.
- Consumer group name stays hardcoded as `email-receiver` — do not generalize or parameterize it in this pass (explicit, approved scope decision).
- No new automated tests for the messaging layer in this pass (approved spec explicitly defers this — matches today's NATS test coverage, which is also zero).
- No message-level retry/DLQ logic — on handler error, log and skip the offset commit. Nothing more.
- Rely on Kafka topic auto-creation (`auto.create.topics.enable=true`) — no explicit topic provisioning in this pass.
- All work happens on branch `feature/migrate-nats-to-kafka` (already checked out). Do not commit to `main`.
- Reference spec: `docs/superpowers/specs/2026-07-09-nats-to-kafka-migration-design.md`.

---

### Task 1: Rewrite `libs/events` to use franz-go

**Files:**
- Modify: `libs/events/service.go`
- Modify: `libs/events/email_received.go`
- Modify: `go.mod`, `go.sum` (via `go get` / `go mod tidy`, not hand-edited)

**Interfaces:**
- Produces: `events.NewEventService(brokers string) (events.EventServiceInterface, error)` where `brokers` is a comma-separated list (e.g. `"localhost:9092"`).
- Produces: `EventServiceInterface.PublishEmailReceived(ctx context.Context, payload EmailCreatedPayload) error` — unchanged signature.
- Produces: `EventServiceInterface.SubscribeEmailReceived(ctx context.Context, handler func(EmailCreatedPayload) error) error` — unchanged signature.
- Produces: `EventServiceInterface.IsConnected() bool` — unchanged signature.
- Produces (unexported, used only within `libs/events`): `EventService.Publish(ctx, topic string, data []byte) error`, `EventService.Subscribe(ctx, groupName, topic string, handler func(data []byte) error) error`.

- [ ] **Step 1: Replace `libs/events/service.go` with the franz-go-based implementation**

```go
package events

import (
	"context"
	"strings"
	"time"

	"planeo/libs/logger"

	"github.com/twmb/franz-go/pkg/kgo"
)

type EventService struct {
	Client  *kgo.Client
	Brokers []string
}

type EventServiceInterface interface {
	SubscribeEmailReceived(ctx context.Context, callback func(payload EmailCreatedPayload) error) error
	PublishEmailReceived(ctx context.Context, payload EmailCreatedPayload) error
	IsConnected() bool
}

func NewEventService(brokers string) (EventServiceInterface, error) {
	seeds := strings.Split(brokers, ",")

	client, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
	)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}

	return &EventService{Client: client, Brokers: seeds}, nil
}

func (es *EventService) IsConnected() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return es.Client.Ping(ctx) == nil
}

func (es *EventService) Publish(ctx context.Context, topic string, data []byte) error {
	record := &kgo.Record{Topic: topic, Value: data}

	results := es.Client.ProduceSync(ctx, record)

	return results.FirstErr()
}

func (es *EventService) Subscribe(ctx context.Context, groupName string, topic string, handler func(data []byte) error) error {
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(es.Brokers...),
		kgo.ConsumerGroup(groupName),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)

	go func() {
		defer consumer.Close()

		for {
			if ctx.Err() != nil {
				return
			}

			fetches := consumer.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return
			}

			fetches.EachError(func(_ string, _ int32, err error) {
				log.Error().Err(err).Msg("kafka fetch error")
			})

			fetches.EachRecord(func(record *kgo.Record) {
				if err := handler(record.Value); err != nil {
					log.Error().Err(err).Msg("failed to process kafka message, skipping commit")
					return
				}

				if err := consumer.CommitRecords(ctx, record); err != nil {
					log.Error().Err(err).Msg("failed to commit kafka offset")
				}
			})
		}
	}()

	return nil
}

func (es *EventService) Close() {
	es.Client.Close()
}
```

This drops the old `EventMessage` interface (dead code — it referenced NATS-specific `Subject()`/`Ack()` naming and had no callers) and the JetStream stream/consumer setup, replacing them with a plain `kgo.Client` plus a per-`Subscribe`-call consumer client.

- [ ] **Step 2: Replace `libs/events/email_received.go`**

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

var topic = "email-received"
var subscriptionName = "email-receiver"

func (es *EventService) PublishEmailReceived(ctx context.Context, payload EmailCreatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return es.Publish(ctx, topic, data)
}

func (es *EventService) SubscribeEmailReceived(ctx context.Context, handler func(EmailCreatedPayload) error) error {
	return es.Subscribe(ctx, subscriptionName, topic, func(data []byte) error {
		var payload EmailCreatedPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			return err
		}

		return handler(payload)
	})
}
```

- [ ] **Step 3: Add franz-go dependency and remove the NATS dependency**

Run:
```bash
go get github.com/twmb/franz-go/pkg/kgo@latest
go mod tidy
```

Expected: `go.mod` now has a `require github.com/twmb/franz-go vX.Y.Z` line, and `github.com/nats-io/nats.go`, `github.com/nats-io/nkeys`, `github.com/nats-io/nuid` are gone (their only importer was `libs/events`, just rewritten). Confirm with:

```bash
grep -i nats go.mod
```
Expected: no output.

- [ ] **Step 4: Verify the package compiles standalone**

Run: `go build ./libs/events/...`
Expected: exits 0, no output.

- [ ] **Step 5: Commit**

```bash
git add libs/events/service.go libs/events/email_received.go go.mod go.sum
git commit -m "feat: rewrite libs/events to use Kafka (franz-go) instead of NATS"
```

---

### Task 2: Update `services/core` config and wiring

**Files:**
- Modify: `services/core/internal/config/config.go:15,55`
- Modify: `services/core/cmd/main.go:77,80`

**Interfaces:**
- Consumes: `events.NewEventService(brokers string)` from Task 1.
- Produces: `config.ApplicationConfiguration.KafkaBrokers string` (replaces `NatsUrl`), read from env var `KAFKA_BROKERS`.

- [ ] **Step 1: Rename the config field and env var in `services/core/internal/config/config.go`**

Change line 15 from:
```go
	NatsUrl             string
```
to:
```go
	KafkaBrokers        string
```

Change line 55 from:
```go
		NatsUrl:             readEnvVariable(ctx, "NATS_URL"),
```
to:
```go
		KafkaBrokers:        readEnvVariable(ctx, "KAFKA_BROKERS"),
```

- [ ] **Step 2: Update the call site in `services/core/cmd/main.go`**

Change lines 76-81 from:
```go
	// initialize event service
	err := coreEvents.InitializeEvents(ctx, config.NatsUrl, coreEvents.Services{RequestService: requestService, CategoryService: categoryService})

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to NATS")
	}
```
to:
```go
	// initialize event service
	err := coreEvents.InitializeEvents(ctx, config.KafkaBrokers, coreEvents.Services{RequestService: requestService, CategoryService: categoryService})

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}
```

Note: `services/core/internal/infra/events/events.go` and `email_received.go` need **no changes** — their `InitializeEvents(ctx, messengerUrl string, ...)` parameter is already named generically and passes straight through to `events.NewEventService`.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./services/core/...`
Expected: fails at this point only if `services/core/.env.template` / actual `.env` still reference `NATS_URL` — that's fine, this is a compile check, not a runtime check. Compilation itself should succeed with exit 0.

- [ ] **Step 4: Commit**

```bash
git add services/core/internal/config/config.go services/core/cmd/main.go
git commit -m "feat(core): switch event service wiring from NATS_URL to KAFKA_BROKERS"
```

---

### Task 3: Update `services/email` config and wiring

**Files:**
- Modify: `services/email/internal/config/config.go:15,49`
- Modify: `services/email/cmd/main.go:28,30`

**Interfaces:**
- Consumes: `events.NewEventService(brokers string)` from Task 1.
- Produces: `config.ApplicationConfiguration.KafkaBrokers string` (replaces `NatsUrl`), read from env var `KAFKA_BROKERS`.

- [ ] **Step 1: Rename the config field and env var in `services/email/internal/config/config.go`**

Change line 15 from:
```go
	NatsUrl         string
```
to:
```go
	KafkaBrokers    string
```

Change line 49 from:
```go
		NatsUrl:         readEnvVariable(ctx, "NATS_URL"),
```
to:
```go
		KafkaBrokers:    readEnvVariable(ctx, "KAFKA_BROKERS"),
```

- [ ] **Step 2: Update the call site in `services/email/cmd/main.go`**

Change lines 28-31 from:
```go
	eventService, err := events.NewEventService(cfg.NatsUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to NATS")
	}
```
to:
```go
	eventService, err := events.NewEventService(cfg.KafkaBrokers)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}
```

Note: `services/email/internal/infra/email/email_service.go` needs **no changes** — it only calls `eventService.PublishEmailReceived(...)`, whose signature is unchanged.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./services/email/...`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add services/email/internal/config/config.go services/email/cmd/main.go
git commit -m "feat(email): switch event service wiring from NATS_URL to KAFKA_BROKERS"
```

---

### Task 4: Update env templates

**Files:**
- Modify: `services/core/.env.template:5`
- Modify: `services/email/.env.template:5`

- [ ] **Step 1: Update `services/core/.env.template`**

Change line 5 from:
```
NATS_URL=nats://localhost:4222
```
to:
```
KAFKA_BROKERS=localhost:9092
```

- [ ] **Step 2: Update `services/email/.env.template`**

Change line 5 from:
```
NATS_URL=nats://localhost:4222
```
to:
```
KAFKA_BROKERS=localhost:9092
```

- [ ] **Step 3: Update your local, gitignored `.env` files to match**

Run (only if you have local `.env` files already copied from the templates):
```bash
grep -l NATS_URL services/core/.env services/email/.env 2>/dev/null
```
For any file listed, manually replace `NATS_URL=nats://localhost:4222` with `KAFKA_BROKERS=localhost:9092`.

- [ ] **Step 4: Commit**

```bash
git add services/core/.env.template services/email/.env.template
git commit -m "chore: replace NATS_URL with KAFKA_BROKERS in env templates"
```

---

### Task 5: Replace the NATS container with Kafka + kafbat-ui in Docker Compose

**Files:**
- Modify: `dev/docker-compose.yaml:68-76`

- [ ] **Step 1: Replace the `nats` service block**

In `dev/docker-compose.yaml`, replace lines 68-76:
```yaml
# nats messaging (single node with JetStream)
  nats:
    container_name: nats
    image: nats
    entrypoint: /nats-server
    command: --server_name N1 --js --sd /data -p 4222
    ports:
      - 4222:4222
      - 8222:8222
```
with:
```yaml
  # kafka messaging (single node, KRaft mode - no Zookeeper)
  kafka:
    container_name: kafka
    image: apache/kafka:latest
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: PLAINTEXT://kafka:19092,CONTROLLER://kafka:9093,PLAINTEXT_HOST://0.0.0.0:9092
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:19092,PLAINTEXT_HOST://localhost:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
      CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk
    ports:
      - 9092:9092

  # kafka UI for inspecting topics, consumer groups and messages
  kafka-ui:
    container_name: kafka-ui
    image: ghcr.io/kafbat/kafka-ui:latest
    depends_on:
      - kafka
    environment:
      KAFKA_CLUSTERS_0_NAME: local
      KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS: kafka:19092
    ports:
      - 8082:8080
```

`kafka` listens on two ports: `19092` for other containers on the compose network (used by `kafka-ui`, via the `PLAINTEXT` listener) and `9092` for the host machine (used by `core`/`email` running via Air outside Docker, via the `PLAINTEXT_HOST` listener). `kafka-ui` will be reachable at `http://localhost:8082`.

- [ ] **Step 2: Validate the compose file syntax**

Run: `docker compose -f dev/docker-compose.yaml config --quiet`
Expected: exits 0, no output (no YAML/schema errors).

- [ ] **Step 3: Commit**

```bash
git add dev/docker-compose.yaml
git commit -m "feat(dev): replace NATS container with Kafka (KRaft) + kafbat-ui"
```

---

### Task 6: Full workspace build verification

**Files:** none (verification only)

- [ ] **Step 1: Build the entire module**

Run: `go build ./...`
Expected: exit 0, no output.

- [ ] **Step 1b: Fix formatting from field renames**

Run: `gofmt -l .`
Expected: may list `services/core/internal/config/config.go` and `services/email/internal/config/config.go` (struct field alignment shifted after renaming `NatsUrl`/`KafkaBrokers`). If listed, run `gofmt -w <file>` on each, then re-run `gofmt -l .` and confirm no output.

- [ ] **Step 2: Run `go vet`**

Run: `go vet ./...`
Expected: exit 0, no output.

- [ ] **Step 3: Confirm no NATS references remain in Go code or env templates**

Run:
```bash
grep -ril nats --include="*.go" --include="*.env.template" .
```
Expected: no output (empty result — if anything prints, go back and clean it up before proceeding).

- [ ] **Step 4: Run existing unit tests to confirm nothing else broke**

Run: `task test:core:unit`
Expected: all tests pass (this migration doesn't touch domain/unit-tested code, so this should be a no-op confirmation).

- [ ] **Step 5: Commit (only if step 3 required cleanup changes; otherwise skip)**

```bash
git add -A
git commit -m "chore: clean up remaining NATS references"
```

---

### Task 7: Manual end-to-end smoke test

**Files:** none (manual verification only)

This migration has no automated integration tests (approved spec explicitly defers messaging-layer test coverage). Verify the flow works end-to-end manually.

- [ ] **Step 1: Bring up the dev environment**

Run: `task up`
Expected: `postgres`, `pgadmin`, `keycloak`, `greenmail`, `roundcube`, `kafka`, `kafka-ui` containers all start healthy. Confirm with:
```bash
docker compose -f dev/docker-compose.yaml ps
```
All listed services should show `Up`/`running`.

- [ ] **Step 2: Open kafbat-ui and confirm the broker is reachable**

Open `http://localhost:8082` in a browser. Expected: the `local` cluster shows as online with 0 topics (nothing produced yet).

- [ ] **Step 3: Start core and email services**

In separate terminals:
```bash
task run:core
```
```bash
task run:email
```
Expected: both start without `Failed to connect to Kafka` fatal errors in their logs.

- [ ] **Step 4: Send a test email through greenmail/roundcube**

Using the existing dev email setup (roundcube at `http://localhost:8089`, configured against greenmail), send an email to an address the email service is configured to poll (per existing `setting` configuration — use whatever test settings already exist for local dev, unchanged by this migration).

- [ ] **Step 5: Confirm the message flowed through Kafka**

In kafbat-ui (`http://localhost:8082`), navigate to Topics. Expected: a topic named `email-received` now exists with 1 message. Navigate to Consumer Groups. Expected: a group named `email-receiver` exists with 0 lag (message consumed).

- [ ] **Step 6: Confirm the request was created in core**

Check the core service logs for `"Received email"` and `"Failed to create request from email"` absence, or query the API/DB directly to confirm a new request row exists for the test email's `MessageID`.

- [ ] **Step 7: Report results**

If all steps pass, the migration is functionally complete. If step 5 or 6 fails, use superpowers:systematic-debugging before making further changes.

---

## Explicitly deferred (do not do in this plan)

Per the approved spec (`docs/superpowers/specs/2026-07-09-nats-to-kafka-migration-design.md`), the following are **out of scope** here and tracked for a future redesign: per-domain consumer group naming, message keys/ordering, dead-letter/retry handling, multi-partition/multi-broker topology, schema registry, consumer lag alerting/metrics, graceful shutdown of the poll loop, auth/encryption on the broker, and integration tests for the messaging layer.
