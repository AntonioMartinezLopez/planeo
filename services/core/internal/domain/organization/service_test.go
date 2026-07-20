package organization_test

import (
	"context"
	. "planeo/services/core/internal/domain/organization"
	"planeo/services/core/internal/domain/organization/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrganizationService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	ctx := context.Background()

	t.Run("GetOrganizationByIAMId", func(t *testing.T) {
		t.Run("Should return error if repository fails", func(t *testing.T) {
			// Setup
			mockOrganizationRepository := mocks.NewMockOrganizationRepository(t)
			mockOrganizationRepository.EXPECT().GetOrganizationByIAMId(ctx, "local").Return(Organization{}, assert.AnError)
			organizationService := NewService(mockOrganizationRepository)

			// Act
			result, err := organizationService.GetOrganizationByIAMId(ctx, "local")

			// Assert
			assert.NotNil(t, err)
			assert.Equal(t, Organization{}, result)
		})

		t.Run("Should return organization if repository succeeds", func(t *testing.T) {
			// Setup
			expectedOrganization := Organization{Id: 1, Name: "local", IAMOrganizationID: "local"}
			mockOrganizationRepository := mocks.NewMockOrganizationRepository(t)
			mockOrganizationRepository.EXPECT().GetOrganizationByIAMId(ctx, "local").Return(expectedOrganization, nil)
			organizationService := NewService(mockOrganizationRepository)

			// Act
			result, err := organizationService.GetOrganizationByIAMId(ctx, "local")

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, expectedOrganization, result)
		})
	})
}
