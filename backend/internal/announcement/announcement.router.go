package announcement

import (
	"github.com/go-chi/chi/v5"
)

func AnnouncementRouter(router chi.Router) {

	announcementHandler := NewAnnouncementHandler()

	router.Route("/announcement", func(r chi.Router) {
		r.Get("/", announcementHandler.GetAnnouncements)
		r.Post("/", announcementHandler.CreateAnnouncement)
		r.Patch("/{id}", announcementHandler.UpdateAnnouncement)
		r.Delete("/{id}", announcementHandler.DeleteAnnouncement)
	})
}
