package categories

import . "planeo/services/core2/internal/domain/category"

type GetCategoriesInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
}

type GetCategoriesOutput struct {
	Body struct {
		Categories []Category `json:"categories" doc:"Array of categories"`
	}
}
