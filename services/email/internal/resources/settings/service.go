package settings

import (
	"context"
	"planeo/services/email/internal/resources/settings/models"
)

type SettingsRepositoryInterface interface {
	GetSettings(ctx context.Context, organizationId int) ([]models.Setting, error)
	CreateSetting(ctx context.Context, setting models.Setting) error
	UpdateSetting(ctx context.Context, setting models.Setting) error
	DeleteSetting(ctx context.Context, organizationId int, settingId int) error
}

type SettingsService struct {
	settingsRepository SettingsRepositoryInterface
}

func NewSettingsService(settingsRepository SettingsRepositoryInterface) *SettingsService {
	return &SettingsService{
		settingsRepository: settingsRepository,
	}
}

func (s *SettingsService) GetSettings(ctx context.Context, organizationId int) ([]models.Setting, error) {
	return s.settingsRepository.GetSettings(ctx, organizationId)
}

func (s *SettingsService) CreateSetting(ctx context.Context, setting models.Setting) error {
	return s.settingsRepository.CreateSetting(ctx, setting)
}

func (s *SettingsService) UpdateSetting(ctx context.Context, setting models.Setting) error {
	return s.settingsRepository.UpdateSetting(ctx, setting)
}

func (s *SettingsService) DeleteSetting(ctx context.Context, organizationId int, settingId int) error {
	return s.settingsRepository.DeleteSetting(ctx, organizationId, settingId)
}
