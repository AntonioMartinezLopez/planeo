package announcement

type AnnouncementOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}

type GetAnnouncementInput struct {
	Id string `path:"id" doc:"ID of the task"`
}

type CreateAnnouncementInput struct{}
type UpdateAnnouncementInput struct {
	GetAnnouncementInput
}
type DeleteAnnouncementInput struct {
	GetAnnouncementInput
}
