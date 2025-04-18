package user

import (
	"context"
	"planeo/services/core/internal/resources/user/mocks"
	"planeo/services/core/internal/resources/user/models"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserService(t *testing.T) {

	if !testing.Short() {
		t.Skip()
	}

	testOrganizationId := 1
	testOrganizationIamId := "local"
	userInput := models.NewUser{
		FirstName: "test",
		LastName:  "test",
		Email:     "test",
		Password:  "test",
	}
	user := models.User{
		Id:              "test",
		Username:        "test",
		FirstName:       "test",
		LastName:        "test",
		Email:           "test",
		Totp:            true,
		Enabled:         true,
		EmailVerified:   true,
		RequiredActions: []models.RequiredAction{},
	}
	ctx := context.Background()

	t.Run("CreateUser", func(t *testing.T) {
		t.Run("Should return error if CreateUser fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().CreateUser(ctx, testOrganizationIamId, userInput).Return(nil, assert.AnError)
			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.CreateUser(ctx, testOrganizationId, userInput)

			// Assert
			assert.NotNil(t, result)
			mockUserRepository.AssertNotCalled(t, "CreateUser")

		})

		t.Run("Should return error if CreateUser fails after repository layer error", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().CreateUser(ctx, testOrganizationIamId, userInput).Return(&user, nil)
			mockIAMInterface.EXPECT().DeleteUser(ctx, testOrganizationIamId, user.Id).Return(nil)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)
			mockUserRepository.EXPECT().CreateUser(ctx, testOrganizationId, user).Return(assert.AnError)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.CreateUser(ctx, testOrganizationId, userInput)

			// Assert
			assert.NotNil(t, result)
		})

		t.Run("Should return nil if CreateUser succeeds", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockIAMInterface.EXPECT().CreateUser(ctx, testOrganizationIamId, userInput).Return(&user, nil)

			mockUserRepository.EXPECT().CreateUser(ctx, testOrganizationId, user).Return(nil)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.CreateUser(ctx, testOrganizationId, userInput)

			// Assert
			assert.Nil(t, result)
		})

	})

	t.Run("DeleteUser", func(t *testing.T) {
		t.Run("Should return error if DeleteUser fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().DeleteUser(ctx, testOrganizationIamId, user.Id).Return(assert.AnError)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.DeleteUser(ctx, testOrganizationId, user.Id)

			// Assert
			assert.NotNil(t, result)
			mockUserRepository.AssertNotCalled(t, "DeleteUser")

		})
		t.Run("Should return nil if DeleteUser succeeds", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().DeleteUser(ctx, testOrganizationIamId, user.Id).Return(nil)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().DeleteUser(ctx, testOrganizationId, user.Id).Return(nil)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.DeleteUser(ctx, testOrganizationId, user.Id)

			// Assert
			assert.Nil(t, result)
		})
	})

	t.Run("UpdateUser", func(t *testing.T) {
		t.Run("Should return error if UpdateUser fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().UpdateUser(ctx, testOrganizationIamId, user.Id, models.UpdateUser{}).Return(assert.AnError)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.UpdateUser(ctx, testOrganizationId, user.Id, models.UpdateUser{})

			// Assert
			assert.NotNil(t, result)
			mockUserRepository.AssertNotCalled(t, "UpdateUser")
		})
		t.Run("Should return nil if UpdateUser succeeds", func(t *testing.T) {
			// Setup
			updatedUser := models.UpdateUser{
				Username:        user.Username,
				FirstName:       user.FirstName,
				LastName:        user.LastName,
				Email:           user.Email,
				Totp:            user.Totp,
				Enabled:         user.Enabled,
				EmailVerified:   user.EmailVerified,
				RequiredActions: user.RequiredActions,
			}
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().UpdateUser(ctx, testOrganizationIamId, user.Id, updatedUser).Return(nil)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)
			mockUserRepository.EXPECT().UpdateUser(ctx, testOrganizationId, user.Id, models.User{
				Id:        user.Id,
				Username:  updatedUser.Username,
				FirstName: updatedUser.FirstName,
				LastName:  updatedUser.LastName,
				Email:     updatedUser.Email,
			}).Return(nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result := userService.UpdateUser(ctx, testOrganizationId, user.Id, updatedUser)

			// Assert
			assert.Nil(t, result)
		})
	})

	t.Run("GetRoles", func(t *testing.T) {
		t.Run("Should return error if GetRoles fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().GetRoles(ctx).Return(nil, assert.AnError)
			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetAvailableRoles(ctx)

			// Assert
			assert.Nil(t, result)
			assert.NotNil(t, err)
		})
		t.Run("Should return roles if GetRoles succeeds", func(t *testing.T) {
			// Setup
			roles := []models.Role{
				{
					Id:   "test",
					Name: "test",
				},
			}
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().GetRoles(ctx).Return(roles, nil)
			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

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
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().GetUserById(ctx, testOrganizationIamId, user.Id).Return(nil, assert.AnError)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetUserById(ctx, testOrganizationId, user.Id)

			// Assert
			assert.Nil(t, result)
			assert.NotNil(t, err)
		})
		t.Run("Should return user if GetUser succeeds", func(t *testing.T) {
			userWithRoles := user
			userWithRoles.Roles = []models.Role{
				{
					Id:   "test",
					Name: "test",
				}}

			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().GetUserById(ctx, testOrganizationIamId, user.Id).Return(&userWithRoles, nil)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetUserById(ctx, testOrganizationId, user.Id)

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, &userWithRoles, result)
		})
	})

	t.Run("GetUsers", func(t *testing.T) {
		t.Run("Should return error if GetUsers fails", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().GetUsers(ctx, testOrganizationIamId).Return(nil, assert.AnError)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetUsers(ctx, testOrganizationId, false)

			// Assert
			assert.Nil(t, result)
			assert.NotNil(t, err)
		})
		t.Run("Should return users if GetUsers succeeds", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().GetUsers(ctx, testOrganizationIamId).Return([]models.User{user}, nil)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetUsers(ctx, testOrganizationId, false)

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, []models.User{user}, result)
		})
		t.Run("Should sync users if sync is true", func(t *testing.T) {
			// Setup
			mockIAMInterface := mocks.NewMockIAMInterface(t)
			mockIAMInterface.EXPECT().GetUsers(ctx, testOrganizationIamId).Return([]models.User{user}, nil)

			mockUserRepository := mocks.NewMockUserRepositoryInterface(t)
			mockUserRepository.EXPECT().GetIamOrganizationIdentifier(ctx, testOrganizationId).Return(testOrganizationIamId, nil)
			mockUserRepository.EXPECT().SyncUsers(ctx, testOrganizationId, []models.User{user}).Return(nil)

			userService := NewUserService(mockUserRepository, mockIAMInterface)

			// Act
			result, err := userService.GetUsers(ctx, testOrganizationId, true)

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, []models.User{user}, result)
		})
	})
}
