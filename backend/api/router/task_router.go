package router

import (
	"planeo/api/api/handler"

	"github.com/go-chi/chi/v5"
)

func TaskRouter(router chi.Router) {

	taskHandler := handler.NewTaskHandler()

	router.Route("/task", func(r chi.Router) {
		r.Get("/{id}", taskHandler.GetTask)
		r.Post("/", taskHandler.CreateTask)
		r.Put("/{id}", taskHandler.UpdateTask)
		r.Delete("/{id}", taskHandler.DeleteTask)
	})
}
