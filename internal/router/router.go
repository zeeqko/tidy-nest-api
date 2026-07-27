package router

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"organizing-app-backend/internal/controller"
	"organizing-app-backend/internal/service"
)

// New builds the application's HTTP router, wiring each route to its
// controller. Controllers own the request/response handling; services own
// the business logic.
func New(database *sql.DB) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	inventoryController := controller.NewInventoryController(service.NewInventoryService(database))
	categoryController := controller.NewCategoryController(service.NewCategoryService(database))
	authController := controller.NewAuthController(service.NewAuthService(database))

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "uploads"
	}
	uploadController := controller.NewUploadController(uploadDir)

	// Public: signup and login only. Everything else requires a session.
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/signup", authController.Signup)
		r.Post("/login", authController.Login)
		r.Post("/logout", authController.Logout)
		r.With(authController.RequireAuth).Get("/me", authController.Me)
	})

	r.Group(func(r chi.Router) {
		r.Use(authController.RequireAuth)

		r.Route("/api/inventory", func(r chi.Router) {
			r.Get("/", inventoryController.List)
			r.Post("/", inventoryController.Create)
			r.Get("/{id}", inventoryController.Get)
			r.Put("/{id}", inventoryController.Update)
			r.Delete("/{id}", inventoryController.Delete)
		})

		r.Route("/api/categories", func(r chi.Router) {
			r.Get("/", categoryController.ListCategories)
			r.Post("/", categoryController.CreateCategory)
			r.Put("/{id}", categoryController.UpdateCategory)
			r.Delete("/{id}", categoryController.DeleteCategory)
			r.Post("/{id}/subcategories", categoryController.CreateSubCategory)
			r.Post("/{id}/tags", categoryController.AttachTag)
			r.Delete("/{id}/tags/{tagId}", categoryController.DetachTag)
		})
		r.Delete("/api/subcategories/{id}", categoryController.DeleteSubCategory)

		// Item photos: uploaded via the API, served same-origin so the
		// session cookie accompanies <img> requests.
		r.Post("/api/uploads", uploadController.Upload)
		r.Get("/uploads/{name}", uploadController.Serve)

		r.Route("/api/tags", func(r chi.Router) {
			r.Get("/", categoryController.ListTags)
			r.Post("/", categoryController.CreateTag)
			r.Delete("/{id}", categoryController.DeleteTag)
		})
	})

	return r
}
