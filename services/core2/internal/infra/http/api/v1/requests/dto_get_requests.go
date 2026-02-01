package requests

import . "planeo/services/core2/internal/domain/request"

type GetRequestsInput struct {
	OrganizationId     int   `path:"organizationId" doc:"ID of the organization"`
	GetClosed          bool  `query:"getClosed" doc:"Flag describing whether to get also closed requests or not"`
	PageSize           int   `query:"pageSize" required:"true" doc:"Number of requests to be returned"`
	Cursor             int   `query:"cursor" required:"false" doc:"Cursor for pagination"`
	SelectedCategories []int `query:"selectedCategories,explode" required:"false" doc:"Array of category IDs to filter requests by"`
}

type GetRequestsOutput struct {
	Body struct {
		Requests   []Request `json:"requests" doc:"Array of requests"`
		Limit      int       `json:"limit" doc:"Number of requests to be returned"`
		NextCursor int       `json:"nextCursor" doc:"Cursor for pagination"`
	}
}
