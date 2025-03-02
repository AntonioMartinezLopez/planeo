package announcement

type AnnouncementService struct {
}

func NewAnnouncementService() *AnnouncementService {
	return &AnnouncementService{}
}

func (s *AnnouncementService) CreateAnnouncement() string {
	return "CreateAnnouncement endpoint"
}

func (s *AnnouncementService) DeleteAnnouncement(id string) string {
	return "DeleteAnnouncement endpoint"
}

func (s *AnnouncementService) UpdateAnnouncement(id string) string {
	return "UpdateAnnouncement endpoint"
}

func (s *AnnouncementService) GetAnnouncement(id string) string {
	return "GetAnnouncement endpoint"
}
