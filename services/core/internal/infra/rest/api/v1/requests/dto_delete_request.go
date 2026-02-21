package requests

type DeleteRequestInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	RequestId      int `path:"requestId" doc:"ID of the request"`
}
