package dto

import "planeo/services/core/internal/resources/category/models"

type GetCategoriesInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
}

type GetCategoriesOutput struct {
	Body struct {
		Categories []models.Category `json:"categories" doc:"Array of categories"`
	}
}

type CreateCategoryInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	Body           CreateCategoryInputBody
}

type CreateCategoryInputBody struct {
	Label            string `json:"label"`
	Color            string `json:"color"`
	LabelDescription string `json:"labelDescription"`
}

type UpdateCategoryInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	CategoryId     int `path:"categoryId" doc:"ID of the category"`
	Body           UpdateCategoryInputBody
}

type UpdateCategoryInputBody struct {
	Label            string `json:"label"`
	Color            string `json:"color"`
	LabelDescription string `json:"labelDescription"`
}

type DeleteCategoryInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	CategoryId     int `path:"categoryId" doc:"ID of the category"`
}
