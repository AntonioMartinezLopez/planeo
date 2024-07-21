package task

import (
	"context"
	"net/http"
	"planeo/api/internal/middlewares"

	"github.com/danielgtaylor/huma/v2"
)

func TaskRouter(api huma.API) {

	taskService := NewTaskService()

	huma.Register(api, huma.Operation{
		OperationID: "get-task",
		Method:      http.MethodGet,
		Path:        "/task/{id}",
		Summary:     "Get Task",
		Tags:        []string{"Task"},
		Security: []map[string][]string{
			{"myAuth": {}},
		},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(api, "read:task")},
	}, func(ctx context.Context, input *GetTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result, err := taskService.GetTask(input.Id)

		if err != nil {
			return resp, huma.Error404NotFound(err.Error())
		}
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-task",
		Method:      http.MethodPost,
		Path:        "/task",
		Summary:     "Create Task",
		Tags:        []string{"Task"},
		Security: []map[string][]string{
			{"myAuth": {}},
		},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(api, "create:task")},
	}, func(ctx context.Context, input *CreateTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := taskService.CreateTask()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-task",
		Method:      http.MethodPut,
		Path:        "/task/{id}",
		Summary:     "Update Task",
		Tags:        []string{"Task"},
		Security: []map[string][]string{
			{"myAuth": {}},
		},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(api, "update:task")},
	}, func(ctx context.Context, input *UpdateTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := taskService.UpdateTask(input.Id)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-task",
		Method:      http.MethodDelete,
		Path:        "/task/{id}",
		Summary:     "Delete Task",
		Tags:        []string{"Task"},
		Security: []map[string][]string{
			{"myAuth": {}},
		},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(api, "delete:task")},
	}, func(ctx context.Context, input *DeleteTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := taskService.DeleteTask(input.Id)
		resp.Body.Message = result
		return resp, nil
	})
}
