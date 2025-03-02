package announcement

import (
	"context"
	"net/http"
	"planeo/services/core/config"
	"planeo/services/core/internal/middlewares"
	"planeo/services/core/internal/setup/operations"

	"github.com/danielgtaylor/huma/v2"
)

type AnnouncementController struct {
	api                 huma.API
	announcementService *AnnouncementService
	config              *config.ApplicationConfiguration
}

func NewAnnouncementController(api huma.API, config *config.ApplicationConfiguration) *AnnouncementController {
	announcementService := NewAnnouncementService()
	return &AnnouncementController{
		api:                 api,
		announcementService: announcementService,
		config:              config,
	}
}

func (a *AnnouncementController) InitializeRoutes() {
	huma.Register(a.api, operations.WithAuth(huma.Operation{
		OperationID: "get-announcement",
		Method:      http.MethodGet,
		Path:        "/{organization}/announcement/{id}",
		Summary:     "Get Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(a.api, a.config, "announcement", "read")},
	}), func(ctx context.Context, input *GetAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.GetAnnouncement(input.Id)

		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(a.api, operations.WithAuth(huma.Operation{
		OperationID: "create-announcement",
		Method:      http.MethodPost,
		Path:        "/{organization}/announcement",
		Summary:     "Create Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(a.api, a.config, "announcement", "create")},
	}), func(ctx context.Context, input *CreateAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.CreateAnnouncement()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(a.api, operations.WithAuth(huma.Operation{
		OperationID: "update-announcement",
		Method:      http.MethodPut,
		Path:        "/{organization}/announcement/{id}",
		Summary:     "Update Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(a.api, a.config, "announcement", "update")},
	}), func(ctx context.Context, input *UpdateAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.UpdateAnnouncement(input.Id)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(a.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-announcement",
		Method:      http.MethodDelete,
		Path:        "/{organization}/announcement/{id}",
		Summary:     "Delete Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(a.api, a.config, "announcement", "delete")},
	}), func(ctx context.Context, input *DeleteAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.DeleteAnnouncement(input.Id)
		resp.Body.Message = result
		return resp, nil
	})
}
