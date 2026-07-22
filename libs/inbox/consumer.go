package inbox

import (
	"context"

	"planeo/libs/logger"

	"github.com/twmb/franz-go/pkg/kgo"
)

type saver interface {
	Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (inserted bool, err error)
}

// fetchClient is the narrow slice of *kgo.Client's API Consumer needs -
// declared here (not imported) so Run can be driven by a fake in tests
// without a live broker. *kgo.Client (built via planeo/libs/kafka.NewClient)
// satisfies this structurally; production code never constructs a
// fetchClient value directly, it passes a real *kgo.Client.
type fetchClient interface {
	PollFetches(ctx context.Context) kgo.Fetches
	CommitRecords(ctx context.Context, rs ...*kgo.Record) error
	Close()
}

// Consumer reads from Kafka and persists into the inbox, committing the
// offset only after Save succeeds. No Handler is invoked here. The given
// client must already be configured with its consumer group and topic
// subscription (via kgo.ConsumerGroup/kgo.ConsumeTopics passed as extraOpts
// to planeo/libs/kafka.NewClient) - Consumer only drives it, it doesn't
// configure it.
type Consumer struct {
	client fetchClient
	topic  string
	store  saver
}

func NewConsumer(client fetchClient, topic string, store saver) *Consumer {
	return &Consumer{
		client: client,
		topic:  topic,
		store:  store,
	}
}

// Run subscribes to the configured topic and persists each fetched record
// into the inbox, deduped on the Kafka coordinate, committing the offset
// only after a successful Save. Non-blocking — starts its own background
// goroutine and returns immediately.
func (c *Consumer) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)

	go func() {
		defer c.client.Close()

		for {
			if ctx.Err() != nil {
				return
			}

			fetches := c.client.PollFetches(ctx)
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

				if err := c.client.CommitRecords(ctx, record); err != nil {
					log.Error().Err(err).Int32("partition", record.Partition).Int64("offset", record.Offset).Msg("failed to commit kafka offset, halting further commits on this partition")
					failedPartitions[record.Partition] = true
				}
			})
		}
	}()

	return nil
}
