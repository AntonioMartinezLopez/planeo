package postgres

import (
	"context"
	"planeo/libs/db"
	"planeo/libs/outbox"
	"planeo/services/email/internal/domain/mail"
	"time"

	"github.com/jackc/pgx/v5"
)

func (c *Client) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]outbox.Record, error) {
	cutoff := time.Now().Add(-claimTTL)

	// claimed_by lets THIS instance immediately reclaim its own stuck
	// "processing" rows (e.g. produce succeeded but MarkProcessed failed)
	// on the very next poll, without waiting for claimTTL — sequential
	// single-instance polling guarantees no other in-flight attempt is
	// still touching it. claimTTL remains the fallback for the case where
	// the instance itself crashed and a new instance id needs to recover
	// orphaned rows left by the old one.
	query := `
		UPDATE outbox
		SET status = 'processing', claimed_at = NOW(), claimed_by = @instanceId
		WHERE id IN (
			SELECT id FROM outbox
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
		RETURNING id, topic, key, payload`
	args := pgx.NamedArgs{"topic": topic, "instanceId": instanceID, "cutoff": cutoff, "limit": limit}

	rows, err := c.db.Query(ctx, query, args)
	if err != nil {
		return nil, NewDatabaseError("error claiming outbox batch", err)
	}
	defer rows.Close()

	records, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (outbox.Record, error) {
		var r outbox.Record
		err := row.Scan(&r.ID, &r.Topic, &r.Key, &r.Payload)
		return r, err
	})
	if err != nil {
		return nil, NewDatabaseError("error collecting outbox batch", err)
	}

	return records, nil
}

func (c *Client) MarkProcessed(ctx context.Context, id int64) error {
	args := pgx.NamedArgs{"id": id}
	_, err := c.db.Exec(ctx, `UPDATE outbox SET status = 'sent', processed_at = NOW() WHERE id = @id`, args)
	if err != nil {
		return NewDatabaseError("error marking outbox record processed", err)
	}
	return nil
}

func (c *Client) MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error {
	args := pgx.NamedArgs{"id": id, "lastError": sendErr.Error(), "maxAttempts": maxAttempts}
	query := `
		UPDATE outbox
		SET attempts = attempts + 1,
		    last_error = @lastError,
		    status = CASE WHEN attempts + 1 >= @maxAttempts THEN 'failed' ELSE 'pending' END,
		    failed_at = CASE WHEN attempts + 1 >= @maxAttempts THEN NOW() ELSE failed_at END
		WHERE id = @id`
	_, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error marking outbox record failed", err)
	}
	return nil
}

func (c *Client) CreateOutboxEvent(ctx context.Context, mailID int, event mail.OutboxEvent) error {
	query := `
		INSERT INTO outbox (mail_id, topic, key, payload)
		VALUES (@mailId, @topic, @key, @payload)`
	args := pgx.NamedArgs{
		"mailId":  mailID,
		"topic":   event.Topic,
		"key":     event.Key,
		"payload": event.Payload,
	}

	q := db.FromContext(ctx, c.db)
	if _, err := q.Exec(ctx, query, args); err != nil {
		return NewDatabaseError("error inserting outbox event", err)
	}
	return nil
}
