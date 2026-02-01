package categories

import "planeo/services/core2/internal/domain/category"

type GetCategoriesInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
}

type GetCategoriesOutput struct {
	Body struct {
		Categories []category.Category `json:"categories" doc:"Array of categories"`
	}
}
