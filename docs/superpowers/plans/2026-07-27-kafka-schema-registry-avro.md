# Kafka Schema Registry + Avro Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hand-maintained JSON Kafka payloads with Avro schemas registered in a local Apicurio schema registry, and fix the domain-layer serialization violation in `domain/mail` and `domain/inbox` along the way.

**Architecture:** Two new packages — `libs/schemaregistry` (generic Kafka/registry/Avro engine, zero app knowledge, built on `franz-go/pkg/sr`'s lower-level client + Go generics) and `libs/eventschemas` (the `email_created.avsc` schema, its `avrogen`-generated Go type, and subject/topic constants). Infra adapters (`infra/events.EmailCreatedProducer`, `infra/inbox.EmailReceivedConsumer`) are the only places that couple a specific type to the generic engine. `domain/mail` and `domain/inbox` become fully free of wire-format knowledge.

**Tech Stack:** Go 1.26.5, `github.com/twmb/franz-go/pkg/sr`, `github.com/iskorotkov/avro/v2` (+ its `avrogen` codegen tool), Apicurio Registry 3.3.0 (Docker), Task, goose, mockery, testcontainers-go.

## Global Constraints

- Module: `planeo`, Go `1.26.5` (per root `go.mod` — the `1.24.5` figure in `CLAUDE.md` is stale; use the real toolchain version).
- Compatibility mode: BACKWARD (spec Section 1) — not configured by this plan (Apicurio's default; no per-subject rule-setting code is in scope for v1).
- Wire format: Confluent standard envelope (magic byte + 4-byte schema ID), via `sr.ConfluentHeader` — never hand-rolled.
- Subject naming: TopicNameStrategy — `email.received-value`.
- `libs/schemaregistry` must never import `libs/eventschemas`, `services/email`, or `services/core`. `libs/eventschemas` must never import `libs/schemaregistry`.
- No placeholder/TODO code — every task below is complete, runnable code.
- Every new Task target follows the existing `Taskfile.yml` style (`desc:`, `dir:` where needed, `cmds:`).
- Every new migration follows the existing goose `-- +goose Up/Down` `StatementBegin/StatementEnd` format and `YYYYMMDDHHmmss_name.sql` naming.

---

## Task 1: Add Avro + schema registry dependencies

**Files:**
- Modify: `go.mod`, `go.sum` (via `go get`)

**Interfaces:**
- Produces: `github.com/twmb/franz-go/pkg/sr` and `github.com/iskorotkov/avro/v2` available as imports for all later tasks.

- [ ] **Step 1: Add the dependencies**

Run:
```bash
go get github.com/twmb/franz-go/pkg/sr@v1.21.5
go get github.com/iskorotkov/avro/v2@latest
```

(`pkg/sr` is a sub-module of `franz-go`, already at `v1.21.5` in `go.mod` — pin the same version so the two stay in lockstep.)

- [ ] **Step 2: Verify the build still compiles**

Run: `go build ./...`
Expected: exits 0, no errors (nothing imports the new packages yet, so this only proves `go.mod`/`go.sum` are consistent).

- [ ] **Step 3: Install the `avrogen` CLI tool**

Run:
```bash
go install github.com/iskorotkov/avro/v2/cmd/avrogen@latest
```

Confirm it's on `PATH`: `avrogen --help` should print usage. If the fork's module path for the CLI differs from `github.com/iskorotkov/avro/v2/cmd/avrogen` (unverified — the fork is described as drop-in compatible with `hamba/avro`'s `avrogen`, but the exact cmd path wasn't confirmed during spec review), check `https://github.com/iskorotkov/avro` for the correct install path before continuing, and use that instead.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "Add franz-go/pkg/sr and iskorotkov/avro dependencies"
```

---

## Task 2: `libs/schemaregistry` — generic engine, wrapping `sr.Serde`

**Files:**
- Create: `libs/schemaregistry/client.go`
- Create: `libs/schemaregistry/serde.go`
- Test: `libs/schemaregistry/serde_test.go`

**Interfaces:**
- Produces: `schemaregistry.Config{URL string}`, `schemaregistry.NewClient(cfg Config, extraOpts ...sr.ClientOpt) (*sr.Client, error)`, `schemaregistry.Register(ctx, client *sr.Client, subject string, schema avro.Schema) (int, error)`, `schemaregistry.NewSerde(client *sr.Client) *Serde`, `schemaregistry.RegisterEncode[T any](ctx, s *Serde, subject string, schema avro.Schema) error`, `schemaregistry.RegisterDecode[T any](ctx, s *Serde, subject string, schema avro.Schema) error`, `schemaregistry.Encode[T any](s *Serde, v T) ([]byte, error)`, `schemaregistry.Decode[T any](ctx, s *Serde, b []byte, readerSchema avro.Schema, v *T) error`. Tasks 4, 9, 11 depend on these exact names.

This wraps franz-go's own `sr.Serde` (its wire-envelope framing, per-ID dispatch, and caching are used as-is) rather than reimplementing them — see design spec Section 2 for why an earlier fully-hand-rolled version was replaced with this. The only thing added on top is what `sr.Serde` doesn't provide: resolving Avro schema evolution for an incoming ID it wasn't told about in advance, via franz-go's own documented "discover on `ErrNotRegistered`" pattern.

- [ ] **Step 1: Write the failing integration test**

This needs a real Apicurio Registry (schema registration + ID resolution can't be meaningfully faked). Use a generic testcontainer, following the same `testcontainers-go` version (`v0.43.0`) already in `go.mod`:

```go
// libs/schemaregistry/serde_test.go
package schemaregistry_test

import (
	"context"
	"testing"
	"time"

	"planeo/libs/schemaregistry"

	avro "github.com/iskorotkov/avro/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/twmb/franz-go/pkg/sr"
)

type testRecordV1 struct {
	Name string `avro:"name" json:"name"`
}

type testRecordV2 struct {
	Name     string `avro:"name" json:"name"`
	Priority string `avro:"priority" json:"priority"`
}

const testSchemaV1 = `{
  "type": "record", "name": "TestRecord", "namespace": "planeo.test",
  "fields": [{"name": "name", "type": "string"}]
}`

const testSchemaV2 = `{
  "type": "record", "name": "TestRecord", "namespace": "planeo.test",
  "fields": [
    {"name": "name", "type": "string"},
    {"name": "priority", "type": ["null", "string"], "default": null}
  ]
}`

func startRegistry(t *testing.T) *sr.Client {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "apicurio/apicurio-registry:3.3.0",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/apis/ccompat/v7/subjects").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start apicurio container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Errorf("failed to terminate apicurio container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}
	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: "http://" + host + ":" + port.Port() + "/apis/ccompat/v7"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestRegisterEncodeAndDecode_roundTrip(t *testing.T) {
	client := startRegistry(t)
	schema := avro.MustParse(testSchemaV1)

	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[testRecordV1](context.Background(), serde, "test-record-value", schema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}

	encoded, err := schemaregistry.Encode(serde, testRecordV1{Name: "hello"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var out testRecordV1
	if err := schemaregistry.Decode(context.Background(), serde, encoded, schema, &out); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out.Name != "hello" {
		t.Errorf("got Name=%q, want %q", out.Name, "hello")
	}
}

// TestDecode_schemaEvolution proves the discover-on-ErrNotRegistered path:
// a FRESH Serde (simulating a consumer process that has never seen the
// producer's schema ID) must still correctly decode a message written
// with an older schema into a newer reader schema with an additional
// defaulted field, without ever having called RegisterEncode/RegisterDecode
// for that specific ID.
func TestDecode_schemaEvolution(t *testing.T) {
	client := startRegistry(t)
	v1Schema := avro.MustParse(testSchemaV1)
	v2Schema := avro.MustParse(testSchemaV2)

	producerSerde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[testRecordV1](context.Background(), producerSerde, "test-record-evolution-value", v1Schema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}
	encoded, err := schemaregistry.Encode(producerSerde, testRecordV1{Name: "hello"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	consumerSerde := schemaregistry.NewSerde(client) // fresh - has never registered this ID
	var out testRecordV2
	if err := schemaregistry.Decode(context.Background(), consumerSerde, encoded, v2Schema, &out); err != nil {
		t.Fatalf("Decode with v2 reader schema: %v", err)
	}
	if out.Name != "hello" {
		t.Errorf("got Name=%q, want %q", out.Name, "hello")
	}
	if out.Priority != "" {
		t.Errorf("got Priority=%q, want empty (zero value for a null-defaulted field the writer never had)", out.Priority)
	}
}

func TestRegisterDecode_failsFastOnUnknownSubject(t *testing.T) {
	client := startRegistry(t)
	schema := avro.MustParse(testSchemaV1)
	serde := schemaregistry.NewSerde(client)

	// A subject that was never registered by anyone (no prior RegisterEncode
	// call for it) — LookupSchema has nothing to find, so this must error
	// rather than silently succeed. This is what RegisterDecode's use as a
	// startup fail-fast check (design spec Section 5) depends on.
	if err := schemaregistry.RegisterDecode[testRecordV1](context.Background(), serde, "never-registered-value", schema); err == nil {
		t.Fatal("expected an error for a subject with no registered schema, got nil")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./libs/schemaregistry/... -v`
Expected: FAIL — `package schemaregistry` doesn't exist yet. Requires Docker running locally.

- [ ] **Step 3: Write `client.go`**

```go
// libs/schemaregistry/client.go
package schemaregistry

import (
	"context"

	avro "github.com/iskorotkov/avro/v2"
	"github.com/twmb/franz-go/pkg/sr"
)

// Config is the connection configuration for the schema registry client.
// Mirrors libs/kafka.Config's shape and purpose: plain data for the common
// case (today just the URL — auth fields get added here once a non-local
// registry's scheme is decided), so cmd/main.go never needs to import
// franz-go/pkg/sr directly for ordinary construction.
type Config struct {
	URL string
}

// NewClient constructs a schema registry HTTP client from cfg. extraOpts
// lets a caller layer additional sr.ClientOpt values (e.g. a one-off
// sr.DialTLSConfig) on top — this package only ever builds the connection,
// never decides what a caller does with it, same division of concerns as
// libs/kafka.NewClient.
func NewClient(cfg Config, extraOpts ...sr.ClientOpt) (*sr.Client, error) {
	opts := append([]sr.ClientOpt{sr.URLs(cfg.URL)}, extraOpts...)
	return sr.NewClient(opts...)
}

// Register performs a content-based lookup-or-register call for subject
// against schema, returning its registry ID. This is the primitive
// cmd/registerschemas (Task 4) uses to walk eventschemas.Schemas()
// generically — it operates on schemas alone, with no Go type involved,
// unlike RegisterEncode/RegisterDecode in serde.go.
func Register(ctx context.Context, client *sr.Client, subject string, schema avro.Schema) (int, error) {
	ss, err := client.LookupSchema(ctx, subject, sr.Schema{Schema: schema.String(), Type: sr.TypeAvro})
	if err != nil {
		return 0, err
	}
	return ss.ID, nil
}
```

- [ ] **Step 4: Write `serde.go`**

```go
// libs/schemaregistry/serde.go
package schemaregistry

import (
	"context"
	"errors"
	"sync"

	avro "github.com/iskorotkov/avro/v2"
	"github.com/twmb/franz-go/pkg/sr"
)

// Serde is the generic Kafka schema-registry engine: it knows how to
// register Go types for Avro encode/decode against a Confluent-compatible
// registry (via franz-go's own sr.Serde for wire-envelope framing and ID
// dispatch/caching), and how to resolve schema differences for IDs it
// hasn't seen before. It has zero knowledge of what an "email" is — see
// docs/superpowers/specs/2026-07-25-kafka-schema-registry-avro-design.md
// Section 3 for why this stays separate from libs/eventschemas.
type Serde struct {
	client *sr.Client
	inner  *sr.Serde
	mu     sync.Mutex
}

func NewSerde(client *sr.Client) *Serde {
	return &Serde{client: client, inner: &sr.Serde{}}
}

// RegisterEncode resolves subject's ID and registers T for encoding.
// Called once per type, at construction time, by whichever infra adapter
// owns that type — this is the coupling site.
func RegisterEncode[T any](ctx context.Context, s *Serde, subject string, schema avro.Schema) error {
	id, err := Register(ctx, s.client, subject, schema)
	if err != nil {
		return err
	}
	s.inner.Register(id, *new(T),
		sr.EncodeFn(func(v any) ([]byte, error) { return avro.Marshal(schema, v) }),
		sr.DecodeFn(func(b []byte, v any) error { return avro.Unmarshal(schema, b, v) }),
	)
	return nil
}

// RegisterDecode resolves subject's ID for schema and registers T for
// decoding under that exact ID. Since this ID corresponds to our own
// compiled-in schema, no writer/reader resolution is needed for it. This
// doubles as a startup fail-fast check: it errors immediately if the
// registry is unreachable or has no entry for our own schema, rather than
// only surfacing that on the first real message.
func RegisterDecode[T any](ctx context.Context, s *Serde, subject string, schema avro.Schema) error {
	id, err := Register(ctx, s.client, subject, schema)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.inner.Register(id, *new(T), sr.DecodeFn(func(b []byte, v any) error {
		return avro.Unmarshal(schema, b, v)
	}))
	s.mu.Unlock()
	return nil
}

func Encode[T any](s *Serde, v T) ([]byte, error) {
	return s.inner.Encode(v)
}

// Decode delegates to sr.Serde first — the fast path for any ID already
// registered via RegisterEncode/RegisterDecode or a prior Decode call on
// this same Serde. On ErrNotRegistered — an ID this process hasn't seen —
// it fetches that writer schema, resolves it against readerSchema,
// registers the result, and retries. This is franz-go's own documented
// pattern for this situation, not a workaround.
func Decode[T any](ctx context.Context, s *Serde, b []byte, readerSchema avro.Schema, v *T) error {
	err := s.inner.Decode(b, v)
	if !errors.Is(err, sr.ErrNotRegistered) {
		return err
	}

	id, _, hdrErr := (&sr.ConfluentHeader{}).DecodeID(b)
	if hdrErr != nil {
		return hdrErr
	}
	raw, fetchErr := s.client.SchemaByID(ctx, id)
	if fetchErr != nil {
		return fetchErr
	}
	writerSchema, parseErr := avro.Parse(raw.Schema)
	if parseErr != nil {
		return parseErr
	}
	resolved, resolveErr := avro.NewSchemaCompatibility().Resolve(readerSchema, writerSchema)
	if resolveErr != nil {
		return resolveErr
	}

	s.mu.Lock()
	s.inner.Register(id, *new(T), sr.DecodeFn(func(bb []byte, vv any) error { return avro.Unmarshal(resolved, bb, vv) }))
	s.mu.Unlock()

	return s.inner.Decode(b, v)
}
```

If `sr.Serde.Register`/`Encode`/`Decode`, `sr.ErrNotRegistered`, `sr.Schema{Schema: ..., Type: sr.TypeAvro}`, `sr.Client.LookupSchema`/`SchemaByID`, or `avro.NewSchemaCompatibility().Resolve(...)`'s exact usage don't match what's actually in the installed module versions (these were verified against package documentation during spec review, not compiled), run `go doc github.com/twmb/franz-go/pkg/sr Serde` and `go doc github.com/iskorotkov/avro/v2 SchemaCompatibility` to check the real signatures and adjust — the tests in Step 1 are what prove correctness, not this text.

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./libs/schemaregistry/... -v`
Expected: PASS for all three tests.

- [ ] **Step 6: Commit**

```bash
git add libs/schemaregistry
git commit -m "Add libs/schemaregistry: wraps sr.Serde, adds schema-evolution resolution"
```

---

## Task 3: `libs/eventschemas` — schema + generated type

**Files:**
- Create: `libs/eventschemas/schemas/email_created.avsc`
- Create: `libs/eventschemas/email_created_gen.go`
- Create: `libs/eventschemas/eventschemas.go`
- Test: `libs/eventschemas/eventschemas_test.go`

**Interfaces:**
- Produces: `eventschemas.EmailCreated` (struct), `eventschemas.EmailCreatedSchema` (`avro.Schema`), `eventschemas.EmailReceivedTopic`, `eventschemas.EmailReceivedSubject` (strings), `eventschemas.Schemas() map[string]avro.Schema`. Tasks 6, 8, 9, 10, 11, 12 all depend on these exact names.

- [ ] **Step 1: Write the `.avsc` schema**

```json
// libs/eventschemas/schemas/email_created.avsc
{
  "type": "record",
  "name": "EmailCreated",
  "namespace": "planeo.email",
  "fields": [
    {"name": "subject", "type": "string"},
    {"name": "body", "type": "string"},
    {"name": "from", "type": "string"},
    {"name": "date", "type": {"type": "long", "logicalType": "timestamp-millis"}},
    {"name": "messageId", "type": "string"},
    {"name": "organizationId", "type": "int"}
  ]
}
```

- [ ] **Step 2: Write the failing test for `Schemas()`**

```go
// libs/eventschemas/eventschemas_test.go
package eventschemas_test

import (
	"testing"

	"planeo/libs/eventschemas"

	avro "github.com/iskorotkov/avro/v2"
)

func TestSchemas_includesEmailCreated(t *testing.T) {
	schemas := eventschemas.Schemas()
	schema, ok := schemas[eventschemas.EmailReceivedSubject]
	if !ok {
		t.Fatalf("Schemas() missing subject %q", eventschemas.EmailReceivedSubject)
	}
	if schema.String() != eventschemas.EmailCreatedSchema.String() {
		t.Errorf("Schemas()[%q] does not match EmailCreatedSchema", eventschemas.EmailReceivedSubject)
	}
}

func TestEmailCreated_avroRoundTrip(t *testing.T) {
	in := eventschemas.EmailCreated{
		Subject:        "hello",
		Body:           "world",
		From:           "a@b.com",
		MessageId:      "msg-1",
		OrganizationId: 7,
	}
	data, err := avro.Marshal(eventschemas.EmailCreatedSchema, in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out eventschemas.EmailCreated
	if err := avro.Unmarshal(eventschemas.EmailCreatedSchema, data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Subject != in.Subject || out.Body != in.Body || out.From != in.From ||
		out.MessageId != in.MessageId || out.OrganizationId != in.OrganizationId {
		t.Errorf("got %+v, want %+v", out, in)
	}
}
```

(Comparing field-by-field rather than `out != in` because `time.Time` zero-value round-tripping through Avro's `timestamp-millis` logical type can differ in monotonic-clock reading even when semantically equal — this avoids a flaky false failure unrelated to what the test is actually checking.)

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./libs/eventschemas/... -v`
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 4: Run `avrogen` to generate the struct**

```bash
cd libs/eventschemas
avrogen -pkg eventschemas -o email_created_gen.go -tags json:camel schemas/email_created.avsc
```

Expected output, `libs/eventschemas/email_created_gen.go` (verify the actual `avrogen` output matches this shape; if the tool's real output differs — e.g. no `avro.Schema` var, different tag casing — keep the tool's real output, it is authoritative over this text):

```go
// Code generated by avrogen. DO NOT EDIT.

package eventschemas

import (
	"time"
)

type EmailCreated struct {
	Subject        string    `avro:"subject" json:"subject"`
	Body           string    `avro:"body" json:"body"`
	From           string    `avro:"from" json:"from"`
	Date           time.Time `avro:"date" json:"date"`
	MessageId      string    `avro:"messageId" json:"messageId"`
	OrganizationId int32     `avro:"organizationId" json:"organizationId"`
}
```

If `avrogen` isn't available or its output differs meaningfully in a way that breaks the rest of this plan (e.g. it doesn't support `-tags`), write this exact struct by hand as a stopgap, and file it as a known deviation from the spec's codegen assumption (Section 3's caveat already flags this as something to verify).

- [ ] **Step 5: Write `eventschemas.go`**

```go
// libs/eventschemas/eventschemas.go
package eventschemas

import (
	_ "embed"

	avro "github.com/iskorotkov/avro/v2"
)

const (
	EmailReceivedTopic   = "email.received"
	EmailReceivedSubject = "email.received-value"
)

//go:embed schemas/email_created.avsc
var emailCreatedSchemaRaw string

// EmailCreatedSchema is the parsed form of schemas/email_created.avsc — the
// single source of truth this type and its registered version are both
// derived from.
var EmailCreatedSchema = avro.MustParse(emailCreatedSchemaRaw)

// Schemas returns every schema this package owns, keyed by registry subject
// name. cmd/registerschemas (Task 4) walks this generically — it never
// hardcodes which event types exist.
func Schemas() map[string]avro.Schema {
	return map[string]avro.Schema{
		EmailReceivedSubject: EmailCreatedSchema,
	}
}
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./libs/eventschemas/... -v`
Expected: PASS for both `TestSchemas_includesEmailCreated` and `TestEmailCreated_avroRoundTrip` (no further test changes needed — Step 1's test already used the real `avro.Marshal`/`avro.Unmarshal` calls, it was just waiting on `eventschemas.go` and the generated struct to exist).

- [ ] **Step 7: Commit**

```bash
git add libs/eventschemas
git commit -m "Add libs/eventschemas: email.received Avro schema and generated type"
```

---

## Task 4: Schema registration CLI + Task target

**Files:**
- Create: `libs/eventschemas/cmd/registerschemas/main.go`
- Modify: `Taskfile.yml`

**Interfaces:**
- Consumes: `eventschemas.Schemas()`, `schemaregistry.NewClient` (Tasks 2, 3).
- Produces: `task register:schemas` — Task 6 (docker-compose) and later manual verification depend on this being runnable.

- [ ] **Step 1: Write `main.go`**

```go
// libs/eventschemas/cmd/registerschemas/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"planeo/libs/eventschemas"
	"planeo/libs/schemaregistry"
)

func main() {
	registryURL := os.Getenv("SCHEMA_REGISTRY_URL")
	if registryURL == "" {
		fmt.Fprintln(os.Stderr, "SCHEMA_REGISTRY_URL is required")
		os.Exit(1)
	}

	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: registryURL})
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating schema registry client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	for subject, schema := range eventschemas.Schemas() {
		id, err := schemaregistry.Register(ctx, client, subject, schema)
		if err != nil {
			fmt.Fprintf(os.Stderr, "registering subject %q: %v\n", subject, err)
			os.Exit(1)
		}
		fmt.Printf("registered subject %q as schema id %d\n", subject, id)
	}
}
```

This uses `schemaregistry.Register` (Task 2's `client.go`) directly — it's already the right shape for this: a one-shot, content-based lookup-or-register call with no concrete Go type involved, since this CLI walks `eventschemas.Schemas()`'s `map[string]avro.Schema` generically. Note this file never imports `github.com/twmb/franz-go/pkg/sr` — `schemaregistry.Config{URL: ...}` is enough for the common case, same as `libs/kafka.Config` elsewhere in this codebase.

- [ ] **Step 2: Verify it builds**

Run: `go build ./libs/eventschemas/cmd/registerschemas/...`
Expected: exits 0.

- [ ] **Step 3: Add the `register:schemas` Task target**

In `Taskfile.yml`, add near the `migrate:*` targets (after `migrate:email:status`, matching that section's style):

```yaml
  register:schemas:
    desc: Register all event schemas with the local schema registry
    env:
      SCHEMA_REGISTRY_URL: http://localhost:8083/apis/ccompat/v7
    cmds:
      - echo "Registering event schemas..."
      - go run ./libs/eventschemas/cmd/registerschemas
```

- [ ] **Step 4: Wire it into `task up`**

Modify the existing `up` task in `Taskfile.yml`:

```yaml
  up:
    desc: Start Docker services and run DB migrations
    cmds:
      - echo "Starting up the dev environment..."
      - cd dev && ./start.sh
      - task: migrate:core
      - task: migrate:email
      - task: register:schemas
```

- [ ] **Step 5: Commit**

```bash
git add libs/eventschemas/cmd Taskfile.yml
git commit -m "Add schema registration CLI and task register:schemas"
```

---

## Task 5: Apicurio Registry in local dev infra

**Files:**
- Modify: `dev/docker-compose.yaml`

- [ ] **Step 1: Add the `schema-registry` service**

In `dev/docker-compose.yaml`, add after the `kafka-ui` service block (before the `email-received-producer` comment/block), matching existing indentation:

```yaml
  # schema registry for Avro-encoded Kafka messages (Confluent-compatible
  # API at /apis/ccompat/v7). In-memory storage: task down already wipes
  # all volumes, so persistence here buys nothing for the normal dev reset
  # workflow. See docs/superpowers/specs/2026-07-25-kafka-schema-registry-avro-design.md
  # Section 1 for the known limitation this implies for compatibility checking.
  schema-registry:
    image: apicurio/apicurio-registry:3.3.0
    container_name: schema-registry
    profiles:
      - kafka-clients
    ports:
      - "8083:8080"
```

- [ ] **Step 2: Add `SCHEMA_REGISTRY_URL` to both sidecar services' environment**

In the same file, add one line to each of `email-received-producer`'s and `email-received-consumer`'s `environment:` blocks:

```yaml
      SCHEMA_REGISTRY_URL: http://schema-registry:8080/apis/ccompat/v7
```

(Container-to-container URL, not `localhost:8083` — these two services reach `schema-registry` over the compose network.)

- [ ] **Step 3: Verify the compose file is valid**

Run: `cd dev && docker compose --profile kafka-clients config -q`
Expected: exits 0 with no output (silent success means valid YAML + valid compose schema).

- [ ] **Step 4: Commit**

```bash
git add dev/docker-compose.yaml
git commit -m "Add local Apicurio schema registry to docker-compose"
```

---

## Task 6: Outbox `payload` column: `BYTEA` → `JSONB`

**Files:**
- Create: `services/email/internal/infra/postgres/migrations/20260727120000_outbox_payload_jsonb.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE outbox ALTER COLUMN payload TYPE JSONB USING convert_from(payload, 'UTF8')::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE outbox ALTER COLUMN payload TYPE BYTEA USING convert_to(payload::text, 'UTF8');
-- +goose StatementEnd
```

- [ ] **Step 2: Apply it locally and verify**

Run:
```bash
task migrate:email
```
Expected: goose reports the new migration applied, no errors.

Verify the column type:
```bash
docker exec -it postgres psql -U planeo -d mail -c "\d outbox" | grep payload
```
Expected: `payload | jsonb`.

- [ ] **Step 3: Verify rollback works**

Run: `task migrate:email:down` then `task migrate:email` again (down then back up), confirming both succeed with no errors.

- [ ] **Step 4: Commit**

```bash
git add services/email/internal/infra/postgres/migrations/20260727120000_outbox_payload_jsonb.sql
git commit -m "Change outbox.payload column from BYTEA to JSONB"
```

---

## Task 7: `domain/mail` — remove wire-format knowledge

**Files:**
- Modify: `services/email/internal/domain/mail/model.go`
- Modify: `services/email/internal/domain/mail/service.go`
- Modify: `services/email/internal/domain/mail/service_test.go`

**Interfaces:**
- Produces: `mail.EmailReceivedFields`, updated `mail.OutboxEvent{Key []byte, Fields EmailReceivedFields}` — Task 8 (the repository) consumes this new shape.

- [ ] **Step 1: Update the failing test first — assert on `Fields`, not just the type name**

The current test at `services/email/internal/domain/mail/service_test.go:41` only checks `mock.AnythingOfType("OutboxEvent")`, which doesn't verify field mapping at all. Tighten it with `mock.MatchedBy` so the test actually proves `buildMailAndEvent`'s mapping is correct:

```go
// services/email/internal/domain/mail/service_test.go — replace the
// "creates the mail and outbox event" subtest's CreateOutboxEvent .EXPECT()
mockRepository.EXPECT().
	CreateOutboxEvent(context.Background(), 1, mock.MatchedBy(func(event OutboxEvent) bool {
		return event.Fields.Subject == "Test" &&
			event.Fields.Body == "body" &&
			event.Fields.From == "sender@example.com" &&
			event.Fields.MessageID == "abc123" &&
			event.Fields.OrganizationID == 1 &&
			string(event.Key) == "1"
	})).
	Return(nil)
```

(Leave every other subtest in this file unchanged — they don't inspect `OutboxEvent`'s contents.)

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./services/email/internal/domain/mail/... -v -short`
Expected: FAIL to compile — `OutboxEvent` has no field `Fields` yet (current struct has `Topic`/`Key`/`Payload`).

- [ ] **Step 3: Update `model.go`**

```go
// services/email/internal/domain/mail/model.go — replace OutboxEvent, add EmailReceivedFields
// EmailReceivedFields is the plain domain shape of an email.received event —
// no wire-format knowledge (no avro/json tags, no schema import). The
// mapping into the wire type lives in the infra repository (see
// infra/postgres/outbox_repository.go), not here.
type EmailReceivedFields struct {
	Subject        string
	Body           string
	From           string
	Date           time.Time
	MessageID      string
	OrganizationID int
}

// OutboxEvent is the Kafka event to be durably queued alongside a NewMail,
// in the same local transaction. Topic is not carried here — the infra
// repository sets it from eventschemas.EmailReceivedTopic, since that
// constant lives in a wire-format package this domain layer must not import.
type OutboxEvent struct {
	Key    []byte
	Fields EmailReceivedFields
}
```

- [ ] **Step 4: Update `service.go`**

```go
// services/email/internal/domain/mail/service.go
package mail

import (
	"context"
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
			newMail, event := buildMailAndEvent(raw)

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

func buildMailAndEvent(raw RawFetchedMail) (NewMail, OutboxEvent) {
	newMail := NewMail{
		MessageID:      raw.MessageID,
		SettingID:      raw.SettingID,
		OrganizationID: raw.OrganizationID,
		Subject:        raw.Subject,
		Sender:         raw.Sender,
		Body:           raw.Body,
		Date:           raw.Date,
	}

	event := OutboxEvent{
		Key: []byte(strconv.Itoa(raw.OrganizationID)),
		Fields: EmailReceivedFields{
			Subject:        raw.Subject,
			Body:           raw.Body,
			From:           raw.Sender,
			Date:           raw.Date,
			MessageID:      raw.MessageID,
			OrganizationID: raw.OrganizationID,
		},
	}

	return newMail, event
}
```

`buildMailAndEvent` no longer returns an `error` — there's no marshaling left in it that could fail, only struct-field copies, so the old error-plumbing was dead weight once the `json.Marshal` call is gone. `SaveFetchedMails`'s call site above already reflects the two-value return.

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./services/email/internal/domain/mail/... -v -short`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/email/internal/domain/mail
git commit -m "Remove wire-format knowledge from domain/mail"
```

---

## Task 8: `infra/postgres` outbox repository — map to the wire type

**Files:**
- Modify: `services/email/internal/infra/postgres/outbox_repository.go`
- Modify: `services/email/internal/test/mail/mail_test.go` (integration test — adjust to the file's actual current assertions when touching it; the exact current content wasn't re-verified during planning, so confirm what it asserts today before changing it)

**Interfaces:**
- Consumes: `mail.OutboxEvent{Key, Fields}` (Task 7), `eventschemas.EmailCreated`, `eventschemas.EmailReceivedTopic` (Task 3).

- [ ] **Step 1: Update `CreateOutboxEvent`**

```go
// services/email/internal/infra/postgres/outbox_repository.go
package postgres

import (
	"context"
	"encoding/json"
	"planeo/libs/db"
	"planeo/libs/eventschemas"
	"planeo/libs/outbox"
	"planeo/services/email/internal/domain/mail"
	"time"

	"github.com/jackc/pgx/v5"
)

// ... FetchBatch, MarkProcessed, MarkFailed unchanged (see below, no edits needed) ...

func (c *Client) CreateOutboxEvent(ctx context.Context, mailID int, event mail.OutboxEvent) error {
	wire := eventschemas.EmailCreated{
		Subject:        event.Fields.Subject,
		Body:           event.Fields.Body,
		From:           event.Fields.From,
		Date:           event.Fields.Date,
		MessageId:      event.Fields.MessageID,
		OrganizationId: int32(event.Fields.OrganizationID),
	}
	payload, err := json.Marshal(wire)
	if err != nil {
		return NewDatabaseError("error marshaling outbox payload", err)
	}

	query := `
		INSERT INTO outbox (mail_id, topic, key, payload)
		VALUES (@mailId, @topic, @key, @payload)`
	args := pgx.NamedArgs{
		"mailId":  mailID,
		"topic":   eventschemas.EmailReceivedTopic,
		"key":     event.Key,
		"payload": payload,
	}

	q := db.FromContext(ctx, c.db)
	if _, err := q.Exec(ctx, query, args); err != nil {
		return NewDatabaseError("error inserting outbox event", err)
	}
	return nil
}
```

Only `CreateOutboxEvent` changes; `FetchBatch`, `MarkProcessed`, `MarkFailed` in this same file are untouched — they operate on `outbox.Record`/opaque bytes and never construct or read the wire type.

- [ ] **Step 2: Read the current integration test before editing it**

Read `services/email/internal/test/mail/mail_test.go` in full first (its exact current content was not re-verified during plan-writing — the design spec's Testing Implications section describes the intent: assert the outbox row's `payload` is valid JSON matching `eventschemas.EmailCreated`'s shape). Update whichever assertion currently checks the outbox row's payload bytes to instead `json.Unmarshal` the row into `eventschemas.EmailCreated` and assert on its fields, matching this task's new `CreateOutboxEvent` behavior. Keep every other assertion in that file (dedup behavior, transaction atomicity) unchanged.

- [ ] **Step 3: Run the integration test**

Run: `task test:email:integration`
Expected: PASS (requires Docker for the Postgres testcontainer).

- [ ] **Step 4: Commit**

```bash
git add services/email/internal/infra/postgres/outbox_repository.go services/email/internal/test/mail/mail_test.go
git commit -m "Map EmailReceivedFields to the eventschemas wire type in CreateOutboxEvent"
```

---

## Task 9: `infra/events.EmailCreatedProducer` — Avro-encode at produce time

**Files:**
- Create: `services/email/internal/infra/events/email_created_producer.go`
- Modify: `services/email/cmd/email-received-producer/config.go`
- Modify: `services/email/cmd/email-received-producer/main.go`
- Test: `services/email/internal/infra/events/email_created_producer_test.go`

**Interfaces:**
- Consumes: `outbox.Producer` (the port `domain/outbox` already defines), `schemaregistry.Encode`/`*schemaregistry.Serde` (Task 2), `eventschemas.EmailCreated` (Task 3).
- Produces: `events.NewEmailCreatedProducer(inner, serde) *EmailCreatedProducer`, satisfying `domain/outbox.Producer` — wired into `outbox.NewService(...)` in place of the raw `outbox.NewProducer(...)` result. This constructor is plain (no `ctx`, no error) — the `*schemaregistry.Serde` it receives is already registered by `cmd/main.go` before construction (Step 6). This package never imports `github.com/twmb/franz-go/pkg/sr`.

- [ ] **Step 1: Write the failing unit test**

```go
// services/email/internal/infra/events/email_created_producer_test.go
package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"planeo/libs/eventschemas"
	"planeo/libs/schemaregistry"
	"planeo/services/email/internal/infra/events"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type fakeProducer struct {
	gotTopic       string
	gotKey, gotVal []byte
	err            error
}

func (f *fakeProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	f.gotTopic, f.gotKey, f.gotVal = topic, key, value
	return f.err
}

func startRegistry(t *testing.T) *schemaregistry.Serde {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "apicurio/apicurio-registry:3.3.0",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/apis/ccompat/v7/subjects").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start apicurio container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Errorf("failed to terminate apicurio container: %v", err)
		}
	})
	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "8080")
	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: "http://" + host + ":" + port.Port() + "/apis/ccompat/v7"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[eventschemas.EmailCreated](ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailCreatedSchema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}
	return serde
}

func TestEmailCreatedProducer_encodesAndForwards(t *testing.T) {
	serde := startRegistry(t)
	fake := &fakeProducer{}
	producer := events.NewEmailCreatedProducer(fake, serde)

	wire := eventschemas.EmailCreated{
		Subject: "hi", Body: "body", From: "a@b.com",
		MessageId: "msg-1", OrganizationId: 1,
	}
	jsonPayload, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	err = producer.ProduceSync(context.Background(), "email.received", []byte("1"), jsonPayload)
	if err != nil {
		t.Fatalf("ProduceSync: %v", err)
	}

	if fake.gotTopic != "email.received" {
		t.Errorf("got topic %q, want %q", fake.gotTopic, "email.received")
	}
	if string(fake.gotKey) != "1" {
		t.Errorf("got key %q, want %q", fake.gotKey, "1")
	}

	// Decode what was actually passed to the inner producer using the same
	// serde, proving the bytes are genuinely Avro-encoded, not just passed
	// through unchanged.
	var decoded eventschemas.EmailCreated
	if err := schemaregistry.Decode(context.Background(), serde, fake.gotVal, eventschemas.EmailCreatedSchema, &decoded); err != nil {
		t.Fatalf("decoding what was passed to inner producer: %v", err)
	}
	if decoded.Subject != "hi" || decoded.OrganizationId != 1 {
		t.Errorf("got %+v, want Subject=hi OrganizationId=1", decoded)
	}
}

func TestEmailCreatedProducer_badJSONPayload(t *testing.T) {
	serde := startRegistry(t)
	fake := &fakeProducer{}
	producer := events.NewEmailCreatedProducer(fake, serde)

	err := producer.ProduceSync(context.Background(), "email.received", []byte("1"), []byte("not-json"))
	if err == nil {
		t.Fatal("expected an error for malformed JSON payload, got nil")
	}
	var target *json.SyntaxError
	if !errors.As(err, &target) {
		t.Errorf("expected error to wrap a json.SyntaxError, got %v", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./services/email/internal/infra/events/... -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Write the implementation**

```go
// services/email/internal/infra/events/email_created_producer.go
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"planeo/libs/eventschemas"
	"planeo/libs/schemaregistry"
	domainoutbox "planeo/services/email/internal/domain/outbox"
)

// EmailCreatedProducer wraps the real Kafka producer so the JSON-staged
// outbox payload gets Avro-encoded right before it reaches Kafka — this is
// the coupling site described in the design spec's Section 3:
// domain/outbox.Service.ProcessRecord stays completely unaware this
// conversion happens. serde is already registered for EmailCreated by the
// time it reaches here (see cmd/main.go, Step 6) — this package never
// imports franz-go/pkg/sr, and this constructor is plain, matching every
// other infra adapter constructor in this codebase.
type EmailCreatedProducer struct {
	inner domainoutbox.Producer
	serde *schemaregistry.Serde
}

func NewEmailCreatedProducer(inner domainoutbox.Producer, serde *schemaregistry.Serde) *EmailCreatedProducer {
	return &EmailCreatedProducer{inner: inner, serde: serde}
}

func (p *EmailCreatedProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	var wire eventschemas.EmailCreated
	if err := json.Unmarshal(value, &wire); err != nil {
		return fmt.Errorf("decode staged outbox payload: %w", err)
	}

	avroBytes, err := schemaregistry.Encode(p.serde, wire)
	if err != nil {
		return fmt.Errorf("avro encode: %w", err)
	}

	return p.inner.ProduceSync(ctx, topic, key, avroBytes)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./services/email/internal/infra/events/... -v`
Expected: PASS for both tests.

- [ ] **Step 5: Add `SCHEMA_REGISTRY_URL` to the sidecar's config**

```go
// services/email/cmd/email-received-producer/config.go — add one field and one load line
type Config struct {
	DbHost             string
	DbPort             string
	DbUser             string
	DbPassword         string
	DbName             string
	KafkaBrokers       string
	KafkaSaslUsername  string
	KafkaSaslPassword  string
	KafkaSaslMechanism string
	KafkaTlsEnabled    bool
	SchemaRegistryURL  string
	PollInterval       time.Duration
	BatchSize          int
	MaxAttempts        int
	ClaimTTL           time.Duration
}
```

Add to `LoadConfig`'s returned struct literal:
```go
		SchemaRegistryURL:  readEnvVariable(ctx, "SCHEMA_REGISTRY_URL"),
```

- [ ] **Step 6: Wire it into `main.go`**

```go
// services/email/cmd/email-received-producer/main.go
package main

import (
	"context"
	"os/signal"
	"planeo/libs/eventschemas"
	"planeo/libs/kafka"
	"planeo/libs/logger"
	"planeo/libs/outbox"
	"planeo/libs/schemaregistry"
	domainoutbox "planeo/services/email/internal/domain/outbox"
	"planeo/services/email/internal/infra/events"
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
	kafkaClient, err := kafka.NewClient(kafka.Config{
		Brokers:    brokers,
		Username:   cfg.KafkaSaslUsername,
		Password:   cfg.KafkaSaslPassword,
		Mechanism:  cfg.KafkaSaslMechanism,
		TLSEnabled: cfg.KafkaTlsEnabled,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}
	defer kafkaClient.Close()

	registryClient, err := schemaregistry.NewClient(schemaregistry.Config{URL: cfg.SchemaRegistryURL})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create schema registry client")
	}
	serde := schemaregistry.NewSerde(registryClient)
	if err := schemaregistry.RegisterEncode[eventschemas.EmailCreated](
		ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailCreatedSchema,
	); err != nil {
		log.Fatal().Err(err).Msg("Failed to register EmailCreated for encoding")
	}

	rawProducer := outbox.NewProducer(kafkaClient)
	producer := events.NewEmailCreatedProducer(rawProducer, serde)

	outboxService := domainoutbox.NewService(db, producer)
	emailReceivedProducer := emailoutbox.NewEmailReceivedProducer(
		outboxService, eventschemas.EmailReceivedTopic, uuid.NewString(),
		cfg.BatchSize, cfg.MaxAttempts, cfg.ClaimTTL,
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Email-received producer running")
	runner := outbox.NewRunner(outbox.WithPollInterval(cfg.PollInterval))
	if err := runner.Run(runCtx, emailReceivedProducer.PollOnce); err != nil {
		log.Info().Err(err).Msg("Email-received producer stopped")
	}
}
```

(Note: `contracts.EmailReceivedTopic` becomes `eventschemas.EmailReceivedTopic` — the `contracts` import is dropped from this file. This file no longer imports `github.com/twmb/franz-go/pkg/sr` either — `schemaregistry.Config` covers the common case.)

- [ ] **Step 7: Verify it builds**

Run: `go build ./services/email/...`
Expected: exits 0.

- [ ] **Step 8: Commit**

```bash
git add services/email/internal/infra/events services/email/cmd/email-received-producer
git commit -m "Add EmailCreatedProducer: Avro-encode at produce time, wired into the sidecar"
```

---

## Task 10: `domain/inbox` — remove wire-format knowledge, add `MarkFailed`

**Files:**
- Modify: `services/core/internal/domain/inbox/model.go`
- Modify: `services/core/internal/domain/inbox/ports.go`
- Modify: `services/core/internal/domain/inbox/service.go`
- Modify: `services/core/internal/domain/inbox/service_test.go`

**Interfaces:**
- Produces: `inbox.EmailReceivedEvent`, `inbox.Service.ProcessEmailReceived(ctx, recordID int64, event EmailReceivedEvent, maxAttempts int) error`, `inbox.Service.MarkFailed(ctx, id int64, procErr error, maxAttempts int) error` — Task 11 (the consumer adapter) depends on these exact names/signatures.

- [ ] **Step 1: Rewrite `service_test.go` first**

Replace the whole `ProcessEmailReceived` subtest block. Every test case drops `libsinbox.Record`/`validPayload`/JSON entirely and calls the new signature directly; the "marks the record failed when the payload is not valid JSON" case is removed outright (that failure mode no longer exists at this layer — it's covered by Task 11's adapter test instead), and a new case proves `MarkFailed` delegates correctly:

```go
// services/core/internal/domain/inbox/service_test.go
package inbox_test

import (
	"context"
	"errors"
	"planeo/services/core/internal/domain/category"
	categorymocks "planeo/services/core/internal/domain/category/mocks"
	. "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/domain/inbox/mocks"
	"planeo/services/core/internal/domain/request"
	requestmocks "planeo/services/core/internal/domain/request/mocks"
	"testing"
	"time"

	libsinbox "planeo/libs/inbox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const wantRaw = "Subject: Need a permit\nFrom: jane@example.com\nDate: Thu, 01 Jan 2026 00:00:00 UTC\nMessage-ID: msg-1\nBody: Please help"

func categoryIDPtr(id int) *int { return &id }

func validEvent() EmailReceivedEvent {
	return EmailReceivedEvent{
		Subject:        "Need a permit",
		Body:           "Please help",
		From:           "jane@example.com",
		Date:           time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MessageID:      "msg-1",
		OrganizationID: 1,
	}
}

func TestService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("FetchBatch delegates to the repository", func(t *testing.T) {
		mockRepository := mocks.NewMockRepository(t)
		record := libsinbox.Record{ID: 1, Topic: "email-received", Payload: []byte("{}")}
		mockRepository.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return([]libsinbox.Record{record}, nil)

		service := NewService(mockRepository, requestmocks.NewMockService(t), categorymocks.NewMockService(t), mocks.NewMockLLMClient(t))

		records, err := service.FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second)

		assert.Nil(t, err)
		assert.Equal(t, []libsinbox.Record{record}, records)
	})

	t.Run("MarkFailed delegates to the repository", func(t *testing.T) {
		mockRepository := mocks.NewMockRepository(t)
		procErr := errors.New("decode failed")
		mockRepository.EXPECT().MarkFailed(context.Background(), int64(9), procErr, 5).Return(nil)

		service := NewService(mockRepository, requestmocks.NewMockService(t), categorymocks.NewMockService(t), mocks.NewMockLLMClient(t))

		err := service.MarkFailed(context.Background(), 9, procErr, 5)

		assert.Nil(t, err)
	})

	t.Run("ProcessEmailReceived", func(t *testing.T) {
		t.Run("upserts the request, then marks the record processed", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRequestService := requestmocks.NewMockService(t)
			mockCategoryService := categorymocks.NewMockService(t)
			mockLLMClient := mocks.NewMockLLMClient(t)

			categories := []category.Category{{Id: 5, Label: "Permits"}}

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(categories, nil)
			mockLLMClient.EXPECT().
				ExtractRequestFields(context.Background(), mock.AnythingOfType("string")).
				Return(ExtractorOutput{Name: "Jane", Address: "Main St", Phone: "123"}, nil)
			mockLLMClient.EXPECT().
				ClassifyRequest(context.Background(), RequestData{Subject: "Need a permit", Text: "Please help"}, categories).
				Return(5, nil)

			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRequestService.EXPECT().
				UpsertRequest(context.Background(), request.Request{
					Text:           "Please help",
					Subject:        "Need a permit",
					Email:          "jane@example.com",
					Name:           "Jane",
					Address:        "Main St",
					Telephone:      "123",
					Raw:            wantRaw,
					ReferenceId:    "msg-1",
					CategoryId:     categoryIDPtr(5),
					OrganizationId: 1,
				}).
				Return(42, nil)
			mockRepository.EXPECT().MarkProcessed(context.Background(), int64(1)).Return(nil)

			service := NewService(mockRepository, mockRequestService, mockCategoryService, mockLLMClient)

			err := service.ProcessEmailReceived(context.Background(), 1, validEvent(), 5)

			assert.Nil(t, err)
		})

		t.Run("marks the record failed when fetching categories errors", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockCategoryService := categorymocks.NewMockService(t)
			categoryErr := errors.New("category lookup failed")

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(nil, categoryErr)
			mockRepository.EXPECT().MarkFailed(context.Background(), int64(3), categoryErr, 5).Return(nil)

			service := NewService(mockRepository, requestmocks.NewMockService(t), mockCategoryService, mocks.NewMockLLMClient(t))

			err := service.ProcessEmailReceived(context.Background(), 3, validEvent(), 5)

			assert.Nil(t, err)
		})

		t.Run("proceeds to classification and request creation when field extraction fails", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRequestService := requestmocks.NewMockService(t)
			mockCategoryService := categorymocks.NewMockService(t)
			mockLLMClient := mocks.NewMockLLMClient(t)

			categories := []category.Category{{Id: 5, Label: "Permits"}}

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(categories, nil)
			mockLLMClient.EXPECT().
				ExtractRequestFields(context.Background(), mock.AnythingOfType("string")).
				Return(ExtractorOutput{}, errors.New("extraction unavailable"))
			mockLLMClient.EXPECT().
				ClassifyRequest(context.Background(), RequestData{Subject: "Need a permit", Text: "Please help"}, categories).
				Return(5, nil)

			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRequestService.EXPECT().
				UpsertRequest(context.Background(), mock.AnythingOfType("request.Request")).
				Return(42, nil)
			mockRepository.EXPECT().MarkProcessed(context.Background(), int64(4)).Return(nil)

			service := NewService(mockRepository, mockRequestService, mockCategoryService, mockLLMClient)

			err := service.ProcessEmailReceived(context.Background(), 4, validEvent(), 5)

			assert.Nil(t, err)
		})

		t.Run("marks the record failed when classification errors", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockCategoryService := categorymocks.NewMockService(t)
			mockLLMClient := mocks.NewMockLLMClient(t)

			categories := []category.Category{{Id: 5, Label: "Permits"}}
			classifyErr := errors.New("classification failed")

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(categories, nil)
			mockLLMClient.EXPECT().
				ExtractRequestFields(context.Background(), mock.AnythingOfType("string")).
				Return(ExtractorOutput{}, nil)
			mockLLMClient.EXPECT().
				ClassifyRequest(context.Background(), RequestData{Subject: "Need a permit", Text: "Please help"}, categories).
				Return(0, classifyErr)
			mockRepository.EXPECT().MarkFailed(context.Background(), int64(5), classifyErr, 5).Return(nil)

			service := NewService(mockRepository, requestmocks.NewMockService(t), mockCategoryService, mockLLMClient)

			err := service.ProcessEmailReceived(context.Background(), 5, validEvent(), 5)

			assert.Nil(t, err)
		})

		t.Run("marks the record failed outside the transaction when the write transaction errors", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRequestService := requestmocks.NewMockService(t)
			mockCategoryService := categorymocks.NewMockService(t)
			mockLLMClient := mocks.NewMockLLMClient(t)

			categories := []category.Category{{Id: 5, Label: "Permits"}}
			txErr := errors.New("write failed")

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(categories, nil)
			mockLLMClient.EXPECT().
				ExtractRequestFields(context.Background(), mock.AnythingOfType("string")).
				Return(ExtractorOutput{}, nil)
			mockLLMClient.EXPECT().
				ClassifyRequest(context.Background(), RequestData{Subject: "Need a permit", Text: "Please help"}, categories).
				Return(5, nil)

			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRequestService.EXPECT().
				UpsertRequest(context.Background(), mock.AnythingOfType("request.Request")).
				Return(0, txErr)
			mockRepository.EXPECT().MarkFailed(context.Background(), int64(6), txErr, 5).Return(nil)

			service := NewService(mockRepository, mockRequestService, mockCategoryService, mockLLMClient)

			err := service.ProcessEmailReceived(context.Background(), 6, validEvent(), 5)

			assert.Nil(t, err)
		})
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./services/core/internal/domain/inbox/... -v -short`
Expected: FAIL to compile — `EmailReceivedEvent`, `Service.MarkFailed`, and the new `ProcessEmailReceived` signature don't exist yet.

- [ ] **Step 3: Add `EmailReceivedEvent` to `model.go`**

```go
// services/core/internal/domain/inbox/model.go — add this type
// (alongside whatever ExtractorOutput/RequestData already live there)
type EmailReceivedEvent struct {
	Subject        string
	Body           string
	From           string
	Date           time.Time
	MessageID      string
	OrganizationID int
}
```

- [ ] **Step 4: Update `ports.go`**

```go
// services/core/internal/domain/inbox/ports.go
package inbox

import (
	"context"
	"time"

	libsinbox "planeo/libs/inbox"
	"planeo/services/core/internal/domain/category"
)

type Repository interface {
	Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error)
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error)
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error
}

type LLMClient interface {
	ExtractRequestFields(ctx context.Context, requestText string) (ExtractorOutput, error)
	ClassifyRequest(ctx context.Context, request RequestData, categories []category.Category) (int, error)
}

type Service interface {
	Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error)
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error)
	ProcessEmailReceived(ctx context.Context, recordID int64, event EmailReceivedEvent, maxAttempts int) error
	MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error
}
```

(`Repository` is unchanged from today — `MarkFailed` already existed there. Only `Service` gains the method.)

- [ ] **Step 5: Update `service.go`**

```go
// services/core/internal/domain/inbox/service.go
package inbox

import (
	"context"
	"fmt"
	"time"

	libsinbox "planeo/libs/inbox"
	"planeo/libs/logger"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
)

type service struct {
	repo            Repository
	requestService  request.Service
	categoryService category.Service
	llmClient       LLMClient
}

func NewService(
	repo Repository,
	requestService request.Service,
	categoryService category.Service,
	llmClient LLMClient,
) Service {
	return &service{
		repo:            repo,
		requestService:  requestService,
		categoryService: categoryService,
		llmClient:       llmClient,
	}
}

func (s *service) Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error) {
	return s.repo.Save(ctx, topic, partition, offset, payload)
}

func (s *service) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error) {
	return s.repo.FetchBatch(ctx, topic, instanceID, limit, claimTTL)
}

func (s *service) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	return s.repo.MarkFailed(ctx, id, procErr, maxAttempts)
}

// ProcessEmailReceived gathers everything the write phase needs (categories,
// LLM extraction, LLM classification) BEFORE opening any transaction — these
// are slow, network-dependent calls, and holding a Postgres row lock and a
// pooled connection across them would be an unnecessary cost. Only the
// domain writes and the inbox row's final status are wrapped together.
func (s *service) ProcessEmailReceived(ctx context.Context, recordID int64, event EmailReceivedEvent, maxAttempts int) error {
	raw := fmt.Sprintf("Subject: %s\nFrom: %s\nDate: %s\nMessage-ID: %s\nBody: %s",
		event.Subject, event.From, event.Date.Format(time.RFC1123), event.MessageID, event.Body)

	categories, err := s.categoryService.GetCategories(ctx, event.OrganizationID)
	if err != nil {
		return s.repo.MarkFailed(ctx, recordID, err, maxAttempts)
	}

	extractedFields, err := s.llmClient.ExtractRequestFields(ctx, raw)
	if err != nil {
		log := logger.FromContext(ctx)
		log.Error().Err(err).Msg("failed to extract fields from request")
		// not fatal - a missing extraction still allows classification and request creation to proceed
	}

	requestData := RequestData{Subject: event.Subject, Text: event.Body}
	categoryId, err := s.llmClient.ClassifyRequest(ctx, requestData, categories)
	if err != nil {
		return s.repo.MarkFailed(ctx, recordID, err, maxAttempts)
	}

	// Everything the request row needs (raw fields plus the LLM-derived
	// ones) is already known at this point, so a single UpsertRequest
	// replaces what would otherwise be a CreateRequest-then-UpdateRequest
	// two-step — that two-step exists for the REST-facing create path,
	// which doesn't have the LLM-derived fields yet, not for this one.
	var categoryIdPtr *int
	if categoryId != 0 {
		categoryIdPtr = &categoryId
	}

	// Write-only transaction: UpsertRequest's write and MarkProcessed are
	// atomic together. MarkFailed is deliberately NOT called inside this
	// transaction: once any statement in a Postgres transaction errors, the
	// whole transaction is aborted and every subsequent statement on it
	// fails too (SQLSTATE 25P02) — calling MarkFailed(txCtx, ...) after a
	// write error here would itself fail, rolling back and losing the
	// attempts increment entirely.
	txErr := s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		_, err := s.requestService.UpsertRequest(txCtx, request.Request{
			Subject:        event.Subject,
			Raw:            raw,
			Text:           event.Body,
			Email:          event.From,
			Name:           extractedFields.Name,
			Address:        extractedFields.Address,
			Telephone:      extractedFields.Phone,
			CategoryId:     categoryIdPtr,
			OrganizationId: event.OrganizationID,
			ReferenceId:    event.MessageID,
		})
		if err != nil {
			return err
		}

		return s.repo.MarkProcessed(txCtx, recordID)
	})
	if txErr != nil {
		return s.repo.MarkFailed(ctx, recordID, txErr, maxAttempts)
	}
	return nil
}
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./services/core/internal/domain/inbox/... -v -short`
Expected: PASS.

- [ ] **Step 7: Regenerate mocks**

```bash
cd services/core && mockery
```

Verify `services/core/internal/domain/inbox/mocks/service_mock.go` now has a `MarkFailed` method and the new `ProcessEmailReceived` signature.

- [ ] **Step 8: Commit**

```bash
git add services/core/internal/domain/inbox
git commit -m "Remove wire-format knowledge from domain/inbox, add Service.MarkFailed"
```

---

## Task 11: `infra/inbox.EmailReceivedConsumer` — Avro-decode at process time

**Files:**
- Modify: `services/core/internal/infra/inbox/email_received_consumer.go`
- Modify: `services/core/cmd/email-received-consumer/config.go`
- Modify: `services/core/cmd/email-received-consumer/main.go`
- Test: `services/core/internal/infra/inbox/email_received_consumer_test.go` (new)

**Interfaces:**
- Consumes: `inbox.Service.MarkFailed`, `inbox.Service.ProcessEmailReceived`, `inbox.EmailReceivedEvent` (Task 10); `schemaregistry.Decode`/`*schemaregistry.Serde` (Task 2), `eventschemas.EmailCreated` (Task 3).
- Produces: `inbox.NewEmailReceivedConsumer(service, serde, topic, instanceID, batchSize, maxAttempts, claimTTL) *EmailReceivedConsumer`. This constructor is plain (no `ctx`, no error) — the `*schemaregistry.Serde` it receives is already registered by `cmd/main.go` before construction (Step 6), mirroring the producer side in Task 9. This package never imports `github.com/twmb/franz-go/pkg/sr`.

- [ ] **Step 1: Write the failing test**

This needs a fake `domaininbox.Service` (mockery-generated, from Task 10's regeneration) and a real registry to produce genuinely-encoded bytes to decode:

```go
// services/core/internal/infra/inbox/email_received_consumer_test.go
package inbox_test

import (
	"context"
	"testing"
	"time"

	"planeo/libs/eventschemas"
	libsinbox "planeo/libs/inbox"
	"planeo/libs/schemaregistry"
	domaininbox "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/domain/inbox/mocks"
	"planeo/services/core/internal/infra/inbox"

	"github.com/stretchr/testify/mock"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startRegistry(t *testing.T) *schemaregistry.Serde {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "apicurio/apicurio-registry:3.3.0",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/apis/ccompat/v7/subjects").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start apicurio container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Errorf("failed to terminate apicurio container: %v", err)
		}
	})
	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "8080")
	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: "http://" + host + ":" + port.Port() + "/apis/ccompat/v7"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterDecode[eventschemas.EmailCreated](ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailCreatedSchema); err != nil {
		t.Fatalf("RegisterDecode: %v", err)
	}
	return serde
}

func TestPollOnce_decodesAndProcesses(t *testing.T) {
	serde := startRegistry(t)

	wire := eventschemas.EmailCreated{
		Subject: "hi", Body: "body", From: "a@b.com",
		Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MessageId: "msg-1", OrganizationId: 7,
	}
	encoded, err := schemaregistry.Encode(serde, wire)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	mockService := mocks.NewMockService(t)
	mockService.EXPECT().
		FetchBatch(context.Background(), "email.received", "instance-a", 10, 30*time.Second).
		Return([]libsinbox.Record{{ID: 1, Topic: "email.received", Payload: encoded}}, nil)
	mockService.EXPECT().
		ProcessEmailReceived(context.Background(), int64(1), domaininbox.EmailReceivedEvent{
			Subject: "hi", Body: "body", From: "a@b.com",
			Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			MessageID: "msg-1", OrganizationID: 7,
		}, 5).
		Return(nil)

	consumer := inbox.NewEmailReceivedConsumer(mockService, serde, "email.received", "instance-a", 10, 5, 30*time.Second)

	if err := consumer.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
}

func TestPollOnce_decodeFailureMarksFailedAndContinues(t *testing.T) {
	serde := startRegistry(t)

	mockService := mocks.NewMockService(t)
	mockService.EXPECT().
		FetchBatch(context.Background(), "email.received", "instance-a", 10, 30*time.Second).
		Return([]libsinbox.Record{
			{ID: 1, Topic: "email.received", Payload: []byte("not-a-valid-avro-envelope")},
		}, nil)
	mockService.EXPECT().
		MarkFailed(context.Background(), int64(1), mock.Anything, 5).
		Return(nil)

	consumer := inbox.NewEmailReceivedConsumer(mockService, serde, "email.received", "instance-a", 10, 5, 30*time.Second)

	if err := consumer.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce should not abort the batch on a decode failure, got: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./services/core/internal/infra/inbox/... -v`
Expected: FAIL to compile — `NewEmailReceivedConsumer`'s signature doesn't take a `*schemaregistry.Serde` yet.

- [ ] **Step 3: Update the adapter**

```go
// services/core/internal/infra/inbox/email_received_consumer.go
package inbox

import (
	"context"
	"time"

	"planeo/libs/eventschemas"
	"planeo/libs/logger"
	"planeo/libs/schemaregistry"
	domaininbox "planeo/services/core/internal/domain/inbox"
)

// EmailReceivedConsumer owns the polling loop for the "email-received"
// topic: it fetches a batch of claimed rows, Avro-decodes each payload
// (this is the coupling site described in the design spec's Section 3),
// and hands the decoded event to the domain service. serde is already
// registered for EmailCreated by the time it reaches here (see
// cmd/main.go, Step 6) — this package never imports franz-go/pkg/sr, and
// this constructor is plain, matching every other infra adapter
// constructor in this codebase.
type EmailReceivedConsumer struct {
	service     domaininbox.Service
	serde       *schemaregistry.Serde
	topic       string
	instanceID  string
	batchSize   int
	claimTTL    time.Duration
	maxAttempts int
}

func NewEmailReceivedConsumer(
	service domaininbox.Service,
	serde *schemaregistry.Serde,
	topic string,
	instanceID string,
	batchSize int,
	maxAttempts int,
	claimTTL time.Duration,
) *EmailReceivedConsumer {
	return &EmailReceivedConsumer{
		service:     service,
		serde:       serde,
		topic:       topic,
		instanceID:  instanceID,
		batchSize:   batchSize,
		claimTTL:    claimTTL,
		maxAttempts: maxAttempts,
	}
}

// PollOnce claims a batch of pending inbox rows and processes each in turn.
// One bad record does not stop the rest of the batch.
func (a *EmailReceivedConsumer) PollOnce(ctx context.Context) error {
	records, err := a.service.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	for _, rec := range records {
		log.Info().Str("topic", rec.Topic).Int64("inbox_id", rec.ID).Msg("received inbox record")

		var wire eventschemas.EmailCreated
		if err := schemaregistry.Decode(ctx, a.serde, rec.Payload, eventschemas.EmailCreatedSchema, &wire); err != nil {
			if markErr := a.service.MarkFailed(ctx, rec.ID, err, a.maxAttempts); markErr != nil {
				log.Error().Err(markErr).Int64("inbox_id", rec.ID).Msg("failed to mark inbox record failed after decode error")
			}
			continue
		}

		event := domaininbox.EmailReceivedEvent{
			Subject:        wire.Subject,
			Body:           wire.Body,
			From:           wire.From,
			Date:           wire.Date,
			MessageID:      wire.MessageId,
			OrganizationID: int(wire.OrganizationId),
		}
		if err := a.service.ProcessEmailReceived(ctx, rec.ID, event, a.maxAttempts); err != nil {
			log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to process inbox record")
		}
	}

	return nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./services/core/internal/infra/inbox/... -v`
Expected: PASS for both tests.

- [ ] **Step 5: Add `SCHEMA_REGISTRY_URL` to the sidecar's config**

```go
// services/core/cmd/email-received-consumer/config.go — add one field and one load line
type Config struct {
	DbHost             string
	DbPort             string
	DbUser             string
	DbPassword         string
	DbName             string
	KafkaBrokers       string
	KafkaSaslUsername  string
	KafkaSaslPassword  string
	KafkaSaslMechanism string
	KafkaTlsEnabled    bool
	SchemaRegistryURL  string
	GroupName          string
	PollInterval       time.Duration
	BatchSize          int
	MaxAttempts        int
	ClaimTTL           time.Duration
}
```

Add to `LoadConfig`'s returned struct literal:
```go
		SchemaRegistryURL:  readEnvVariable(ctx, "SCHEMA_REGISTRY_URL"),
```

- [ ] **Step 6: Wire it into `main.go`**

```go
// services/core/cmd/email-received-consumer/main.go
package main

import (
	"context"
	"os/signal"
	"planeo/libs/eventschemas"
	"planeo/libs/inbox"
	"planeo/libs/kafka"
	"planeo/libs/logger"
	"planeo/libs/schemaregistry"
	"planeo/services/core/internal/domain/category"
	domaininbox "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/domain/request"
	coreinbox "planeo/services/core/internal/infra/inbox"
	"planeo/services/core/internal/infra/llm"
	"planeo/services/core/internal/infra/postgres"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
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

	llmClient, err := llm.NewClient()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create LLM client")
	}

	registryClient, err := schemaregistry.NewClient(schemaregistry.Config{URL: cfg.SchemaRegistryURL})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create schema registry client")
	}
	serde := schemaregistry.NewSerde(registryClient)
	if err := schemaregistry.RegisterDecode[eventschemas.EmailCreated](
		ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailCreatedSchema,
	); err != nil {
		log.Fatal().Err(err).Msg("Failed to register EmailCreated for decoding")
	}

	instanceID := uuid.NewString()

	inboxService := domaininbox.NewService(db, requestService, categoryService, llmClient)
	emailReceivedConsumer := coreinbox.NewEmailReceivedConsumer(
		inboxService, serde, eventschemas.EmailReceivedTopic, instanceID,
		cfg.BatchSize, cfg.MaxAttempts, cfg.ClaimTTL,
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	kafkaClient, err := kafka.NewClient(
		kafka.Config{
			Brokers:    brokers,
			Username:   cfg.KafkaSaslUsername,
			Password:   cfg.KafkaSaslPassword,
			Mechanism:  cfg.KafkaSaslMechanism,
			TLSEnabled: cfg.KafkaTlsEnabled,
		},
		kgo.ConsumerGroup(cfg.GroupName),
		kgo.ConsumeTopics(eventschemas.EmailReceivedTopic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}

	consumer := inbox.NewConsumer(kafkaClient, eventschemas.EmailReceivedTopic, inboxService)
	if err := consumer.Run(runCtx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start email-received consumer")
	}

	log.Info().Msg("Email-received consumer running")
	runner := inbox.NewRunner(inbox.WithPollInterval(cfg.PollInterval))
	if err := runner.Run(runCtx, emailReceivedConsumer.PollOnce); err != nil {
		log.Info().Err(err).Msg("Email-received consumer stopped")
	}
}
```

(Note: `contracts.EmailReceivedTopic` becomes `eventschemas.EmailReceivedTopic`, both in the `NewEmailReceivedConsumer` call and the two `libs/inbox`-facing calls — the `contracts` import is dropped from this file. This file no longer imports `github.com/twmb/franz-go/pkg/sr` either — `schemaregistry.Config` covers the common case.)

- [ ] **Step 7: Verify it builds**

Run: `go build ./services/core/...`
Expected: exits 0.

- [ ] **Step 8: Commit**

```bash
git add services/core/internal/infra/inbox services/core/cmd/email-received-consumer
git commit -m "Add Avro-decode to EmailReceivedConsumer, wired into the sidecar"
```

---

## Task 12: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run every unit test suite**

```bash
task test:core:unit
task test:email:unit
task test:libs:unit
```
Expected: all PASS.

- [ ] **Step 2: Run every integration test suite (requires Docker)**

```bash
task test:core:integration
task test:email:integration
go test ./libs/schemaregistry/... ./libs/eventschemas/... -v
```
Expected: all PASS.

- [ ] **Step 3: Full build**

```bash
go build ./...
```
Expected: exits 0.

- [ ] **Step 4: End-to-end manual smoke test**

```bash
task down
task up
docker compose -f dev/docker-compose.yaml --profile kafka-clients up -d schema-registry email-received-producer email-received-consumer
curl -s http://localhost:8083/apis/ccompat/v7/subjects
```
Expected: the curl output includes `"email.received-value"`, proving `task register:schemas` (wired into `task up` in Task 4) ran successfully against the freshly-started registry.

- [ ] **Step 5: Commit any final fixups discovered during verification**

If any of the above steps required a fix, commit it with a message describing what verification step caught it.

---

## Explicitly out of scope for this plan

(Mirrors the design spec's Non-goals — do not add these while implementing.)

- Publishing `libs/schemaregistry` or `libs/eventschemas` as external Go modules; per-repo CI.
- Multi-schema/multi-topic governance tooling; schema-drift alerting.
- Production/staging registry provisioning, or picking a non-local registry auth scheme.
- Protobuf/JSON-Schema support.
- Any event type other than `email.received`.
