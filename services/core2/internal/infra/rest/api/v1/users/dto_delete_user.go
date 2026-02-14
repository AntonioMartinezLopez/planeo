package users

type DeleteUserInput struct {
	OrganizationId int    `path:"organizationId" doc:"ID of the organization"`
	Uuid           string `path:"uuid" doc:"IAM uuid of the user to be deleted"`
}
