package categories

type CreateCategoryInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	Body           CreateCategoryInputBody
}

type CreateCategoryOutput struct {
	Body struct {
		Id int `json:"id" doc:"ID of the created category"`
	}
}

type CreateCategoryInputBody struct {
	Label            string `json:"label"`
	Color            string `json:"color"`
	LabelDescription string `json:"labelDescription"`
}
