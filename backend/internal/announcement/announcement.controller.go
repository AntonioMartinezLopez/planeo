package announcement

import (
	"context"
	"net/http"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/setup/operations"

	"github.com/danielgtaylor/huma/v2"
)

type AnnouncementController struct {
	api                 *huma.API
	announcementService *AnnouncementService
}

func NewAnnouncementController(api *huma.API) *AnnouncementController {
	announcementService := NewAnnouncementService()
	return &AnnouncementController{
		api:                 api,
		announcementService: announcementService,
	}
}

func (a *AnnouncementController) InitializeRoutes() {
	huma.Register(*a.api, operations.WithAuth(huma.Operation{
		OperationID: "get-announcement",
		Method:      http.MethodGet,
		Path:        "/announcement/{id}",
		Summary:     "Get Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*a.api, "read:announcement")},
	}), func(ctx context.Context, input *GetAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.GetAnnouncement(input.Id)

		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*a.api, operations.WithAuth(huma.Operation{
		OperationID: "create-announcement",
		Method:      http.MethodPost,
		Path:        "/announcement",
		Summary:     "Create Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*a.api, "create:announcement")},
	}), func(ctx context.Context, input *CreateAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.CreateAnnouncement()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*a.api, operations.WithAuth(huma.Operation{
		OperationID: "update-announcement",
		Method:      http.MethodPut,
		Path:        "/announcement/{id}",
		Summary:     "Update Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*a.api, "update:announcement")},
	}), func(ctx context.Context, input *UpdateAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.UpdateAnnouncement(input.Id)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*a.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-announcement",
		Method:      http.MethodDelete,
		Path:        "/announcement/{id}",
		Summary:     "Delete Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*a.api, "delete:announcement")},
	}), func(ctx context.Context, input *DeleteAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.DeleteAnnouncement(input.Id)
		resp.Body.Message = result
		return resp, nil
	})
}
