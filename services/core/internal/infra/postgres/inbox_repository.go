package postgres

import (
	"context"
	"errors"
	"planeo/libs/db"
	"planeo/libs/inbox"
	"time"

	"github.com/jackc/pgx/v5"
)

// WithTransaction runs fn within a single database transaction — repository
// methods called with fn's ctx (via db.FromContext) participate in it.
func (c *Client) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return db.WithTx(ctx, c.db, fn)
}

func (c *Client) Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error) {
	query := `
		INSERT INTO inbox (topic, partition, "offset", payload)
		VALUES (@topic, @partition, @offset, @payload)
		ON CONFLICT (topic, partition, "offset") DO NOTHING
		RETURNING id`
	args := pgx.NamedArgs{
		"topic":     topic,
		"partition": partition,
		"offset":    offset,
		"payload":   payload,
	}

	row, err := c.db.Query(ctx, query, args)
	if err != nil {
		return false, NewDatabaseError("error inserting inbox record", err)
	}

	_, err = pgx.CollectExactlyOneRow(row, pgx.RowTo[int64])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, NewDatabaseError("error collecting inserted inbox id", err)
	}

	return true, nil
}

func (c *Client) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]inbox.Record, error) {
	cutoff := time.Now().Add(-claimTTL)

	// Same claimed_by fast-reclaim / claimTTL fallback shape as
	// services/email's outbox FetchBatch — see that file's comment for the
	// full rationale. Scoped by topic, mirroring the outbox side, so that if
	// services/core's inbox ever receives a second topic, two consumer
	// adapters can't steal each other's rows.
	query := `
		UPDATE inbox
		SET status = 'processing', claimed_at = NOW(), claimed_by = @instanceId
		WHERE id IN (
			SELECT id FROM inbox
			WHERE topic = @topic
			  AND (
			      status = 'pending'
			   OR (status = 'processing' AND claimed_by = @instanceId)
			   OR (status = 'processing' AND claimed_at < @cutoff)
			  )
			ORDER BY id
			LIMIT @limit
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, topic, payload`
	args := pgx.NamedArgs{"topic": topic, "instanceId": instanceID, "cutoff": cutoff, "limit": limit}

	rows, err := c.db.Query(ctx, query, args)
	if err != nil {
		return nil, NewDatabaseError("error claiming inbox batch", err)
	}
	defer rows.Close()

	records, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (inbox.Record, error) {
		var r inbox.Record
		err := row.Scan(&r.ID, &r.Topic, &r.Payload)
		return r, err
	})
	if err != nil {
		return nil, NewDatabaseError("error collecting inbox batch", err)
	}

	return records, nil
}

// MarkProcessed resolves its Querier via db.FromContext: when called with a
// WithTransaction-derived ctx (as the consumer adapter does), this
// participates in that transaction rather than running on a separate,
// auto-committed connection.
func (c *Client) MarkProcessed(ctx context.Context, id int64) error {
	args := pgx.NamedArgs{"id": id}
	q := db.FromContext(ctx, c.db)
	_, err := q.Exec(ctx, `UPDATE inbox SET status = 'processed', processed_at = NOW() WHERE id = @id`, args)
	if err != nil {
		return NewDatabaseError("error marking inbox record processed", err)
	}
	return nil
}

// MarkFailed is always called on a plain (non-transaction) ctx by the
// consumer adapter — see that adapter's own comment for why it must never
// be called on a ctx whose transaction has already errored.
func (c *Client) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	args := pgx.NamedArgs{"id": id, "lastError": procErr.Error(), "maxAttempts": maxAttempts}
	query := `
		UPDATE inbox
		SET attempts = attempts + 1,
		    last_error = @lastError,
		    status = CASE WHEN attempts + 1 >= @maxAttempts THEN 'failed' ELSE 'pending' END,
		    failed_at = CASE WHEN attempts + 1 >= @maxAttempts THEN NOW() ELSE failed_at END
		WHERE id = @id`
	q := db.FromContext(ctx, c.db)
	_, err := q.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error marking inbox record failed", err)
	}
	return nil
}
