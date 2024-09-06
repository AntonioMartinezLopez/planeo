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
		Path:        "/{organization}/announcement/{id}",
		Summary:     "Get Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*a.api, "announcement", "read")},
	}), func(ctx context.Context, input *GetAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.GetAnnouncement(input.Id)

		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*a.api, operations.WithAuth(huma.Operation{
		OperationID: "create-announcement",
		Method:      http.MethodPost,
		Path:        "/{organization}/announcement",
		Summary:     "Create Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*a.api, "announcement", "create")},
	}), func(ctx context.Context, input *CreateAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.CreateAnnouncement()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*a.api, operations.WithAuth(huma.Operation{
		OperationID: "update-announcement",
		Method:      http.MethodPut,
		Path:        "/{organization}/announcement/{id}",
		Summary:     "Update Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*a.api, "announcement", "update")},
	}), func(ctx context.Context, input *UpdateAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.UpdateAnnouncement(input.Id)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*a.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-announcement",
		Method:      http.MethodDelete,
		Path:        "/{organization}/announcement/{id}",
		Summary:     "Delete Announcement",
		Tags:        []string{"Announcement"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*a.api, "announcement", "delete")},
	}), func(ctx context.Context, input *DeleteAnnouncementInput) (*AnnouncementOutput, error) {
		resp := &AnnouncementOutput{}
		result := a.announcementService.DeleteAnnouncement(input.Id)
		resp.Body.Message = result
		return resp, nil
	})
}
