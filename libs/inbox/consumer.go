package inbox

import (
	"context"

	"planeo/libs/logger"

	"github.com/twmb/franz-go/pkg/kgo"
)

// saver is the one method Consumer needs from whatever persists a raw
// Kafka record. Declared here, inline, now that the shared Store interface
// (which used to live in store.go alongside Record) has been removed —
// Consumer's needs are narrower than the removed interface's full shape.
type saver interface {
	Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (inserted bool, err error)
}

// Consumer reads from Kafka and persists into the inbox, committing the
// offset only after Save succeeds. No Handler is invoked here. Owns its
// own kgo consumer-group client directly — mirrors outbox.Producer's
// independence from any other planeo-internal package.
type Consumer struct {
	brokers   []string
	groupName string
	topic     string
	store     saver
}

func NewConsumer(brokers []string, groupName, topic string, store saver) *Consumer {
	return &Consumer{
		brokers:   brokers,
		groupName: groupName,
		topic:     topic,
		store:     store,
	}
}

// Run subscribes to the configured topic and persists each fetched record
// into the inbox, deduped on the Kafka coordinate, committing the offset
// only after a successful Save. Non-blocking — starts its own background
// goroutine and returns once the subscription is established (or fails to
// start).
func (c *Consumer) Run(ctx context.Context) error {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(c.brokers...),
		kgo.AllowAutoTopicCreation(),
		kgo.ConsumerGroup(c.groupName),
		kgo.ConsumeTopics(c.topic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)

	go func() {
		defer client.Close()

		for {
			if ctx.Err() != nil {
				return
			}

			fetches := client.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return
			}

			fetches.EachError(func(_ string, _ int32, err error) {
				log.Error().Err(err).Msg("kafka fetch error")
			})

			// Kafka offset commits are a single cumulative per-partition
			// marker, not a per-record acknowledgement: if record N's Save
			// fails but record N+1's Save (and commit) succeeds, the
			// committed offset would advance past N, and N would never be
			// redelivered — a silent, permanent loss of exactly the message
			// this pattern exists to protect. To prevent that, once a
			// partition has a failure in this poll, stop committing further
			// records on that partition for the rest of the poll (Save's
			// (topic, partition, offset) dedup makes it safe to reprocess
			// already-saved records on the next poll/redelivery).
			failedPartitions := map[int32]bool{}
			fetches.EachRecord(func(record *kgo.Record) {
				if failedPartitions[record.Partition] {
					return
				}

				if _, err := c.store.Save(ctx, c.topic, record.Partition, record.Offset, record.Value); err != nil {
					log.Error().Err(err).Int32("partition", record.Partition).Int64("offset", record.Offset).Msg("failed to persist inbox record, halting commits on this partition until next poll")
					failedPartitions[record.Partition] = true
					return
				}

				if err := client.CommitRecords(ctx, record); err != nil {
					log.Error().Err(err).Int32("partition", record.Partition).Int64("offset", record.Offset).Msg("failed to commit kafka offset, halting further commits on this partition")
					failedPartitions[record.Partition] = true
				}
			})
		}
	}()

	return nil
}
