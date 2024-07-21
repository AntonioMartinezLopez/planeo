package task

import (
	"context"
	"net/http"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/setup/operations"

	"github.com/danielgtaylor/huma/v2"
)

type TaskController struct {
	api         *huma.API
	taskService *TaskService
}

func NewTaskController(api *huma.API) *TaskController {
	taskService := NewTaskService()
	return &TaskController{
		api:         api,
		taskService: taskService,
	}
}

func (t *TaskController) InitializeRoutes() {
	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "get-task",
		Method:      http.MethodGet,
		Path:        "/task/{id}",
		Summary:     "Get Task",
		Tags:        []string{"Task"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "read:task")},
	}), func(ctx context.Context, input *GetTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result, err := t.taskService.GetTask(input.Id)

		if err != nil {
			return resp, huma.Error404NotFound(err.Error())
		}
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "create-task",
		Method:      http.MethodPost,
		Path:        "/task",
		Summary:     "Create Task",
		Tags:        []string{"Task"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "create:task")},
	}), func(ctx context.Context, input *CreateTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := t.taskService.CreateTask()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "update-task",
		Method:      http.MethodPut,
		Path:        "/task/{id}",
		Summary:     "Update Task",
		Tags:        []string{"Task"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "update:task")},
	}), func(ctx context.Context, input *UpdateTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := t.taskService.UpdateTask(input.Id)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-task",
		Method:      http.MethodDelete,
		Path:        "/task/{id}",
		Summary:     "Delete Task",
		Tags:        []string{"Task"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "delete:task")},
	}), func(ctx context.Context, input *DeleteTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := t.taskService.DeleteTask(input.Id)
		resp.Body.Message = result
		return resp, nil
	})
}
