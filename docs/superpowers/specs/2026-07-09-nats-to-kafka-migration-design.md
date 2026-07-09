# NATS → Kafka Migration (franz-go)

## Purpose

Replace NATS/JetStream with Kafka as the messaging backbone for the `email.received` event flow between `services/email` (publisher) and `services/core` (subscriber). This is an explicitly transitional step: the goal is a low-effort, low-risk swap that keeps the system working exactly as it does today. The way this codebase uses Kafka (topic design, consumer group strategy, retry/DLQ handling, multi-topic support) will be redesigned in a later pass — this migration should not anticipate that redesign.

## Non-goals

- Redesigning the event/messaging abstraction, topic naming scheme, or consumer group strategy beyond what's needed to functionally replace NATS.
- Building retry/dead-letter infrastructure. Failure handling intentionally stays simple (see below).
- Adding integration tests for the messaging layer (none exist today for NATS either).
- Managing Kafka topics via infrastructure-as-code. Topic auto-creation is relied on for now; explicit topic management will move to an OpenTofu-managed infra folder in a future, separate piece of work.

## Current state (NATS)

- `libs/events` is the only package touching NATS. It exposes `EventServiceInterface` with `PublishEmailReceived`, `SubscribeEmailReceived`, `IsConnected`, used unchanged by both services.
- `EventService` wraps `*nats.Conn` + a JetStream `Stream` (name `EVENTS`, subject wildcard `events.>`, `WorkQueuePolicy` retention).
- Subject: `events.email.received`. Durable consumer name: `email-receiver`.
- Publish: `js.Publish(ctx, subject, data)` (JSON-marshaled `EmailCreatedPayload`), synchronous, returns error.
- Subscribe: `consumer.Consume(handler)` — push-based; handler calls `msg.Ack()` on success, `msg.NakWithDelay(1*time.Minute)` on error (JetStream redelivers only that message later; other messages keep flowing).
- Config: `NATS_URL` env var (`nats://localhost:4222`) in both services' config structs and `.env.template` files.
- Docker: single `nats` container running with `--js` (JetStream) flag, ports 4222/8222.
- Dependency: `github.com/nats-io/nats.go v1.52.0` (+ indirect `nkeys`, `nuid`).
- No existing tests reference NATS.

## Target state (Kafka via franz-go)

### Scope

Only `libs/events` changes internally. Call sites (`services/core/cmd/main.go`, `services/core/internal/infra/events/*.go`, `services/email/cmd/main.go`, `services/email/internal/infra/email/email_service.go`) keep the same function calls; only the config field name changes (`NatsUrl` → `KafkaBrokers`).

### `libs/events` component design

**`EventService` struct**: wraps a single `*kgo.Client` (replaces `*nats.Conn` + `jetstream.Stream`).

**`NewEventService(brokers string) (EventServiceInterface, error)`**: splits `brokers` on `,` into a broker list, creates a `kgo.Client` via `kgo.NewClient(kgo.SeedBrokers(brokers...))`, then calls `client.Ping(ctx)` with a short timeout to fail fast on connection problems (mirrors `nats.Connect`'s synchronous connect-or-fail behavior). No stream/topic creation call — Kafka auto-creates topics on first produce/consume (`auto.create.topics.enable=true` on the broker).

**`Publish(ctx, topic string, data []byte) error`**: `client.ProduceSync(ctx, &kgo.Record{Topic: topic, Value: data}).FirstErr()`. Synchronous, returns error directly — same call-site shape as today's `Publish`.

**`Subscribe(ctx, groupName, topic string, handler func([]byte) error) error`**: creates a second, call-scoped `kgo.Client` configured with `kgo.ConsumerGroup(groupName)`, `kgo.ConsumeTopics(topic)`, `kgo.DisableAutoCommit()`. This mirrors today's shape, where `CreateOrUpdateConsumer` + `Consume(handler)` both happen inside `Subscribe`. Spawns one goroutine running a `PollFetches(ctx)` loop for the lifetime of the passed context. For each fetched record:
  - call `handler(record.Value)`
  - on success (`nil` error): `client.CommitRecords(ctx, record)`
  - on error: log the error and skip the commit — **no redelivery-with-delay loop, no blocking retry**. The record is simply not committed; if the process restarts, the consumer group will redeliver from the last committed offset. This is the closest low-effort match to today's ack/nak behavior without building real retry infrastructure, per explicit decision to keep this simple for now.

**`IsConnected() bool`**: `client.Ping(ctx)` with a short timeout, returns whether it errored.

**`Close()`**: `client.Close()`.

### `email_received.go` (both `libs/events` and `services/core/internal/infra/events`)

Same structure as today: package-level `topic = "email-received"` (renamed from `events.email.received` — Kafka convention avoids dots) and `subscriptionName = "email-receiver"` (unchanged consumer group name, explicitly kept as-is per current design even though it's known to not generalize — that redesign is future work). `PublishEmailReceived` JSON-marshals `EmailCreatedPayload` and calls `Publish`. `SubscribeEmailReceived`'s handler signature changes from `func(jetstream.Msg)` to a plain `func(data []byte) error` closure that JSON-unmarshals into `EmailCreatedPayload` and invokes the caller's callback — same marshal/unmarshal logic as today.

### Config changes

- `services/core/internal/config/config.go`: field `NatsUrl` → `KafkaBrokers`, env var `NATS_URL` → `KAFKA_BROKERS`.
- `services/email/internal/config/config.go`: same rename.
- `services/core/.env.template` and `services/email/.env.template`: `NATS_URL=nats://localhost:4222` → `KAFKA_BROKERS=localhost:9092`.

### Docker Compose (`dev/docker-compose.yaml`)

Replace the `nats` service with a single-node KRaft-mode Kafka broker (no Zookeeper), using the official `apache/kafka` image in combined broker+controller mode, exposing `9092`, with `auto.create.topics.enable=true`.

Also add a `kafka-ui` service running **kafbat-ui** (community-maintained fork of `provectus/kafka-ui`), pointed at the broker's internal listener, exposed on a dev-friendly port (e.g. `8080`) for browsing topics, consumer groups/lag, and messages during local development.

### Dependencies (`go.mod`)

Remove `github.com/nats-io/nats.go` and its indirect deps (`nats-io/nkeys`, `nats-io/nuid`). Add `github.com/twmb/franz-go` (`kgo` package).

### Testing

No existing tests reference NATS, so there is nothing to migrate. No new integration tests are added in this pass — messaging-layer testing is out of scope until the broader Kafka redesign.

## Explicitly deferred (future redesign)

- Per-domain/per-event consumer group naming strategy (currently hardcoded, known limitation, intentionally kept).
- Real retry/dead-letter handling for failed message processing.
- Explicit topic provisioning via OpenTofu-managed infrastructure.
- Any multi-topic or multi-consumer-group support beyond the single `email-received` flow.
