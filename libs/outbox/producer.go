package outbox

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// RawProducer sends a single record to Kafka. Implemented by kafkaProducer in
// production; test code can supply a fake so an adapter's poll/mark logic is
// unit-testable without a live broker.
type RawProducer interface {
	ProduceSync(ctx context.Context, topic string, key, value []byte) error
}

// syncProducer is the narrow slice of *kgo.Client's API kafkaProducer needs -
// declared here (not imported) so kafkaProducer can be driven by a fake in
// tests without a live broker. *kgo.Client (built via planeo/libs/kafka.NewClient)
// satisfies this structurally; production code never constructs a
// syncProducer value directly, it passes a real *kgo.Client.
type syncProducer interface {
	ProduceSync(ctx context.Context, rs ...*kgo.Record) kgo.ProduceResults
}

type kafkaProducer struct {
	client syncProducer
}

func (p *kafkaProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	result := p.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: key, Value: value})
	return result.FirstErr()
}

// NewKafkaProducer wraps an already-authenticated *kgo.Client (see
// planeo/libs/kafka.NewClient) as a RawProducer. This package no longer
// constructs or owns the client's lifecycle - the caller does, and is
// responsible for calling client.Close() on shutdown.
func NewKafkaProducer(client syncProducer) RawProducer {
	return &kafkaProducer{client: client}
}
