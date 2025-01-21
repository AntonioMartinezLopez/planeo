package task

import (
	"context"
	"net/http"
	"planeo/api/config"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/setup/operations"

	"github.com/danielgtaylor/huma/v2"
)

type TaskController struct {
	api         *huma.API
	taskService *TaskService
	config      *config.ApplicationConfiguration
}

func NewTaskController(api *huma.API, config *config.ApplicationConfiguration) *TaskController {
	taskService := NewTaskService()
	return &TaskController{
		api:         api,
		taskService: taskService,
		config:      config,
	}
}

func (t *TaskController) InitializeRoutes() {
	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "get-task",
		Method:      http.MethodGet,
		Path:        "/{organization}/groups/{groupId}/tasks/{taskId}",
		Summary:     "Get Task",
		Tags:        []string{"Tasks"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, t.config, "task", "read")},
	}), func(ctx context.Context, input *GetTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result, err := t.taskService.GetTask(input.TaskId)

		if err != nil {
			return resp, huma.Error404NotFound(err.Error())
		}
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "create-task",
		Method:      http.MethodPost,
		Path:        "/{organization}/groups/{groupId}/tasks",
		Summary:     "Create Task",
		Tags:        []string{"Tasks"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, t.config, "task", "create")},
	}), func(ctx context.Context, input *CreateTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := t.taskService.CreateTask()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "update-task",
		Method:      http.MethodPut,
		Path:        "/{organization}/groups/{groupId}/tasks/{taskId}",
		Summary:     "Update Task",
		Tags:        []string{"Tasks"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, t.config, "task", "update")},
	}), func(ctx context.Context, input *UpdateTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := t.taskService.UpdateTask(input.TaskId)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-task",
		Method:      http.MethodDelete,
		Path:        "/{organization}/groups/{groupId}/tasks/{taskId}",
		Summary:     "Delete Task",
		Tags:        []string{"Tasks"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, t.config, "task", "delete")},
	}), func(ctx context.Context, input *DeleteTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := t.taskService.DeleteTask(input.TaskId)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "get-tasks",
		Method:      http.MethodGet,
		Path:        "/{organization}/groups/{groupId}/tasks",
		Summary:     "Get Tasks",
		Tags:        []string{"Tasks"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, t.config, "task", "read")},
	}), func(ctx context.Context, input *GetTasksInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := t.taskService.GetTasks()
		resp.Body.Message = result
		return resp, nil
	})
}
