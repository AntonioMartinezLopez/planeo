package setup

import (
	"context"
	"fmt"
	"net/http"

	cfg "planeo/api/config"
	"planeo/api/internal/middlewares"
	"planeo/api/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

type Controller interface {
	InitializeRoutes()
}

func registerControllers(config *cfg.ApplicationConfiguration, api huma.API, controllers []Controller) {
	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", config.OauthIssuerUrl())
	api.UseMiddleware(middlewares.AuthMiddleware(api, jwksURL, config.OauthIssuerUrl()))
	type Message struct {
		Alive bool `json:"alive" path:"status" doc:"Status of the API server" `
	}

	type StatusOutput struct {
		Body struct {
			Message
		}
	}

	huma.Register(api, huma.Operation{
		OperationID: "server-status",
		Method:      http.MethodGet,
		Path:        "/status",
		Summary:     "Get the server status",
	}, func(ctx context.Context, input *struct{}) (*StatusOutput, error) {
		resp := &StatusOutput{}
		resp.Body.Message.Alive = true
		return resp, nil
	})

	// initialize new routes
	for _, controller := range controllers {
		controller.InitializeRoutes()
	}

}

func SetupRoutes(api huma.API, config *cfg.ApplicationConfiguration, db *db.DBConnection) {
	controllers := InitializeControllers(&api, config, db)
	registerControllers(config, api, controllers)
}
