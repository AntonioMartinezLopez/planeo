package outbox

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer sends a single record to Kafka. Implemented by kafkaProducer in
// production; test code can supply a fake so an adapter's poll/mark logic is
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
