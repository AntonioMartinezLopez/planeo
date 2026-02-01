package requests

type UpdateRequestInputBody struct {
	Subject    string `json:"subject" doc:"Subject of the request" example:"Some request subject"`
	Text       string `json:"text" doc:"Description of the request" example:"Some request text"`
	Name       string `json:"name" doc:"Name of the requester" example:"John Doe"`
	Email      string `json:"email" doc:"Email of the requester" example:"John.Doe@example.com"`
	Address    string `json:"address" doc:"Address of the requester" example:"789 Oak St, Metropolis"`
	Telephone  string `json:"telephone" doc:"Telephone number of the requester" example:"1234567"`
	Closed     bool   `json:"closed" doc:"Indicates if the request is closed" example:"false"`
	CategoryId int    `json:"categoryId" doc:"Identifier of the category" example:"1" minimum:"1"`
}

type UpdateRequestInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	RequestId      int `path:"requestId" doc:"ID of the request"`
	Body           UpdateRequestInputBody
}
