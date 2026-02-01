package user

import (
	"context"
)

type UserService struct {
	iamService     IAM
	userRepository UserRepository
}

func NewService(userRepository UserRepository, iamService IAM) *UserService {
	return &UserService{
		iamService:     iamService,
		userRepository: userRepository,
	}
}

func (s *UserService) GetIAMUsers(ctx context.Context, organizationId int, sync bool) ([]IAMUser, error) {
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
			return nil, err
		}
	}

	return users, nil
}

func (s *UserService) CreateUser(ctx context.Context, organizationId int, newUser NewUser) error {
	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	user, err := s.iamService.CreateUser(ctx, organizationIamIdentifier, newUser)

	if err != nil {
		return err
	}

	err = s.userRepository.CreateUser(ctx, organizationId, user.Id, newUser)

	if err != nil {
		_ = s.iamService.DeleteUser(ctx, organizationIamIdentifier, user.Id)
		return err
	}

	return nil
}

func (s *UserService) DeleteUser(ctx context.Context, organizationId int, uuid string) error {
	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	err = s.iamService.DeleteUser(ctx, organizationIamIdentifier, uuid)

	if err != nil {
		return err
	}

	err = s.userRepository.DeleteUser(ctx, organizationId, uuid)

	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) UpdateUser(ctx context.Context, organizationId int, uuid string, user UpdateUser) error {
	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	err = s.iamService.UpdateUser(ctx, organizationIamIdentifier, uuid, user)

	if err != nil {
		return err
	}

	err = s.userRepository.UpdateUser(ctx, organizationId, uuid, user)

	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) GetAvailableRoles(ctx context.Context) ([]Role, error) {
	roles, err := s.iamService.GetRoles(ctx)

	if err != nil {
		return nil, err
	}

	return roles, nil
}

func (s *UserService) AssignRoles(ctx context.Context, organizationId int, uuid string, roles []Role) error {
	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return err
	}

	return s.iamService.AssignRolesToUser(ctx, organizationIamIdentifier, uuid, roles)
}

func (s *UserService) GetUserByUuid(ctx context.Context, organizationId int, uuid string) (*IAMUser, error) {
	organizationIamIdentifier, err := s.userRepository.GetIamOrganizationIdentifier(ctx, organizationId)

	if err != nil {
		return nil, err
	}

	return s.iamService.GetUserById(ctx, organizationIamIdentifier, uuid)
}

func (s *UserService) GetUsers(ctx context.Context, organizationId int) ([]User, error) {
	user, err := s.userRepository.GetUsers(ctx, organizationId)

	if err != nil {
		return nil, err
	}

	return user, nil
}
