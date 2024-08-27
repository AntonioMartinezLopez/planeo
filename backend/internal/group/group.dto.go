package group

type GroupOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}

type GetGroupInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	GroupId      string `path:"groupId" doc:"ID of the Group"`
}

type CreateGroupInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}
type UpdateGroupInput struct {
	GetGroupInput
}
type DeleteGroupInput struct {
	GetGroupInput
}
