package user

type UserOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}

type GetUserInput struct {
	UserId string `path:"userId" doc:"ID of the User"`
}

type CreateUserInput struct{}
type UpdateUserInput struct {
	GetUserInput
}
type DeleteUserInput struct {
	GetUserInput
}
