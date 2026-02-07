package users

import (
	"context"
	"planeo/services/core2/internal/domain/user"
	"planeo/services/core2/internal/infra/http/server"
)

type UserHandler struct {
	userService user.Service
}

func NewUserController(userService user.Service) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (u *UserHandler) GetIAMUsers(ctx context.Context, input *GetIAMUsersInput) (*GetIAMUsersOutput, error) {
	users, err := u.userService.GetIAMUsers(ctx, input.OrganizationId, input.Sync)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	response := &GetIAMUsersOutput{}
	response.Body.Users = users
	return response, nil
}

func (u *UserHandler) GetIAMUser(ctx context.Context, input *GetIAMUserInput) (*GetIAMUserOutput, error) {
	user, err := u.userService.GetIAMUserByUuid(ctx, input.OrganizationId, input.Uuid)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	response := &GetIAMUserOutput{}
	response.Body.User = user
	return response, nil
}

func (u *UserHandler) CreateUser(ctx context.Context, input *CreateUserInput) (*struct{}, error) {
	newUser := user.NewUser{
		Email:     input.Body.Email,
		FirstName: input.Body.FirstName,
		LastName:  input.Body.LastName,
		Password:  input.Body.Password,
	}
	err := u.userService.CreateUser(ctx, input.OrganizationId, newUser)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	return nil, nil
}

func (u *UserHandler) UpdateUser(ctx context.Context, input *UpdateUserInput) (*struct{}, error) {
	user := user.UpdateUser{
		Username:        input.Body.Username,
		FirstName:       input.Body.FirstName,
		LastName:        input.Body.LastName,
		Email:           input.Body.Email,
		Totp:            input.Body.Totp,
		Enabled:         input.Body.Enabled,
		EmailVerified:   input.Body.EmailVerified,
		RequiredActions: input.Body.RequiredActions,
	}
	err := u.userService.UpdateUser(ctx, input.OrganizationId, input.Uuid, user)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	return nil, nil
}

func (u *UserHandler) DeleteUser(ctx context.Context, input *DeleteUserInput) (*struct{}, error) {
	err := u.userService.DeleteUser(ctx, input.OrganizationId, input.Uuid)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	return nil, nil
}

func (u *UserHandler) AssignRoles(ctx context.Context, input *PutUserRolesInput) (*struct{}, error) {
	roles := []user.Role{}
	for _, role := range input.Body {
		roles = append(roles, user.Role{
			Id:   role.Id,
			Name: role.Name,
		})
	}
	err := u.userService.AssignRoles(ctx, input.OrganizationId, input.Uuid, roles)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	return nil, nil
}

func (u *UserHandler) GetAvailableRoles(ctx context.Context, input *GetRolesInput) (*GetRolesOutput, error) {
	roles, err := u.userService.GetAvailableRoles(ctx)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	response := &GetRolesOutput{}
	response.Body.Roles = roles
	return response, nil
}

func (u *UserHandler) GetUsers(ctx context.Context, input *GetUsersInput) (*GetUsersOutput, error) {
	users, err := u.userService.GetUsers(ctx, input.OrganizationId)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	response := &GetUsersOutput{}
	response.Body.Users = users
	return response, nil
}

// func (u *UserHandler) InitializeRoutes() {
// 	permissions := middlewares.NewPermissionMiddlewareConfig(u.api, u.config.OauthIssuerUrl(), u.config.KcOauthClientID)
// 	huma.Register(u.api, humaUtils.WithAuth(huma.Operation{
// 		OperationID: "get-users",
// 		Method:      http.MethodGet,
// 		Path:        "/organizations/{organizationId}/iam/users",
// 		Summary:     "[Admin] Get all users from organization",
// 		Tags:        []string{"User"},
// 		Middlewares: huma.Middlewares{permissions.Apply("user", "read")},
// 	}), func(ctx context.Context, input *dto.GetUsersInput) (*dto.GetUsersOutput, error) {
// 		users, err := u.userService.GetUsers(ctx, input.OrganizationId, input.Sync)

// 		if err != nil {
// 			return nil, humaUtils.NewHumaError(err)
// 		}

// 		response := &dto.GetUsersOutput{}
// 		response.Body.Users = users
// 		return response, nil
// 	})

// 	huma.Register(u.api, humaUtils.WithAuth(huma.Operation{
// 		OperationID: "get-user",
// 		Method:      http.MethodGet,
// 		Path:        "/organizations/{organizationId}/iam/users/{iamUserId}",
// 		Summary:     "[Admin] Get single user",
// 		Tags:        []string{"User"},
// 		Middlewares: huma.Middlewares{permissions.Apply("user", "read")},
// 	}), func(ctx context.Context, input *dto.GetUserInput) (*dto.GetUserOutput, error) {
// 		user, err := u.userService.GetUserById(ctx, input.OrganizationId, input.IamUserId)

// 		if err != nil {
// 			return nil, humaUtils.NewHumaError(err)
// 		}

// 		response := &dto.GetUserOutput{}
// 		response.Body.User = user
// 		return response, nil
// 	})

// 	huma.Register(u.api, humaUtils.WithAuth(huma.Operation{
// 		OperationID:   "create-user",
// 		Method:        http.MethodPost,
// 		DefaultStatus: http.StatusCreated,
// 		Path:          "/organizations/{organizationId}/iam/users",
// 		Summary:       "[Admin] Create user",
// 		Tags:          []string{"User"},
// 		Middlewares:   huma.Middlewares{permissions.Apply("user", "create")},
// 	}), func(ctx context.Context, input *dto.CreateUserInput) (*struct{}, error) {

// 		newUser := models.NewUser{
// 			Email:     input.Body.Email,
// 			FirstName: input.Body.FirstName,
// 			LastName:  input.Body.LastName,
// 			Password:  input.Body.Password,
// 		}
// 		err := u.userService.CreateUser(ctx, input.OrganizationId, newUser)

// 		if err != nil {
// 			return nil, humaUtils.NewHumaError(err)
// 		}

// 		return nil, nil
// 	})

// 	huma.Register(u.api, humaUtils.WithAuth(huma.Operation{
// 		OperationID:   "update-user",
// 		Method:        http.MethodPut,
// 		DefaultStatus: http.StatusNoContent,
// 		Path:          "/organizations/{organizationId}/iam/users/{iamUserId}",
// 		Summary:       "[Admin] Update user",
// 		Tags:          []string{"User"},
// 		Middlewares:   huma.Middlewares{permissions.Apply("user", "update")},
// 	}), func(ctx context.Context, input *dto.UpdateUserInput) (*struct{}, error) {

// 		user := models.UpdateUser{
// 			Username:        input.Body.Username,
// 			FirstName:       input.Body.FirstName,
// 			LastName:        input.Body.LastName,
// 			Email:           input.Body.Email,
// 			Totp:            input.Body.Totp,
// 			Enabled:         input.Body.Enabled,
// 			EmailVerified:   input.Body.EmailVerified,
// 			RequiredActions: input.Body.RequiredActions,
// 		}
// 		err := u.userService.UpdateUser(ctx, input.OrganizationId, input.IamUserId, user)

// 		if err != nil {
// 			return nil, humaUtils.NewHumaError(err)
// 		}

// 		return nil, nil
// 	})

// 	huma.Register(u.api, humaUtils.WithAuth(huma.Operation{
// 		OperationID:   "delete-user",
// 		Method:        http.MethodDelete,
// 		DefaultStatus: http.StatusNoContent,
// 		Path:          "/organizations/{organizationId}/iam/users/{iamUserId}",
// 		Summary:       "[Admin] Delete user",
// 		Tags:          []string{"User"},
// 		Middlewares:   huma.Middlewares{permissions.Apply("user", "delete")},
// 	}), func(ctx context.Context, input *dto.DeleteUserInput) (*struct{}, error) {

// 		err := u.userService.DeleteUser(ctx, input.OrganizationId, input.IamUserId)

// 		if err != nil {
// 			return nil, humaUtils.NewHumaError(err)
// 		}

// 		return nil, nil
// 	})

// 	huma.Register(u.api, humaUtils.WithAuth(huma.Operation{
// 		OperationID:   "Assign-user-roles",
// 		Method:        http.MethodPut,
// 		DefaultStatus: http.StatusNoContent,
// 		Path:          "/organizations/{organizationId}/iam/users/{iamUserId}/roles",
// 		Summary:       "[Admin] Assign roles to a user",
// 		Tags:          []string{"User"},
// 		Middlewares:   huma.Middlewares{permissions.Apply("user", "update")},
// 	}), func(ctx context.Context, input *dto.PutUserRolesInput) (*struct{}, error) {

// 		roles := []models.Role{}
// 		for _, role := range input.Body {
// 			roles = append(roles, models.Role{
// 				Id:   role.Id,
// 				Name: role.Name,
// 			})
// 		}
// 		err := u.userService.AssignRoles(ctx, input.OrganizationId, input.IamUserId, roles)

// 		if err != nil {
// 			return nil, humaUtils.NewHumaError(err)
// 		}

// 		return nil, nil
// 	})

// 	huma.Register(u.api, humaUtils.WithAuth(huma.Operation{
// 		OperationID: "get-roles",
// 		Method:      http.MethodGet,
// 		Path:        "/organizations/{organizationId}/iam/roles",
// 		Summary:     "[Admin] Get roles",
// 		Tags:        []string{"Roles"},
// 		Middlewares: huma.Middlewares{permissions.Apply("role", "read")},
// 	}), func(ctx context.Context, input *dto.GetRolesInput) (*dto.GetRolesOutput, error) {

// 		roles, err := u.userService.GetAvailableRoles(ctx)

// 		if err != nil {
// 			return nil, humaUtils.NewHumaError(err)
// 		}

// 		response := &dto.GetRolesOutput{}
// 		response.Body.Roles = roles
// 		return response, nil
// 	})

// 	huma.Register(u.api, humaUtils.WithAuth(huma.Operation{
// 		OperationID: "get-basic-user-information",
// 		Method:      http.MethodGet,
// 		Path:        "/organizations/{organizationId}/users",
// 		Summary:     "Get basic information of users",
// 		Tags:        []string{"User"},
// 		Middlewares: huma.Middlewares{permissions.Apply("userinfo", "read")},
// 	}), func(ctx context.Context, input *dto.GetUserInfoInput) (*dto.GetUserInfoOutput, error) {

// 		users, err := u.userService.GetUsersInformation(ctx, input.OrganizationId)

// 		if err != nil {
// 			return nil, humaUtils.NewHumaError(err)
// 		}

// 		response := &dto.GetUserInfoOutput{}
// 		response.Body.Users = users
// 		return response, nil
// 	})
// }
