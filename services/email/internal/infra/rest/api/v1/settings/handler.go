// services/email/internal/infra/rest/api/v1/settings/handler.go
package settings

import (
	"context"
	"net/http"
	humaUtils "planeo/libs/huma_utils"
	"planeo/libs/middlewares"
	"planeo/services/email/internal/config"
	"planeo/services/email/internal/domain/setting"

	"github.com/danielgtaylor/huma/v2"
)

type SettingsHandler struct {
	api            huma.API
	settingService setting.Service
	config         *config.ApplicationConfiguration
}

func NewSettingsHandler(api huma.API, cfg *config.ApplicationConfiguration, svc setting.Service) *SettingsHandler {
	return &SettingsHandler{
		api:            api,
		settingService: svc,
		config:         cfg,
	}
}

//nolint:funlen
func (h *SettingsHandler) InitializeRoutes() {
	permissions := middlewares.NewPermissionMiddlewareConfig(h.api, h.config.OauthIssuerUrl(), h.config.KcOauthClientID)

	huma.Register(h.api, humaUtils.WithAuth(huma.Operation{
		OperationID: "get-settings",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/settings",
		Summary:     "Get email settings for an organization",
		Tags:        []string{"Settings"},
		Middlewares: huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *GetSettingsInput) (*GetSettingsOutput, error) {
		settings, err := h.settingService.GetSettings(ctx, input.OrganizationId)
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}
		resp := &GetSettingsOutput{}
		resp.Body.Settings = settings
		return resp, nil
	})

	huma.Register(h.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "create-setting",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/organizations/{organizationId}/settings",
		Summary:       "Create a new email setting",
		Tags:          []string{"Settings"},
		Middlewares:   huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *CreateSettingInput) (*struct{}, error) {
		err := h.settingService.CreateSetting(ctx, setting.NewSetting{
			Host:           input.Body.Host,
			Port:           input.Body.Port,
			Username:       input.Body.Username,
			Password:       input.Body.Password,
			OrganizationID: input.OrganizationId,
		})
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}
		return nil, nil
	})

	huma.Register(h.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "update-setting",
		Method:        http.MethodPut,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/settings/{settingId}",
		Summary:       "Update an existing email setting",
		Tags:          []string{"Settings"},
		Middlewares:   huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *UpdateSettingInput) (*struct{}, error) {
		err := h.settingService.UpdateSetting(ctx, setting.UpdateSetting{
			ID:             input.SettingId,
			Host:           input.Body.Host,
			Port:           input.Body.Port,
			Username:       input.Body.Username,
			Password:       input.Body.Password,
			OrganizationID: input.OrganizationId,
		})
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}
		return nil, nil
	})

	huma.Register(h.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "delete-setting",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/settings/{settingId}",
		Summary:       "Delete an email setting",
		Tags:          []string{"Settings"},
		Middlewares:   huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *DeleteSettingInput) (*struct{}, error) {
		err := h.settingService.DeleteSetting(ctx, input.OrganizationId, input.SettingId)
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}
		return nil, nil
	})

	huma.Register(h.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "test-setting",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusOK,
		Path:          "/settings/test",
		Summary:       "Test an email setting",
		Tags:          []string{"Settings"},
		Middlewares:   huma.Middlewares{permissions.Apply("organization", "manage")},
	}), func(ctx context.Context, input *TestSettingInput) (*struct{}, error) {
		err := h.settingService.TestConnection(ctx, setting.Setting{
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
