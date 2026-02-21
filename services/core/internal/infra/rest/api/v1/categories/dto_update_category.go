package categories

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
