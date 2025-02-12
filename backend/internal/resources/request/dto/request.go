package dto

import "planeo/api/internal/resources/request/models"

// GET Requests
type GetRequestsInput struct {
	OrganizationId string `path:"organizationId" doc:"ID of the organization"`
	GetClosed      bool   `query:"getClosed" doc:"Flag describing whether to get also closed requests or not"`
	PageSize       int    `query:"pageSize" required:"true" doc:"Number of requests to be returned"`
	Cursor         int    `query:"cursor" required:"false" doc:"Cursor for pagination"`
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
	Text  string `json:"text" doc:"Description of the request" validate:"required"`
	Name  string `json:"name" doc:"Name of the requester" validate:"required"`
	Email string `json:"email" doc:"Email of the requester" validate:"required"`
}

type CreateRequestInput struct {
	OrganizationId string `path:"organizationId" doc:"ID of the organization"`
	Body           CreateRequestInputBody
}

type CreateRequestOutput struct {
	Body struct {
		Success bool
	}
}

// UPDATE Request
type UpdateRequestInputBody struct {
	Text       string `json:"text" doc:"Description of the request" validate:"required"`
	Name       string `json:"name" doc:"Name of the requester" validate:"required"`
	Email      string `json:"email" doc:"Email of the requester" validate:"required"`
	Closed     bool   `json:"closed" doc:"Indicates if the request is closed" validate:"required"`
	CategoryId string `json:"categoryId" doc:"Identifier of the category" validate:"required"`
}

type UpdateRequestInput struct {
	GetRequestsInput
	RequestId int `path:"requestId" doc:"ID of the request"`
	Body      UpdateRequestInputBody
}

type UpdateRequestOutput struct {
	Body struct {
		Success bool
	}
}

// DELETE Request
type DeleteRequestInput struct {
	GetRequestsInput
	RequestId int `path:"requestId" doc:"ID of the request"`
}

type DeleteRequestOutput struct {
	Body struct {
		Success bool
	}
}
