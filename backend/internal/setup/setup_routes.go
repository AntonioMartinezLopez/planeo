package setup

import (
	"net/http"

	"planeo/api/internal/middlewares"
	"planeo/api/internal/task"
	jsonHelper "planeo/api/pkg/json"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func registerRoutes(rootRouter *chi.Mux) {
	rootRouter.Get("/", func(w http.ResponseWriter, r *http.Request) {
		jsonHelper.HttpResponse(struct{ Live bool }{Live: true}, w)
	})

	rootRouter.Route("/api", func(r chi.Router) {
		r.Use(middlewares.JwtValidator)
		// Add new sub routers
		task.TaskRouter(r)
	})
}

func SetupRouter() *chi.Mux {

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middlewares.Cors())

	registerRoutes(router)
	return router
}
