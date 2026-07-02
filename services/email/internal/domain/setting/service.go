// services/email/internal/domain/setting/service.go
package setting

import "context"

type service struct {
	repository   Repository
	emailFetcher EmailFetcher
}

func NewService(repository Repository, emailFetcher EmailFetcher) (Service, error) {
	settings, err := repository.GetAllSettings(context.Background())
	if err != nil {
		return nil, err
	}

	if err := emailFetcher.StartFetching(context.Background(), settings); err != nil {
		return nil, err
	}

	return &service{
		repository:   repository,
		emailFetcher: emailFetcher,
	}, nil
}

func (s *service) GetSettings(ctx context.Context, organizationId int) ([]Setting, error) {
	return s.repository.GetSettings(ctx, organizationId)
}

func (s *service) CreateSetting(ctx context.Context, setting NewSetting) error {
	created, err := s.repository.CreateSetting(ctx, setting)
	if err != nil {
		return err
	}
	return s.emailFetcher.StartFetching(ctx, []Setting{created})
}

func (s *service) UpdateSetting(ctx context.Context, setting UpdateSetting) error {
	updated, err := s.repository.UpdateSetting(ctx, setting)
	if err != nil {
		return err
	}
	s.emailFetcher.StopFetching(ctx, updated.ID)
	return s.emailFetcher.StartFetching(ctx, []Setting{updated})
}

func (s *service) DeleteSetting(ctx context.Context, organizationId int, settingId int) error {
	if err := s.repository.DeleteSetting(ctx, organizationId, settingId); err != nil {
		return err
	}
	s.emailFetcher.StopFetching(ctx, settingId)
	return nil
}

func (s *service) TestConnection(ctx context.Context, setting Setting) error {
	return s.emailFetcher.TestConnection(ctx, setting)
}
