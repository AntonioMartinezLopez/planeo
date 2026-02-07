package user_test

import (
	"context"
	. "planeo/services/core2/internal/domain/user"
	"planeo/services/core2/internal/domain/user/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserService(t *testing.T) {

	if !testing.Short() {
		t.Skip()
	}

	testOrganizationId := 1
	testOrganizationIamId := "local"
	userInput := NewUser{
		FirstName: "test",
		LastName:  "test",
		Email:     "test",
		Password:  "test",
	}
	user := IAMUser{
		Uuid:            "test",
		Username:        "test",
		FirstName:       "test",
		LastName:        "test",
		Email:           "test",
		Totp:            true,
		Enabled:         true,
		EmailVerified:   true,
		RequiredActions: []RequiredAction{},
	}
	ctx := context.Background()

	t.Run("CreateUser", func(t *testing.T) {
		t.Run("Should return error if CreateUser fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().CreateUser(ctx, testOrganizationIamId, userInput).Return(nil, assert.AnError)
			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)
			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.CreateUser(ctx, testOrganizationId, userInput)

			// Assert
			assert.NotNil(t, result)
			mockUserRepository.AssertNotCalled(t, "CreateUser")

		})

		t.Run("Should return error if CreateUser fails after repository layer error", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().CreateUser(ctx, testOrganizationIamId, userInput).Return(&user, nil)
			mockIAMInterface.EXPECT().DeleteUser(ctx, testOrganizationIamId, user.Uuid).Return(nil)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)
			mockUserRepository.EXPECT().CreateUser(ctx, testOrganizationId, user.Uuid, userInput).Return(assert.AnError)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.CreateUser(ctx, testOrganizationId, userInput)

			// Assert
			assert.NotNil(t, result)
		})

		t.Run("Should return nil if CreateUser succeeds", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockUserRepository := mocks.NewMockUserRepository(t)
			mockIAMInterface.EXPECT().CreateUser(ctx, testOrganizationIamId, userInput).Return(&user, nil)

			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)
			mockUserRepository.EXPECT().CreateUser(ctx, testOrganizationId, user.Uuid, userInput).Return(nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.CreateUser(ctx, testOrganizationId, userInput)

			// Assert
			assert.Nil(t, result)
		})

	})

	t.Run("DeleteUser", func(t *testing.T) {
		t.Run("Should return error if DeleteUser fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().DeleteUser(ctx, testOrganizationIamId, user.Uuid).Return(assert.AnError)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.DeleteUser(ctx, testOrganizationId, user.Uuid)

			// Assert
			assert.NotNil(t, result)
			mockUserRepository.AssertNotCalled(t, "DeleteUser")

		})
		t.Run("Should return nil if DeleteUser succeeds", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().DeleteUser(ctx, testOrganizationIamId, user.Uuid).Return(nil)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().DeleteUser(ctx, testOrganizationId, user.Uuid).Return(nil)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.DeleteUser(ctx, testOrganizationId, user.Uuid)

			// Assert
			assert.Nil(t, result)
		})
	})

	t.Run("UpdateUser", func(t *testing.T) {
		t.Run("Should return error if UpdateUser fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().UpdateUser(ctx, testOrganizationIamId, user.Uuid, UpdateUser{}).Return(assert.AnError)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.UpdateUser(ctx, testOrganizationId, user.Uuid, UpdateUser{})

			// Assert
			assert.NotNil(t, result)
			mockUserRepository.AssertNotCalled(t, "UpdateUser")
		})
		t.Run("Should return nil if UpdateUser succeeds", func(t *testing.T) {
			// Setup
			updatedUser := UpdateUser{
				Username:        user.Username,
				FirstName:       user.FirstName,
				LastName:        user.LastName,
				Email:           user.Email,
				Totp:            user.Totp,
				Enabled:         user.Enabled,
				EmailVerified:   user.EmailVerified,
				RequiredActions: user.RequiredActions,
			}
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().UpdateUser(ctx, testOrganizationIamId, user.Uuid, updatedUser).Return(nil)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)
			mockUserRepository.EXPECT().UpdateUser(ctx, testOrganizationId, user.Uuid, updatedUser).Return(nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.UpdateUser(ctx, testOrganizationId, user.Uuid, updatedUser)

			// Assert
			assert.Nil(t, result)
		})
	})

	t.Run("GetRoles", func(t *testing.T) {
		t.Run("Should return error if GetRoles fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().GetRoles(ctx).Return(nil, assert.AnError)
			mockUserRepository := mocks.NewMockUserRepository(t)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetAvailableRoles(ctx)

			// Assert
			assert.Nil(t, result)
			assert.NotNil(t, err)
		})
		t.Run("Should return roles if GetRoles succeeds", func(t *testing.T) {
			// Setup
			roles := []Role{
				{
					Id:   "test",
					Name: "test",
				},
			}
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().GetRoles(ctx).Return(roles, nil)
			mockUserRepository := mocks.NewMockUserRepository(t)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetAvailableRoles(ctx)

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, roles, result)
		})
	})

	t.Run("GetUser", func(t *testing.T) {
		t.Run("Should return error if GetUser fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().GetUserById(ctx, testOrganizationIamId, user.Uuid).Return(nil, assert.AnError)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetIAMUserByUuid(ctx, testOrganizationId, user.Uuid)

			// Assert
			assert.Nil(t, result)
			assert.NotNil(t, err)
		})
		t.Run("Should return user if GetUser succeeds", func(t *testing.T) {
			userWithRoles := user
			userWithRoles.Roles = []Role{
				{
					Id:   "test",
					Name: "test",
				}}

			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().GetUserById(ctx, testOrganizationIamId, user.Uuid).Return(&userWithRoles, nil)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetIAMUserByUuid(ctx, testOrganizationId, user.Uuid)

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, &userWithRoles, result)
		})
	})

	t.Run("GetIAMUsers", func(t *testing.T) {
		t.Run("Should return error if GetIAMUsers fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().GetUsers(ctx, testOrganizationIamId).Return(nil, assert.AnError)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetIAMUsers(ctx, testOrganizationId, false)

			// Assert
			assert.Nil(t, result)
			assert.NotNil(t, err)
		})
		t.Run("Should return users if GetIAMUsers succeeds", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().GetUsers(ctx, testOrganizationIamId).Return([]IAMUser{user}, nil)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetIAMUsers(ctx, testOrganizationId, false)

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, []IAMUser{user}, result)
		})
		t.Run("Should sync users if sync is true", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().GetUsers(ctx, testOrganizationIamId).Return([]IAMUser{user}, nil)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)
			mockUserRepository.EXPECT().SyncUsers(ctx, testOrganizationId, []IAMUser{user}).Return(nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetIAMUsers(ctx, testOrganizationId, true)

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, []IAMUser{user}, result)
		})
	})

	t.Run("GetUsers", func(t *testing.T) {
		t.Run("Should return error if GetUsers fails", func(t *testing.T) {
			// Setup
			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetUsers(ctx, testOrganizationId).Return(nil, assert.AnError)

			userService := NewService(mockUserRepository, nil)

			// Act
			result, err := userService.GetUsers(ctx, testOrganizationId)

			// Assert
			assert.Nil(t, result)
			assert.NotNil(t, err)
		})
		t.Run("Should return users if GetUsers succeeds", func(t *testing.T) {
			expectedUsers := []User{
				{
					Id:    1,
					UUID:  "test",
					Email: "test",
				},
			}
			// Setup
			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetUsers(ctx, testOrganizationId).Return(expectedUsers, nil)

			userService := NewService(mockUserRepository, nil)

			// Act
			result, err := userService.GetUsers(ctx, testOrganizationId)

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, expectedUsers, result)
		})
	})

	t.Run("AssignRoles", func(t *testing.T) {
		roles := []Role{
			{
				Id:   "role1",
				Name: "Role 1",
			},
			{
				Id:   "role2",
				Name: "Role 2",
			},
		}

		t.Run("Should return error if AssignRolesToUser fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().AssignRolesToUser(ctx, testOrganizationIamId, user.Uuid, roles).Return(assert.AnError)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.AssignRoles(ctx, testOrganizationId, user.Uuid, roles)

			// Assert
			assert.NotNil(t, result)
		})
		t.Run("Should return nil if AssignRolesToUser succeeds", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAM(t)
			mockIAMInterface.EXPECT().AssignRolesToUser(ctx, testOrganizationIamId, user.Uuid, roles).Return(nil)

			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.AssignRoles(ctx, testOrganizationId, user.Uuid, roles)

			// Assert
			assert.Nil(t, result)
		})
	})
}
