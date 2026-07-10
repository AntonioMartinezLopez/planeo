package mail

import "context"

type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{repository: repository}
}

func (s *service) SaveFetchedMails(ctx context.Context, mails []FetchedMail) ([]SaveResult, error) {
	if len(mails) == 0 {
		return nil, nil
	}
	return s.repository.SaveFetchedMails(ctx, mails)
}
