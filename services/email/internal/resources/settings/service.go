package settings

import (
	"context"
	"planeo/services/email/internal/resources/settings/models"
)

type SettingsRepositoryInterface interface {
	GetSettings(ctx context.Context, organizationId int) ([]models.Setting, error)
	CreateSetting(ctx context.Context, setting models.Setting) (models.Setting, error)
	UpdateSetting(ctx context.Context, setting models.Setting) (models.Setting, error)
	DeleteSetting(ctx context.Context, organizationId int, settingId int) error
	GetAllSettings(ctx context.Context) ([]models.Setting, error)
}

type EmailServiceInterface interface {
	StartFetching(context context.Context, settings []models.Setting) error
	StopFetching(context context.Context, settingsId int)
	TestConnection(context context.Context, settings models.Setting) error
}

type SettingsService struct {
	settingsRepository SettingsRepositoryInterface
	emailService       EmailServiceInterface
}

func NewSettingsService(settingsRepository SettingsRepositoryInterface, emailService EmailServiceInterface) *SettingsService {

	settings, err := settingsRepository.GetAllSettings(context.Background())
	if err != nil {
		panic(err)
	}

	err = emailService.StartFetching(context.Background(), settings)
	if err != nil {
		panic(err)
	}

	return &SettingsService{
		settingsRepository: settingsRepository,
		emailService:       emailService,
	}
}

func (s *SettingsService) GetSettings(ctx context.Context, organizationId int) ([]models.Setting, error) {
	return s.settingsRepository.GetSettings(ctx, organizationId)
}

func (s *SettingsService) CreateSetting(ctx context.Context, setting models.Setting) error {
	createdSetting, err := s.settingsRepository.CreateSetting(ctx, setting)

	if err == nil {
		err = s.emailService.StartFetching(ctx, []models.Setting{createdSetting})
	}

	return err

}

func (s *SettingsService) UpdateSetting(ctx context.Context, setting models.Setting) error {
	updatedSetting, err := s.settingsRepository.UpdateSetting(ctx, setting)
	if err != nil {
		return err
	}

	s.emailService.StopFetching(ctx, updatedSetting.ID)
	return s.emailService.StartFetching(ctx, []models.Setting{updatedSetting})
}

func (s *SettingsService) DeleteSetting(ctx context.Context, organizationId int, settingId int) error {
	err := s.settingsRepository.DeleteSetting(ctx, organizationId, settingId)

	if err == nil {
		s.emailService.StopFetching(ctx, settingId)
	}

	return err
}

func (s *SettingsService) TestConnection(ctx context.Context, setting models.Setting) error {
	return s.emailService.TestConnection(ctx, setting)
}
