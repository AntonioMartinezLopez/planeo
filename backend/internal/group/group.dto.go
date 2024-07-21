package group

type GroupOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}

type GetGroupInput struct {
	GroupId string `path:"groupId" doc:"ID of the Group"`
}

type CreateGroupInput struct{}
type UpdateGroupInput struct {
	GetGroupInput
}
type DeleteGroupInput struct {
	GetGroupInput
}
