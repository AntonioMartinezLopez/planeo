package organizations

import (
	. "planeo/services/core2/internal/domain/organization"
)

// GetOrganizationsInput defines the input for getting organizations for the authenticated user
// The user's sub (subject) claim is read directly from the JWT token, not from query params
type GetOrganizationsInput struct {
	// No query params needed - sub is read from JWT claims in the controller
}

// GetOrganizationsOutput defines the output for getting organizations
type GetOrganizationsOutput struct {
	Body []Organization `json:"organizations" doc:"Array of organizations the user belongs to"`
}
