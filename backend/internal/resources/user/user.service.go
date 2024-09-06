package user

import (
	"errors"
	"fmt"
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) GetUser(id string) (string, error) {
	if id == "1" {
		return fmt.Sprintf("GetUser endpoint. Id called: %s", id), errors.New(
			"instance not found",
		)
	}
	return fmt.Sprintf("GetUser endpoint. Id called: %s", id), nil
}

func (s *UserService) CreateUser() string {
	return "CreateUser endpoint"
}

func (s *UserService) UpdateUser(id string) string {
	return "UpdateUser endpoint"
}

func (s *UserService) DeleteUser(id string) string {
	return "DeleteUser endpoint"
}

func (s *UserService) GetUsers() string {
	return "GetUsers endpoint"
}
