package mail

import "context"

type Repository interface {
	// CreateMail inserts a mail row, returning its id, whether it was
	// newly inserted (false on a duplicate setting_id+message_id, a safe
	// no-op), and any error. Must be called within WithTransaction
	// alongside CreateOutboxEvent to keep both writes atomic.
	CreateMail(ctx context.Context, m NewMail) (mailID int, inserted bool, err error)

	// CreateOutboxEvent inserts the outbox row for a given mail.
	CreateOutboxEvent(ctx context.Context, mailID int, event OutboxEvent) error

	// WithTransaction runs fn within a single database transaction,
	// committing on success and rolling back on error. Repository calls
	// made using the ctx passed to fn participate in that same
	// transaction.
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service interface {
	SaveFetchedMails(ctx context.Context, raws []RawFetchedMail) ([]SaveResult, error)
}
