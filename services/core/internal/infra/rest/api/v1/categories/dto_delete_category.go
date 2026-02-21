package categories

type DeleteCategoryInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	CategoryId     int `path:"categoryId" doc:"ID of the category"`
}
