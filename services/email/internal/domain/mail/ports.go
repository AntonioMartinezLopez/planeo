package mail

import "context"

type Repository interface {
	// SaveFetchedMails durably records each fetched mail and its outbox
	// event in one Postgres transaction. A mail that already exists
	// (duplicate setting_id+message_id) is a no-op for both tables; it is
	// still reported in the result (Inserted: false) so the caller can
	// safely mark it seen on IMAP.
	SaveFetchedMails(ctx context.Context, mails []FetchedMail) ([]SaveResult, error)
}

type Service interface {
	SaveFetchedMails(ctx context.Context, mails []FetchedMail) ([]SaveResult, error)
}
