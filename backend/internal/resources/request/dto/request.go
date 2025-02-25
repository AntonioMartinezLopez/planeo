package dto

import "planeo/api/internal/resources/request/models"

// GET Requests
type GetRequestsInput struct {
	OrganizationId int  `path:"organizationId" doc:"ID of the organization"`
	GetClosed      bool `query:"getClosed" doc:"Flag describing whether to get also closed requests or not"`
	PageSize       int  `query:"pageSize" required:"true" doc:"Number of requests to be returned"`
	Cursor         int  `query:"cursor" required:"false" doc:"Cursor for pagination"`
}

type GetRequestsOutput struct {
	Body struct {
		Requests   []models.Request `json:"requests" doc:"Array of requests"`
		Limit      int              `json:"limit" doc:"Number of requests to be returned"`
		NextCursor int              `json:"nextCursor" doc:"Cursor for pagination"`
	}
}

// POST Request
type CreateRequestInputBody struct {
	Text      string `json:"text" doc:"Description of the request" example:"Some request text" validate:"required"`
	Name      string `json:"name" doc:"Name of the requester" example:"John Doe" validate:"required"`
	Email     string `json:"email" doc:"Email of the requester" example:"John.Doe@example.com" validate:"required"`
	Address   string `json:"address" doc:"Address of the requester" example:"789 Oak St, Metropolis" validate:"required"`
	Telephone string `json:"telephone" doc:"Telephone number of the requester" example:"1234567" validate:"required"`
	Closed    bool   `json:"closed" doc:"Indicates if the request is closed" example:"false" validate:"required"`
}

type CreateRequestInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	Body           CreateRequestInputBody
}

// UPDATE Request
type UpdateRequestInputBody struct {
	Text       string `json:"text" doc:"Description of the request" example:"Some request text" validate:"required"`
	Name       string `json:"name" doc:"Name of the requester" example:"John Doe" validate:"required"`
	Email      string `json:"email" doc:"Email of the requester" example:"John.Doe@example.com" validate:"required"`
	Address    string `json:"address" doc:"Address of the requester" example:"789 Oak St, Metropolis" validate:"required"`
	Telephone  string `json:"telephone" doc:"Telephone number of the requester" example:"1234567" validate:"required"`
	Closed     bool   `json:"closed" doc:"Indicates if the request is closed" example:"false" validate:"required"`
	CategoryId int    `json:"categoryId" doc:"Identifier of the category" example:"0" validate:"required"`
}

type UpdateRequestInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	RequestId      int `path:"requestId" doc:"ID of the request"`
	Body           UpdateRequestInputBody
}

// DELETE Request
type DeleteRequestInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	RequestId      int `path:"requestId" doc:"ID of the request"`
}
