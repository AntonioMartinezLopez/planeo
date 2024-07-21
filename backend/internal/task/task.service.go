package task

import (
	"errors"
	"fmt"
)

type TaskService struct{}

func NewTaskService() *TaskService {
	return &TaskService{}
}

func (s *TaskService) GetTask(id string) (string, error) {
	if id == "1" {
		return fmt.Sprintf("GetTask endpoint. Id called: %s", id), errors.New(
			"instance not found",
		)
	}
	return fmt.Sprintf("GetTask endpoint. Id called: %s", id), nil
}

func (s *TaskService) CreateTask() string {
	return "CreateTask endpoint"
}

func (s *TaskService) UpdateTask(id string) string {
	return "UpdateTask endpoint"
}

func (s *TaskService) DeleteTask(id string) string {
	return "DeleteTask endpoint"
}
