package user

type UserOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}

type GetUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of the User"`
}

type CreateUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}
type UpdateUserInput struct {
	GetUserInput
}
type DeleteUserInput struct {
	GetUserInput
}
