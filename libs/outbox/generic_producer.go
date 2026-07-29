package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"planeo/libs/kafka"
	"planeo/libs/logger"
	"planeo/libs/schemaregistry"

	avro "github.com/iskorotkov/avro/v2"
)

// Encoder Avro-encodes a decoded T value. Declared here (rather than
// depending on *schemaregistry.Encoder[T] directly) so it can be mocked in
// unit tests without a live registry; *schemaregistry.Encoder[T] satisfies
// this structurally, no wrapper needed.
type Encoder[T any] interface {
	Encode(v T) ([]byte, error)
}

// Deps are Producer[T]'s business-side collaborators — the domain service's
// three methods, as function values. Method values match by signature, not
// name: a domain service can call this method anything (ProcessRecord,
// whatever a future topic needs) and it still plugs in here unchanged.
type Deps[T any] struct {
	FetchBatch    func(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]Record, error)
	MarkProcessed func(ctx context.Context, id int64) error
	MarkFailed    func(ctx context.Context, id int64, err error, maxAttempts int) error
}

// Config is Producer[T]'s scalar, topic/instance-specific configuration —
// independent of how the underlying connections were built.
type Config struct {
	Topic, InstanceID      string
	BatchSize, MaxAttempts int
	ClaimTTL, PollInterval time.Duration
}

// Producer[T] owns a full outbox produce loop for one topic and one wire
// type T: poll the domain service for a batch of pending rows, JSON-decode
// each row's staged payload into T, Avro-encode it, send it, and record the
// outcome. Zero knowledge of what T actually is beyond its JSON/Avro tags.
type Producer[T any] struct {
	rawProducer RawProducer
	encoder     Encoder[T]
	service     Deps[T]
	cfg         Config
	closeFn     func() error // no-op unless built via NewFromConfig
}

// New is the low-level constructor: takes an already-built raw producer and
// encoder. No I/O, no error return — this is what unit tests use, with
// fakes, no live Kafka or registry required.
func New[T any](rawProducer RawProducer, encoder Encoder[T], deps Deps[T], cfg Config) *Producer[T] {
	return &Producer[T]{
		rawProducer: rawProducer, encoder: encoder,
		service: deps, cfg: cfg, closeFn: func() error { return nil },
	}
}

// BuildConfig is the connection-level configuration NewFromConfig needs to
// build the raw Kafka producer and schema-registry encoder itself. Flat
// scalar fields only — no kafka.Config or schemaregistry.Config — so a
// caller populating this doesn't need to import either package just to
// wrap one or five values in a struct. This package still imports both
// internally (it has to, to actually call
// kafka.NewClient/schemaregistry.NewClient); this only decouples the
// caller's Config shape from theirs.
type BuildConfig struct {
	KafkaBrokers    []string
	KafkaUsername   string
	KafkaPassword   string
	KafkaMechanism  string
	KafkaTLSEnabled bool
	RegistryURL     string
	Subject         string
	Schema          avro.Schema
}

// NewFromConfig is the high-level, one-call constructor most callers use:
// builds the Kafka client, the schema-registry client, registers T for
// encoding, and returns a ready-to-run Producer[T]. The caller owns calling
// Close() (it owns the Kafka client's lifecycle now, unlike New).
func NewFromConfig[T any](ctx context.Context, build BuildConfig, deps Deps[T], cfg Config) (*Producer[T], error) {
	kafkaClient, err := kafka.NewClient(kafka.Config{
		Brokers: build.KafkaBrokers, Username: build.KafkaUsername,
		Password: build.KafkaPassword, Mechanism: build.KafkaMechanism, TLSEnabled: build.KafkaTLSEnabled,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to kafka: %w", err)
	}

	registryClient, err := schemaregistry.NewClient(schemaregistry.Config{URL: build.RegistryURL})
	if err != nil {
		kafkaClient.Close()
		return nil, fmt.Errorf("create schema registry client: %w", err)
	}
	serde := schemaregistry.NewSerde(registryClient)
	if err := schemaregistry.RegisterEncode[T](ctx, serde, build.Subject, build.Schema); err != nil {
		kafkaClient.Close()
		return nil, fmt.Errorf("register %s for encoding: %w", build.Subject, err)
	}

	p := New[T](NewKafkaProducer(kafkaClient), schemaregistry.NewEncoder[T](serde), deps, cfg)
	// (*kgo.Client).Close returns nothing, not an error — wrap it to match
	// closeFn's signature.
	p.closeFn = func() error { kafkaClient.Close(); return nil }
	return p, nil
}

// Close releases the Kafka connection. A Producer[T] built via New (tests,
// or a caller supplying its own already-managed client) has nothing to
// release — Close is a no-op there.
func (p *Producer[T]) Close() error { return p.closeFn() }

// Run polls on a fixed interval until ctx is cancelled. Replaces the
// deleted libs/outbox.Runner — this type now owns its own loop directly.
func (p *Producer[T]) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.PollOnce(ctx); err != nil {
				log.Error().Err(err).Msg("outbox producer poll failed")
			}
		}
	}
}

// PollOnce claims a batch of pending rows and processes each in turn. One
// bad record does not stop the rest of the batch.
func (p *Producer[T]) PollOnce(ctx context.Context) error {
	records, err := p.service.FetchBatch(ctx, p.cfg.Topic, p.cfg.InstanceID, p.cfg.BatchSize, p.cfg.ClaimTTL)
	if err != nil {
		return err
	}
	log := logger.FromContext(ctx)
	for _, rec := range records {
		log.Info().Str("topic", rec.Topic).Int64("outbox_id", rec.ID).Msg("sending outbox record")
		p.processRecord(ctx, rec)
	}
	return nil
}

func (p *Producer[T]) processRecord(ctx context.Context, rec Record) {
	log := logger.FromContext(ctx)
	if err := p.produce(ctx, rec); err != nil {
		if markErr := p.service.MarkFailed(ctx, rec.ID, err, p.cfg.MaxAttempts); markErr != nil {
			log.Error().Err(markErr).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record failed after send error")
		}
		return
	}
	if err := p.service.MarkProcessed(ctx, rec.ID); err != nil {
		log.Error().Err(err).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record processed")
	}
}

func (p *Producer[T]) produce(ctx context.Context, rec Record) error {
	var v T
	if err := json.Unmarshal(rec.Payload, &v); err != nil {
		return fmt.Errorf("decode staged outbox payload: %w", err)
	}
	avroBytes, err := p.encoder.Encode(v)
	if err != nil {
		return fmt.Errorf("avro encode: %w", err)
	}
	return p.rawProducer.ProduceSync(ctx, rec.Topic, rec.Key, avroBytes)
}
