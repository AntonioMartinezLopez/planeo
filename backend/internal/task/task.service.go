package task

import "fmt"

type TaskService struct{}

func NewTaskService() *TaskService {
	return &TaskService{}
}

func (s *TaskService) GetTask(id string) string {
	return fmt.Sprintf("GetTask endpoint. Id called: %s", id)
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
