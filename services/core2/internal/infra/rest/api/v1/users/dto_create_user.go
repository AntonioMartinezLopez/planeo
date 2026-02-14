package users

type CreateUserInputBody struct {
	FirstName string `json:"firstName" doc:"First name of the user to be created" example:"John"`
	LastName  string `json:"lastName" doc:"Last name of the user to be created" example:"Doe"`
	Email     string `json:"email" doc:"Email of the user to be created" example:"John.Doe@planeo.de"`
	Password  string `json:"password" doc:"Initial password for the user to be set" example:"password123"`
}

type CreateUserInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	Body           CreateUserInputBody
	RawBody        []byte
}
