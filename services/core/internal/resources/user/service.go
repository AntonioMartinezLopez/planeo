package user

import (
	"context"
	appError "planeo/libs/errors"
	"planeo/services/core/internal/resources/user/models"
)

type IAMInterface interface {
	GetUsers(ctx context.Context, organizationIamIdentifier string) ([]models.User, error)
	GetUserById(ctx context.Context, organizationIamIdentifier string, userId string) (*models.User, error)
	CreateUser(ctx context.Context, organizationIamIdentifier string, newUser models.NewUser) (*models.User, error)
	UpdateUser(ctx context.Context, organizationIamIdentifier string, userId string, updateUser models.UpdateUser) error
	DeleteUser(ctx context.Context, organizationIamIdentifier string, userId string) error
	GetRoles(ctx context.Context) ([]models.Role, error)
	AssignRolesToUser(ctx context.Context, organizationIamIdentifier string, userId string, roles []models.Role) error
}

type UserRepositoryInterface interface {
	GetIamOrganizationIdentifier(ctx context.Context, organizationId int) (string, error)
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
	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return nil, err
	}

	users, err := s.iamService.GetUsers(ctx, organizationIamIdentifier)

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

func (s *UserService) CreateUser(ctx context.Context, organizationId int, newUser models.NewUser) error {

	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	user, err := s.iamService.CreateUser(ctx, organizationIamIdentifier, newUser)

	if err != nil {
		return err
	}

	err = s.userRepository.CreateUser(ctx, organizationId, *user)

	if err != nil {
		s.iamService.DeleteUser(ctx, organizationIamIdentifier, user.Id)
		return err
	}

	return nil
}

func (s *UserService) DeleteUser(ctx context.Context, organizationId int, iamUserId string) error {

	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	err = s.iamService.DeleteUser(ctx, organizationIamIdentifier, iamUserId)

	if err != nil {
		return err
	}

	err = s.userRepository.DeleteUser(ctx, organizationId, iamUserId)

	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) UpdateUser(ctx context.Context, organizationId int, iamUserId string, user models.UpdateUser) error {

	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	err = s.iamService.UpdateUser(ctx, organizationIamIdentifier, iamUserId, user)

	if err != nil {
		return err
	}

	err = s.userRepository.UpdateUser(ctx, organizationId, iamUserId, models.User{
		Id:        iamUserId,
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

	roles, err := s.iamService.GetRoles(ctx)

	if err != nil {
		return nil, err
	}

	return roles, nil
}

func (s *UserService) AssignRoles(ctx context.Context, organizationId int, iamUserId string, roles []models.Role) error {
	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	return s.iamService.AssignRolesToUser(ctx, organizationIamIdentifier, iamUserId, roles)
}

func (s *UserService) GetUserById(ctx context.Context, organizationId int, iamUserId string) (*models.User, error) {
	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return nil, err
	}

	return s.iamService.GetUserById(ctx, organizationIamIdentifier, iamUserId)
}

func (s *UserService) GetUsersInformation(ctx context.Context, organizationId int) ([]models.BasicUserInformation, error) {
	user, err := s.userRepository.GetUsersInformation(ctx, organizationId)

	if err != nil {
		return nil, err
	}

	return user, nil
}
