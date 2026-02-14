package users

import (
	"context"
	"planeo/services/core2/internal/domain/user"
	. "planeo/services/core2/internal/infra/rest/api"
)

type UserHandler struct {
	userService user.Service
}

func NewUserHandler(userService user.Service) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (u *UserHandler) GetIAMUsers(ctx context.Context, input *GetIAMUsersInput) (*GetIAMUsersOutput, error) {
	users, err := u.userService.GetIAMUsers(ctx, input.OrganizationId, input.Sync)

	if err != nil {
		return nil, NewHTTPError(err)
	}

	response := &GetIAMUsersOutput{}
	response.Body.Users = users
	return response, nil
}

func (u *UserHandler) GetIAMUser(ctx context.Context, input *GetIAMUserInput) (*GetIAMUserOutput, error) {
	user, err := u.userService.GetIAMUserByUuid(ctx, input.OrganizationId, input.Uuid)

	if err != nil {
		return nil, NewHTTPError(err)
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
		return nil, NewHTTPError(err)
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
		return nil, NewHTTPError(err)
	}

	return nil, nil
}

func (u *UserHandler) DeleteUser(ctx context.Context, input *DeleteUserInput) (*struct{}, error) {
	err := u.userService.DeleteUser(ctx, input.OrganizationId, input.Uuid)

	if err != nil {
		return nil, NewHTTPError(err)
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
		return nil, NewHTTPError(err)
	}

	return nil, nil
}

func (u *UserHandler) GetAvailableRoles(ctx context.Context, input *GetRolesInput) (*GetRolesOutput, error) {
	roles, err := u.userService.GetAvailableRoles(ctx)

	if err != nil {
		return nil, NewHTTPError(err)
	}

	response := &GetRolesOutput{}
	response.Body.Roles = roles
	return response, nil
}

func (u *UserHandler) GetUsers(ctx context.Context, input *GetUsersInput) (*GetUsersOutput, error) {
	users, err := u.userService.GetUsers(ctx, input.OrganizationId)

	if err != nil {
		return nil, NewHTTPError(err)
	}

	response := &GetUsersOutput{}
	response.Body.Users = users
	return response, nil
}
