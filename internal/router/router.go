package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"organizing-app-backend/internal/controller"
	"organizing-app-backend/internal/service"
)

// New builds the application's HTTP router, wiring each route to its
// controller. Controllers own the request/response handling; services own
// the business logic.
func New() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	inventoryController := controller.NewInventoryController(service.NewInventoryService())

	r.Route("/api/inventory", func(r chi.Router) {
		r.Get("/", inventoryController.List)
		r.Post("/", inventoryController.Create)
		r.Get("/{id}", inventoryController.Get)
		r.Put("/{id}", inventoryController.Update)
		r.Delete("/{id}", inventoryController.Delete)
	})

	return r
}
