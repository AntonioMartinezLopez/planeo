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
