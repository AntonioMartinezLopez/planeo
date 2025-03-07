package settings

import (
	"context"
	"planeo/libs/logger"
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
	logger.Info("Retrieving settings for organizationId: %d", organizationId)
	return s.settingsRepository.GetSettings(ctx, organizationId)
}

func (s *SettingsService) CreateSetting(ctx context.Context, setting models.Setting) error {
	logger.Info("Creating new email setting for organizationId: %d", setting.OrganizationID)
	return s.settingsRepository.CreateSetting(ctx, setting)
}

func (s *SettingsService) UpdateSetting(ctx context.Context, setting models.Setting) error {
	logger.Info("Updating email setting with Id: %d", setting.ID)
	return s.settingsRepository.UpdateSetting(ctx, setting)
}

func (s *SettingsService) DeleteSetting(ctx context.Context, organizationId int, settingId int) error {
	logger.Info("Deleting email setting with Id: %d", settingId)
	return s.settingsRepository.DeleteSetting(ctx, organizationId, settingId)
}
