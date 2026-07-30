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
	return &Serde{client: client, inner: sr.NewSerde()}
}

// RegisterEncode resolves subject's ID and registers T for encoding.
// Called once per type, at construction time, by whichever infra adapter
// owns that type — this is the coupling site.
func RegisterEncode[T any](ctx context.Context, s *Serde, subject string, schema avro.Schema) error {
	id, err := Register(ctx, s.client, subject, schema)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.inner.Register(id, *new(T),
		sr.EncodeFn(func(v any) ([]byte, error) { return avro.Marshal(schema, v) }),
		sr.DecodeFn(func(b []byte, v any) error { return avro.Unmarshal(schema, b, v) }),
	)
	s.mu.Unlock()
	return nil
}

// RegisterDecode resolves subject's ID for schema and registers T for
// decoding under that exact ID. Since this ID corresponds to our own
// compiled-in schema, no writer/reader resolution is needed for it. Unlike
// RegisterEncode, this looks the schema up rather than creating it —
// RegisterDecode is for consumers, which must not conjure a registry entry
// into existence for a schema no producer has ever registered. This
// doubles as a startup fail-fast check: it errors immediately if the
// registry is unreachable or has no entry for our own schema, rather than
// only surfacing that on the first real message.
func RegisterDecode[T any](ctx context.Context, s *Serde, subject string, schema avro.Schema) error {
	ss, err := s.client.LookupSchema(ctx, subject, sr.Schema{Schema: schema.String(), Type: sr.TypeAvro})
	if err != nil {
		return err
	}
	id := ss.ID
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
