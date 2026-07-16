package mail

import (
	"context"
	"encoding/json"
	"planeo/libs/events/contracts"
	"strconv"
)

type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{repository: repository}
}

func (s *service) SaveFetchedMails(ctx context.Context, raws []RawFetchedMail) ([]SaveResult, error) {
	if len(raws) == 0 {
		return nil, nil
	}

	var results []SaveResult
	err := s.repository.WithTransaction(ctx, func(ctx context.Context) error {
		for _, raw := range raws {
			newMail, event, err := buildMailAndEvent(raw)
			if err != nil {
				return err
			}

			mailID, inserted, err := s.repository.CreateMail(ctx, newMail)
			if err != nil {
				return err
			}

			if inserted {
				if err := s.repository.CreateOutboxEvent(ctx, mailID, event); err != nil {
					return err
				}
			}

			results = append(results, SaveResult{UID: raw.UID, Inserted: inserted})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

func buildMailAndEvent(raw RawFetchedMail) (NewMail, OutboxEvent, error) {
	newMail := NewMail{
		MessageID:      raw.MessageID,
		SettingID:      raw.SettingID,
		OrganizationID: raw.OrganizationID,
		Subject:        raw.Subject,
		Sender:         raw.Sender,
		Body:           raw.Body,
		Date:           raw.Date,
	}

	payload, err := json.Marshal(contracts.EmailCreatedPayload{
		Subject:        raw.Subject,
		Body:           raw.Body,
		From:           raw.Sender,
		Date:           raw.Date,
		MessageID:      raw.MessageID,
		OrganizationId: raw.OrganizationID,
	})
	if err != nil {
		return NewMail{}, OutboxEvent{}, err
	}

	event := OutboxEvent{
		Topic:   contracts.EmailReceivedTopic,
		Key:     []byte(strconv.Itoa(raw.OrganizationID)),
		Payload: payload,
	}

	return newMail, event, nil
}
