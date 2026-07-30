package inbox

import (
	"context"
	"fmt"
	"time"

	"planeo/libs/kafka"
	"planeo/libs/logger"
	"planeo/libs/schemaregistry"

	avro "github.com/iskorotkov/avro/v2"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Decoder Avro-decodes a raw payload into a T value. Declared here (rather
// than depending on *schemaregistry.Decoder[T] directly) so it can be
// mocked in unit tests without a live registry; *schemaregistry.Decoder[T]
// satisfies this structurally, no wrapper needed.
type Decoder[T any] interface {
	Decode(ctx context.Context, b []byte) (T, error)
}

// Deps are Consumer[T]'s business-side collaborators, as function values —
// same method-value binding as libs/outbox.Deps[T]. Process is deliberately
// not required to be named "Process": a domain service's ProcessEmailReceived
// (or any future topic's own name) plugs in as-is.
type Deps[T any] struct {
	FetchBatch func(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]Record, error)
	Process    func(ctx context.Context, id int64, event T, maxAttempts int) error
	MarkFailed func(ctx context.Context, id int64, err error, maxAttempts int) error
}

// Config is Consumer[T]'s scalar, topic/instance-specific configuration.
type Config struct {
	Topic, InstanceID      string
	BatchSize, MaxAttempts int
	ClaimTTL, PollInterval time.Duration
}

// Consumer[T] owns a full inbox consume loop for one topic and one wire
// type T: the raw Kafka-to-DB ingestion loop, plus polling the domain
// service for a batch of claimed rows, Avro-decoding each payload, and
// handing the decoded event to the domain service.
type Consumer[T any] struct {
	rawConsumer RawConsumer
	decoder     Decoder[T]
	service     Deps[T]
	cfg         Config
}

// New is the low-level constructor: takes an already-built raw Kafka
// consumer and decoder. No I/O, no error return.
func New[T any](rawConsumer RawConsumer, decoder Decoder[T], deps Deps[T], cfg Config) *Consumer[T] {
	return &Consumer[T]{rawConsumer: rawConsumer, decoder: decoder, service: deps, cfg: cfg}
}

// SaveFunc is the raw Kafka-to-DB persistence step (dedup by Kafka
// coordinate) the ingestion loop needs. It's a parameter of NewFromConfig
// only, not part of Deps[T] — New's caller supplies an already-built
// RawConsumer that has this baked in from its own construction, so Deps[T]
// would otherwise carry a field New never reads.
//
// NewKafkaConsumer's third parameter is the unexported saver interface
// (Save(ctx, topic, partition, offset, payload) (bool, error)) — a bare
// func value doesn't satisfy that on its own, so SaveFunc carries a Save
// method that calls itself, the same http.HandlerFunc-style adapter used
// throughout the standard library.
type SaveFunc func(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error)

func (f SaveFunc) Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error) {
	return f(ctx, topic, partition, offset, payload)
}

// BuildConfig is the connection-level configuration NewFromConfig needs to
// build the raw Kafka consumer and schema-registry decoder itself. Flat
// scalar fields only, for the same reason as libs/outbox.BuildConfig.
type BuildConfig struct {
	KafkaBrokers    []string
	KafkaUsername   string
	KafkaPassword   string
	KafkaMechanism  string
	KafkaTLSEnabled bool
	GroupName       string
	RegistryURL     string
	Subject         string
	Schema          avro.Schema
}

// NewFromConfig is the high-level, one-call constructor most callers use.
func NewFromConfig[T any](ctx context.Context, build BuildConfig, save SaveFunc, deps Deps[T], cfg Config) (*Consumer[T], error) {
	kafkaClient, err := kafka.NewClient(kafka.Config{
		Brokers: build.KafkaBrokers, Username: build.KafkaUsername,
		Password: build.KafkaPassword, Mechanism: build.KafkaMechanism, TLSEnabled: build.KafkaTLSEnabled,
	},
		kgo.ConsumerGroup(build.GroupName),
		kgo.ConsumeTopics(cfg.Topic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to kafka: %w", err)
	}

	registryClient, err := schemaregistry.NewClient(schemaregistry.Config{URL: build.RegistryURL})
	if err != nil {
		kafkaClient.Close()
		return nil, fmt.Errorf("create schema registry client: %w", err)
	}
	serde := schemaregistry.NewSerde(registryClient)
	if err := schemaregistry.RegisterDecode[T](ctx, serde, build.Subject, build.Schema); err != nil {
		kafkaClient.Close()
		return nil, fmt.Errorf("register %s for decoding: %w", build.Subject, err)
	}

	rawConsumer := NewKafkaConsumer(kafkaClient, cfg.Topic, save)
	decoder := schemaregistry.NewDecoder[T](serde, build.Schema)
	return New[T](rawConsumer, decoder, deps, cfg), nil
}

// Run starts the raw Kafka ingestion loop (non-blocking — it starts its own
// goroutine, which closes the Kafka client itself on ctx.Done(); no
// separate Close is needed on Consumer[T]) and then blocks running its own
// poll ticker over the inbox table until ctx is cancelled.
func (c *Consumer[T]) Run(ctx context.Context) error {
	if err := c.rawConsumer.Run(ctx); err != nil {
		return err
	}
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(c.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.PollOnce(ctx); err != nil {
				log.Error().Err(err).Msg("inbox consumer poll failed")
			}
		}
	}
}

// PollOnce claims a batch of pending inbox rows and processes each in turn.
// One bad record does not stop the rest of the batch.
func (c *Consumer[T]) PollOnce(ctx context.Context) error {
	records, err := c.service.FetchBatch(ctx, c.cfg.Topic, c.cfg.InstanceID, c.cfg.BatchSize, c.cfg.ClaimTTL)
	if err != nil {
		return err
	}
	log := logger.FromContext(ctx)
	for _, rec := range records {
		log.Info().Str("topic", rec.Topic).Int64("inbox_id", rec.ID).Msg("received inbox record")
		c.processRecord(ctx, rec.ID, rec.Payload)
	}
	return nil
}

func (c *Consumer[T]) processRecord(ctx context.Context, recordID int64, payload []byte) {
	log := logger.FromContext(ctx)
	event, err := c.decoder.Decode(ctx, payload)
	if err != nil {
		if markErr := c.service.MarkFailed(ctx, recordID, err, c.cfg.MaxAttempts); markErr != nil {
			log.Error().Err(markErr).Int64("inbox_id", recordID).Msg("failed to mark inbox record failed after decode error")
		}
		return
	}
	if err := c.service.Process(ctx, recordID, event, c.cfg.MaxAttempts); err != nil {
		log.Error().Err(err).Int64("inbox_id", recordID).Msg("failed to process inbox record")
	}
}
