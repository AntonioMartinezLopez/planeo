package postgres

import (
	"context"
	"errors"
	"planeo/libs/db"
	"planeo/services/email/internal/domain/mail"

	"github.com/jackc/pgx/v5"
)

func (c *Client) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return db.WithTx(ctx, c.db, fn)
}

func (c *Client) CreateMail(ctx context.Context, m mail.NewMail) (int, bool, error) {
	query := `
		INSERT INTO mails (message_id, setting_id, organization_id, subject, sender, body, date)
		VALUES (@messageId, @settingId, @organizationId, @subject, @sender, @body, @date)
		ON CONFLICT (setting_id, message_id) DO NOTHING
		RETURNING id`
	args := pgx.NamedArgs{
		"messageId":      m.MessageID,
		"settingId":      m.SettingID,
		"organizationId": m.OrganizationID,
		"subject":        m.Subject,
		"sender":         m.Sender,
		"body":           m.Body,
		"date":           m.Date,
	}

	q := db.FromContext(ctx, c.db)
	row, err := q.Query(ctx, query, args)
	if err != nil {
		return 0, false, NewDatabaseError("error inserting mail", err)
	}

	mailID, err := pgx.CollectExactlyOneRow(row, pgx.RowTo[int])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, NewDatabaseError("error collecting inserted mail id", err)
	}

	return mailID, true, nil
}
