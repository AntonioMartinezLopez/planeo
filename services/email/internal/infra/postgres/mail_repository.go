package postgres

import (
	"context"
	"errors"
	"planeo/services/email/internal/domain/mail"

	"github.com/jackc/pgx/v5"
)

func (c *Client) SaveFetchedMails(ctx context.Context, mails []mail.FetchedMail) ([]mail.SaveResult, error) {
	results := make([]mail.SaveResult, 0, len(mails))

	tx, err := c.db.Begin(ctx)
	if err != nil {
		return nil, NewDatabaseError("error starting transaction", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, fetched := range mails {
		insertMailQuery := `
			INSERT INTO mails (message_id, setting_id, organization_id, subject, sender, body, date)
			VALUES (@messageId, @settingId, @organizationId, @subject, @sender, @body, @date)
			ON CONFLICT (setting_id, message_id) DO NOTHING
			RETURNING id`
		args := pgx.NamedArgs{
			"messageId":      fetched.Mail.MessageID,
			"settingId":      fetched.Mail.SettingID,
			"organizationId": fetched.Mail.OrganizationID,
			"subject":        fetched.Mail.Subject,
			"sender":         fetched.Mail.Sender,
			"body":           fetched.Mail.Body,
			"date":           fetched.Mail.Date,
		}

		row, err := tx.Query(ctx, insertMailQuery, args)
		if err != nil {
			return nil, NewDatabaseError("error inserting mail", err)
		}

		mailID, err := pgx.CollectExactlyOneRow(row, pgx.RowTo[int])
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				results = append(results, mail.SaveResult{UID: fetched.UID, Inserted: false})
				continue
			}
			return nil, NewDatabaseError("error collecting inserted mail id", err)
		}

		insertOutboxQuery := `
			INSERT INTO outbox (mail_id, topic, key, payload)
			VALUES (@mailId, @topic, @key, @payload)`
		outboxArgs := pgx.NamedArgs{
			"mailId":  mailID,
			"topic":   fetched.Event.Topic,
			"key":     fetched.Event.Key,
			"payload": fetched.Event.Payload,
		}

		if _, err := tx.Exec(ctx, insertOutboxQuery, outboxArgs); err != nil {
			return nil, NewDatabaseError("error inserting outbox event", err)
		}

		results = append(results, mail.SaveResult{UID: fetched.UID, Inserted: true})
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, NewDatabaseError("error committing transaction", err)
	}

	return results, nil
}
