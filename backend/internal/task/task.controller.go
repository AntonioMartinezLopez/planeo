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
		Security: []map[string][]string{
			{"myAuth": {"openid", "email", "profile"}},
		},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(api, "read:task")},
	}, func(ctx context.Context, input *GetTaskInput) (*TaskOutput, error) {
		resp := &TaskOutput{}
		result := taskService.GetTask(input.Id)
		resp.Body.Message = result
		return resp, nil
	})

	// router.Route("/task", func(r chi.Router) {
	// 	r.Get("/{id}", taskHandler.GetTask)
	// 	r.Post("/", taskHandler.CreateTask)
	// 	r.Put("/{id}", taskHandler.UpdateTask)
	// 	r.Delete("/{id}", taskHandler.DeleteTask)
	// })
}
