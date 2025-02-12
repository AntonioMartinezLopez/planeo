package user

import (
	"context"
	appError "planeo/api/internal/errors"
	"planeo/api/internal/resources/user/dto"
	"planeo/api/internal/resources/user/models"
)

type IAMInterface interface {
	GetUsers(organizationIamIdentifier string) ([]models.User, error)
	GetUserById(organizationIamIdentifier string, userId string) (*models.User, error)
	CreateUser(organizationIamIdentifier string, createUserInput dto.CreateUserInputBody) (*models.User, error)
	UpdateUser(organizationIamIdentifier string, userId string, updateUserInput dto.UpdateUserInputBody) error
	DeleteUser(organizationIamIdentifier string, userId string) error
	GetRoles() ([]models.Role, error)
	AssignRolesToUser(organizationIamIdentifier string, userId string, roles []dto.PutUserRoleInputBody) error
}

type UserRepositoryInterface interface {
	GetOrganizationIamIdentifier(ctx context.Context, organizationId int) (string, error)
	GetUsersInformation(ctx context.Context, organizationId int) ([]models.BasicUserInformation, error)
	SyncUsers(ctx context.Context, organizationId int, users []models.User) error
	CreateUser(ctx context.Context, organizationId int, user models.User) error
	DeleteUser(ctx context.Context, organizationId int, userId string) error
	UpdateUser(ctx context.Context, organizationId int, userId string, user models.User) error
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

func (s *UserService) GetUsers(ctx context.Context, organizationId int, sync bool) ([]models.User, error) {
	organizationIamIdentifier, err := s.userRepository.GetOrganizationIamIdentifier(ctx, organizationId)

	if err != nil {
		return nil, err
	}

	users, err := s.iamService.GetUsers(organizationIamIdentifier)

	if err != nil {
		return nil, err
	}

	if sync {
		err := s.userRepository.SyncUsers(ctx, organizationId, users)

		if err != nil {
			return nil, appError.New(appError.InternalError, "Something went wrong", err)
		}
	}

	return users, nil
}

func (s *UserService) CreateUser(ctx context.Context, organizationId int, createUserInput dto.CreateUserInputBody) error {

	organizationIamIdentifier, err := s.userRepository.GetOrganizationIamIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	user, err := s.iamService.CreateUser(organizationIamIdentifier, createUserInput)

	if err != nil {
		return err
	}

	err = s.userRepository.CreateUser(ctx, organizationId, *user)

	if err != nil {
		s.iamService.DeleteUser(organizationIamIdentifier, user.Id)
		return err
	}

	return nil
}

func (s *UserService) DeleteUser(ctx context.Context, organizationId int, userId string) error {

	organizationIamIdentifier, err := s.userRepository.GetOrganizationIamIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	err = s.iamService.DeleteUser(organizationIamIdentifier, userId)

	if err != nil {
		return err
	}

	err = s.userRepository.DeleteUser(ctx, organizationId, userId)

	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) UpdateUser(ctx context.Context, organizationId int, userId string, user dto.UpdateUserInputBody) error {

	organizationIamIdentifier, err := s.userRepository.GetOrganizationIamIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	err = s.iamService.UpdateUser(organizationIamIdentifier, userId, user)

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

func (s *UserService) AssignRoles(ctx context.Context, organizationId int, userId string, roles []dto.PutUserRoleInputBody) error {
	organizationIamIdentifier, err := s.userRepository.GetOrganizationIamIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	return s.iamService.AssignRolesToUser(organizationIamIdentifier, userId, roles)
}

func (s *UserService) GetUserById(ctx context.Context, organizationId int, userId string) (*models.User, error) {
	organizationIamIdentifier, err := s.userRepository.GetOrganizationIamIdentifier(ctx, organizationId)

	if err != nil {
		return nil, err
	}

	return s.iamService.GetUserById(organizationIamIdentifier, userId)
}

func (s *UserService) GetUsersInformation(ctx context.Context, organizationId int) ([]models.BasicUserInformation, error) {
	user, err := s.userRepository.GetUsersInformation(ctx, organizationId)

	if err != nil {
		return nil, err
	}

	return user, nil
}
