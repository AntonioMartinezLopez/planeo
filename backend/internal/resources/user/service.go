package user

import (
	"context"
	"planeo/api/internal/resources/user/dto"
	"planeo/api/internal/resources/user/models"
)

type IAMInterface interface {
	GetUsers(organizationId string) ([]models.User, error)
	GetUserById(organizationId string, userId string) (*models.User, error)
	CreateUser(organizationId string, createUserInput dto.CreateUserInputBody) (*models.User, error)
	UpdateUser(organizationId string, userId string, updateUserInput dto.UpdateUserInputBody) error
	DeleteUser(organizationId string, userId string) error
	GetRoles() ([]models.Role, error)
	AssignRolesToUser(organizationId string, userId string, roles []dto.PutUserRoleInputBody) error
}

type UserRepositoryInterface interface {
	GetUsersInformation(ctx context.Context, organizationId string) ([]models.BasicUserInformation, error)
	SyncUsers(ctx context.Context, organizationId string, users []models.User) error
	CreateUser(ctx context.Context, organizationId string, user models.User) error
	DeleteUser(ctx context.Context, organizationId string, userId string) error
	UpdateUser(ctx context.Context, organizationId string, userId string, user models.User) error
}

type UserService struct {
	iamService     IAMInterface
	userRepository UserRepositoryInterface
}

func NewUserService(userRepository UserRepositoryInterface, iamService IAMInterface) *UserService {
	return &UserService{
		iamService:     iamService,
		userRepository: userRepository,
	}
}

func (s *UserService) GetUsers(ctx context.Context, organizationId string, sync bool) ([]models.User, error) {
	return s.iamService.GetUsers(organizationId)
}

func (s *UserService) CreateUser(ctx context.Context, organizationId string, createUserInput dto.CreateUserInputBody) error {

	user, err := s.iamService.CreateUser(organizationId, createUserInput)

	if err != nil {
		return err
	}

	err = s.userRepository.CreateUser(ctx, organizationId, *user)

	if err != nil {
		s.iamService.DeleteUser(organizationId, user.Id)
		return err
	}

	return nil
}

func (s *UserService) DeleteUser(ctx context.Context, organizationId string, userId string) error {

	err := s.iamService.DeleteUser(organizationId, userId)

	if err != nil {
		return err
	}

	err = s.userRepository.DeleteUser(ctx, organizationId, userId)

	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) UpdateUser(ctx context.Context, organizationId string, userId string, user dto.UpdateUserInputBody) error {

	err := s.iamService.UpdateUser(organizationId, userId, user)

	if err != nil {
		return err
	}

	err = s.userRepository.UpdateUser(ctx, organizationId, userId, models.User{
		Id:        userId,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	})

	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) GetAvailableRoles(ctx context.Context) ([]models.Role, error) {

	roles, err := s.iamService.GetRoles()

	if err != nil {
		return nil, err
	}

	return roles, nil
}

func (s *UserService) AssignRoles(ctx context.Context, organizationId string, userId string, roles []dto.PutUserRoleInputBody) error {
	return s.iamService.AssignRolesToUser(organizationId, userId, roles)
}

func (s *UserService) GetUserById(ctx context.Context, organizationId string, userId string) (*models.User, error) {
	return s.iamService.GetUserById(organizationId, userId)
}

func (s *UserService) GetUsersInformation(ctx context.Context, organizationId string) ([]models.BasicUserInformation, error) {
	user, err := s.userRepository.GetUsersInformation(ctx, organizationId)

	if err != nil {
		return nil, err
	}

	return user, nil
}
