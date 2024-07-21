package group

import (
	"errors"
	"fmt"
)

type GroupService struct{}

func NewGroupService() *GroupService {
	return &GroupService{}
}

func (s *GroupService) GetGroup(id string) (string, error) {
	if id == "1" {
		return fmt.Sprintf("GetGroup endpoint. Id called: %s", id), errors.New(
			"instance not found",
		)
	}
	return fmt.Sprintf("GetGroup endpoint. Id called: %s", id), nil
}

func (s *GroupService) CreateGroup() string {
	return "CreateGroup endpoint"
}

func (s *GroupService) UpdateGroup(id string) string {
	return "UpdateGroup endpoint"
}

func (s *GroupService) DeleteGroup(id string) string {
	return "DeleteGroup endpoint"
}
