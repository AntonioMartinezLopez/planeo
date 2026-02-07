package user

import (
	"context"
)

type IAM interface {
	GetUsers(ctx context.Context, organizationIamIdentifier string) ([]IAMUser, error)
	GetUserById(ctx context.Context, organizationIamIdentifier string, uuid string) (*IAMUser, error)
	CreateUser(ctx context.Context, organizationIamIdentifier string, newUser NewUser) (*IAMUser, error)
	UpdateUser(ctx context.Context, organizationIamIdentifier string, uuid string, updateUser UpdateUser) error
	DeleteUser(ctx context.Context, organizationIamIdentifier string, uuid string) error
	GetRoles(ctx context.Context) ([]Role, error)
	AssignRolesToUser(ctx context.Context, organizationIamIdentifier string, uuid string, roles []Role) error
}

type UserRepository interface {
	GetIamOrganizationIdentifier(ctx context.Context, organizationId int) (string, error)
	GetUsers(ctx context.Context, organizationId int) ([]User, error)
	SyncUsers(ctx context.Context, organizationId int, users []IAMUser) error
	CreateUser(ctx context.Context, organizationId int, uuid string, user NewUser) error
	DeleteUser(ctx context.Context, organizationId int, uuid string) error
	UpdateUser(ctx context.Context, organizationId int, uuid string, user UpdateUser) error
}

type Service interface {
	GetIAMUsers(ctx context.Context, organizationId int, sync bool) ([]IAMUser, error)
	CreateUser(ctx context.Context, organizationId int, newUser NewUser) error
	DeleteUser(ctx context.Context, organizationId int, uuid string) error
	UpdateUser(ctx context.Context, organizationId int, uuid string, user UpdateUser) error
	GetAvailableRoles(ctx context.Context) ([]Role, error)
	AssignRoles(ctx context.Context, organizationId int, uuid string, roles []Role) error
	GetUsers(ctx context.Context, organizationId int) ([]User, error)
	GetIAMUserByUuid(ctx context.Context, organizationId int, uuid string) (*IAMUser, error)
}
