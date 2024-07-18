package announcement

import "net/http"

type AnnouncementHandler struct{}

func NewAnnouncementHandler() *AnnouncementHandler {
	return &AnnouncementHandler{}
}

func (s *AnnouncementHandler) CreateAnnouncement(w http.ResponseWriter, request *http.Request) {
	w.Write([]byte("Create Announcement endpoint"))
}

func (s *AnnouncementHandler) DeleteAnnouncement(w http.ResponseWriter, request *http.Request) {
	w.Write([]byte("DeleteAnnouncement endpoint"))
}

func (s *AnnouncementHandler) UpdateAnnouncement(w http.ResponseWriter, request *http.Request) {
	w.Write([]byte("UpdateAnnouncement endpoint"))
}

func (s *AnnouncementHandler) GetAnnouncements(w http.ResponseWriter, request *http.Request) {
	w.Write([]byte("GetAnnouncements endpoint"))
}
