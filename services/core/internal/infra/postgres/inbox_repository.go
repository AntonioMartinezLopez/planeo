package postgres

import (
	"context"
	"errors"
	"planeo/libs/inbox"
	"time"

	"github.com/jackc/pgx/v5"
)

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

func (c *Client) FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]inbox.Record, error) {
	cutoff := time.Now().Add(-claimTTL)

	query := `
		UPDATE inbox
		SET status = 'processing', claimed_at = NOW()
		WHERE id IN (
			SELECT id FROM inbox
			WHERE status = 'pending'
			   OR (status = 'processing' AND claimed_at < @cutoff)
			ORDER BY id
			LIMIT @limit
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, topic, payload`
	args := pgx.NamedArgs{"cutoff": cutoff, "limit": limit}

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

func (c *Client) MarkProcessed(ctx context.Context, id int64) error {
	args := pgx.NamedArgs{"id": id}
	_, err := c.db.Exec(ctx, `UPDATE inbox SET status = 'processed', processed_at = NOW() WHERE id = @id`, args)
	if err != nil {
		return NewDatabaseError("error marking inbox record processed", err)
	}
	return nil
}

func (c *Client) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	args := pgx.NamedArgs{"id": id, "lastError": procErr.Error(), "maxAttempts": maxAttempts}
	query := `
		UPDATE inbox
		SET attempts = attempts + 1,
		    last_error = @lastError,
		    status = CASE WHEN attempts + 1 >= @maxAttempts THEN 'failed' ELSE 'pending' END,
		    failed_at = CASE WHEN attempts + 1 >= @maxAttempts THEN NOW() ELSE failed_at END
		WHERE id = @id`
	_, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error marking inbox record failed", err)
	}
	return nil
}
