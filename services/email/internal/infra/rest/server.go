// services/email/internal/infra/rest/server.go
package rest

import (
	"fmt"
	"planeo/libs/api"
	"planeo/libs/middlewares"
	"planeo/services/email/internal/config"
	"planeo/services/email/internal/domain/setting"
	"planeo/services/email/internal/infra/rest/api/v1/settings"
)

type Services struct {
	SettingService setting.Service
}

func New(cfg *config.ApplicationConfiguration, services Services) *api.API {
	humaAPI := api.NewHumaAPI(cfg, "Planeo E-Mail Service", "0.0.1", "/api")

	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", cfg.OauthIssuerUrl())
	appMiddlewares := []api.Middleware{
		middlewares.AuthMiddleware(humaAPI.Api, jwksURL, cfg.OauthIssuerUrl()),
	}

	settingsHandler := settings.NewSettingsHandler(humaAPI.Api, cfg, services.SettingService)

	api.InitializeControllers(humaAPI.Api, appMiddlewares, []api.Controller{settingsHandler})

	return humaAPI
}
