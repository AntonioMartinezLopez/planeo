package announcement

type AnnouncementOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}

type GetAnnouncementInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	Id           string `path:"id" doc:"ID of the task"`
}

type CreateAnnouncementInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}
type UpdateAnnouncementInput struct {
	GetAnnouncementInput
}
type DeleteAnnouncementInput struct {
	GetAnnouncementInput
}
