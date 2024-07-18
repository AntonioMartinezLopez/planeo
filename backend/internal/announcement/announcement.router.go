package announcement

import (
	"planeo/api/internal/middlewares"

	"github.com/go-chi/chi/v5"
)

func AnnouncementRouter(router chi.Router) {

	announcementHandler := NewAnnouncementHandler()

	router.Route("/announcement", func(r chi.Router) {
		r.With(middlewares.PermissionValidator("read:announcement")).Get("/", announcementHandler.GetAnnouncements)
		r.With(middlewares.PermissionValidator("create:announcement")).Post("/", announcementHandler.CreateAnnouncement)
		r.With(middlewares.PermissionValidator("update:announcement")).Patch("/{id}", announcementHandler.UpdateAnnouncement)
		r.With(middlewares.PermissionValidator("delete:announcement")).Delete("/{id}", announcementHandler.DeleteAnnouncement)
	})
}
