package settings

import (
	"context"
	"net/http"
	humaUtils "planeo/libs/huma_utils"
	"planeo/libs/middlewares"
	"planeo/services/email/config"
	"planeo/services/email/internal/resources/settings/dto"
	"planeo/services/email/internal/resources/settings/models"

	"github.com/danielgtaylor/huma/v2"
)

type SettingsController struct {
	api             huma.API
	settingsService *SettingsService
	config          *config.ApplicationConfiguration
}

func NewSettingsController(api huma.API, config *config.ApplicationConfiguration, settingsService *SettingsService) *SettingsController {
	return &SettingsController{
		api:             api,
		settingsService: settingsService,
		config:          config,
	}
}

func (s *SettingsController) InitializeRoutes() {
	permissions := middlewares.NewPermissionMiddlewareConfig(s.api, s.config.OauthIssuerUrl(), s.config.KcOauthClientID)

	huma.Register(s.api, humaUtils.WithAuth(huma.Operation{
		OperationID: "get-settings",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/settings",
		Summary:     "Get email settings for an organization",
		Tags:        []string{"Settings"},
		Middlewares: huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *dto.GetSettingsInput) (*dto.GetSettingsOutput, error) {

		settings, err := s.settingsService.GetSettings(ctx, input.OrganizationId)
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		response := &dto.GetSettingsOutput{}
		response.Body.Settings = settings
		return response, nil
	})

	huma.Register(s.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "create-setting",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/organizations/{organizationId}/settings",
		Summary:       "Create a new email setting",
		Tags:          []string{"Settings"},
		Middlewares:   huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *dto.CreateSettingInput) (*struct{}, error) {
		setting := models.Setting{
			Host:           input.Body.Host,
			Port:           input.Body.Port,
			Username:       input.Body.Username,
			Password:       input.Body.Password,
			OrganizationID: input.OrganizationId,
		}

		err := s.settingsService.CreateSetting(ctx, setting)
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		return nil, nil
	})

	huma.Register(s.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "update-setting",
		Method:        http.MethodPut,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/settings/{settingId}",
		Summary:       "Update an existing email setting",
		Tags:          []string{"Settings"},
		Middlewares:   huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *dto.UpdateSettingInput) (*struct{}, error) {
		setting := models.Setting{
			ID:             input.SettingId,
			Host:           input.Body.Host,
			Port:           input.Body.Port,
			Username:       input.Body.Username,
			Password:       input.Body.Password,
			OrganizationID: input.OrganizationId,
		}

		err := s.settingsService.UpdateSetting(ctx, setting)
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}
		return nil, nil
	})

	// Delete a setting
	huma.Register(s.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "delete-setting",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/settings/{settingId}",
		Summary:       "Delete an email setting",
		Tags:          []string{"Settings"},
		Middlewares:   huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *dto.DeleteSettingInput) (*struct{}, error) {
		err := s.settingsService.DeleteSetting(ctx, input.OrganizationId, input.SettingId)
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		return nil, nil
	})

	// Test a setting
	huma.Register(s.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "test-setting",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusOK,
		Path:          "/settings/test",
		Summary:       "Test an email setting",
		Tags:          []string{"Settings"},
		Middlewares:   huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *dto.TestSettingInput) (*struct{}, error) {
		err := s.settingsService.TestConnection(ctx, models.Setting{
			Host:     input.Body.Host,
			Port:     input.Body.Port,
			Username: input.Body.Username,
			Password: input.Body.Password,
		})
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		return nil, nil
	})

}
