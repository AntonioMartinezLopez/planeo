package user

import (
	"context"
)

type IAMInterface interface {
	GetUsers(ctx context.Context, organizationIamIdentifier string) ([]IAMUser, error)
	GetUserById(ctx context.Context, organizationIamIdentifier string, userId string) (*IAMUser, error)
	CreateUser(ctx context.Context, organizationIamIdentifier string, newUser NewUser) (*IAMUser, error)
	UpdateUser(ctx context.Context, organizationIamIdentifier string, userId string, updateUser UpdateUser) error
	DeleteUser(ctx context.Context, organizationIamIdentifier string, userId string) error
	GetRoles(ctx context.Context) ([]Role, error)
	AssignRolesToUser(ctx context.Context, organizationIamIdentifier string, userId string, roles []Role) error
}

type UserRepositoryInterface interface {
	GetIamOrganizationIdentifier(ctx context.Context, organizationId int) (string, error)
	GetUsersInformation(ctx context.Context, organizationId int) ([]User, error)
	SyncUsers(ctx context.Context, organizationId int, users []User) error
	CreateUser(ctx context.Context, organizationId int, user User) error
	DeleteUser(ctx context.Context, organizationId int, userId string) error
	UpdateUser(ctx context.Context, organizationId int, userId string, user User) error
}
