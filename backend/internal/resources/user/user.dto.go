package user

// GET users
type GetUsersOutput struct {
	Body struct {
		Users []User `json:"users" doc:"Array of users managed in organization"`
	}
}

type GetUsersInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}

// GET user
type GetUserOutput struct {
	Body struct {
		User *UserWithRoles `json:"user" doc:"Information about a user managed in given auth system"`
	}
}

type GetUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of a user"`
}

// POST user
type CreateUserOutput struct {
	Body struct {
		Success bool
	}
}

type CreateUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	Body         CreateUserData
	RawBody      []byte
}

// UPDATE user
type UpdateUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of the user to be deleted"`
	Body         User
}

type UpdateUserOutput struct {
	CreateUserOutput
}

// DELETE user
type DeleteUserOutput struct {
	CreateUserOutput
}

type DeleteUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of the user to be deleted"`
}

// PUT user/roles
type PutUserRoleInputBody struct {
	Role
}
type PutUserRolesInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of the user to be deleted"`
	Body         struct {
		Roles []PutUserRoleInputBody `json:"roles" doc:"Array of role representations"`
	}
}

type PutUserRoleOutput struct {
	Body struct {
		Success bool
	}
}

// GET roles
type GetRolesInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}

type GetRolesOutput struct {
	Body struct {
		Roles []Role `json:"roles" doc:"Array of roles that can be assigned to users"`
	}
}
