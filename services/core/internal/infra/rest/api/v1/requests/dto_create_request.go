package requests

type CreateRequestInputBody struct {
	Subject    string `json:"subject" doc:"Subject of the request" example:"Some request subject"`
	Text       string `json:"text" doc:"Description of the request" example:"Some request text"`
	Name       string `json:"name" doc:"Name of the requester" example:"John Doe"`
	Email      string `json:"email" doc:"Email of the requester" example:"John.Doe@example.com"`
	Address    string `json:"address" doc:"Address of the requester" example:"789 Oak St, Metropolis"`
	Telephone  string `json:"telephone" doc:"Telephone number of the requester" example:"1234567"`
	Closed     bool   `json:"closed" doc:"Indicates if the request is closed" example:"false"`
	CategoryId int    `json:"categoryId" doc:"Identifier of the category" example:"1" required:"false" minimum:"1"`
}

type CreateRequestInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	Body           CreateRequestInputBody
}

type CreateRequestOutput struct {
	Body struct {
		Id int `json:"id" doc:"ID of the created category"`
	}
}
