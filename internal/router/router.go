package router

import (
	"context"
	"database/sql"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"organizing-app-backend/internal/ai"
	"organizing-app-backend/internal/controller"
	"organizing-app-backend/internal/service"
	"organizing-app-backend/internal/storage"
)

// http.FileServer resolves content types via the OS mime database, which
// doesn't reliably know these extensions (e.g. .webmanifest reads back as
// text/plain on some hosts) — register them explicitly so behavior doesn't
// depend on what's installed on the machine the binary happens to run on.
// Chrome's PWA installability check in particular wants the manifest served
// as application/manifest+json.
func init() {
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
	mime.AddExtensionType(".js", "text/javascript")
}

// New builds the application's HTTP router, wiring each route to its
// controller. Controllers own the request/response handling; services own
// the business logic.
func New(database *sql.DB, photos storage.Store, aiClient *ai.Client) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	categoryService := service.NewCategoryService(database, photos)

	inventoryController := controller.NewInventoryController(service.NewInventoryService(database, photos))
	categoryController := controller.NewCategoryController(categoryService)
	authController := controller.NewAuthController(service.NewAuthService(database))
	recognitionController := controller.NewRecognitionController(aiClient, categoryService)

	uploadController := controller.NewUploadController(photos)

	// Unauthenticated: k8s liveness/readiness probes hit this. It checks the
	// DB connection (not just "the process is running") so a pod with a
	// severed DB link gets pulled out of rotation instead of serving 500s.
	r.Get("/healthz", healthzHandler(database))

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

		// Item photos: uploaded via the API into object storage, served back
		// same-origin so the session cookie accompanies <img> requests and
		// the bucket itself stays private.
		r.Post("/api/uploads", uploadController.Upload)
		r.Get("/uploads/{name}", uploadController.Serve)

		// AI Recognition add-item flow: classifies a photo into one of the
		// user's existing categories plus a suggested item name. Never
		// touches storage — it's a separate call from /api/uploads so a
		// photo used only to try recognition (never saved) is never
		// persisted.
		r.Post("/api/recognize", recognitionController.Recognize)

		r.Route("/api/tags", func(r chi.Router) {
			r.Get("/", categoryController.ListTags)
			r.Post("/", categoryController.CreateTag)
			r.Delete("/{id}", categoryController.DeleteTag)
		})
	})

	// Everything else: serve the built frontend (client/dist) with an SPA
	// fallback, so client-side routes like /category/1 work on refresh/deep
	// link. Registered last so /api/* and /uploads/* above always win; the
	// handler itself also refuses those prefixes as a second line of defense
	// (see staticFileHandler).
	r.Get("/*", staticFileHandler(staticDir()))

	return r
}

func healthzHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := database.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("db unreachable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

// staticDir is where the built client lives, relative to the backend
// process's working directory. The server is normally started with `backend`
// as the cwd (e.g. `cd backend && go run ./cmd/server`, matching
// DATABASE_URL/UPLOAD_DIR conventions), so client/dist is one level up by
// default. Override with STATIC_DIR for deployments that run the compiled
// binary from elsewhere.
func staticDir() string {
	if dir := os.Getenv("STATIC_DIR"); dir != "" {
		return dir
	}
	return "../client/dist"
}

// staticFileHandler serves the built frontend out of dir, falling back to
// index.html (the SPA shell) for any GET/HEAD request that doesn't map to a
// real file on disk — e.g. client-side routes like /category/1 — so deep
// links and page refreshes work. /api/* and /uploads/* are routed above and
// always take precedence, but the prefix check here is a deliberate second
// guard: it ensures an unmatched path under those prefixes (e.g. an unknown
// /api/ endpoint) 404s plainly instead of ever getting the SPA shell.
func staticFileHandler(dir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(dir))
	indexPath := filepath.Join(dir, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/uploads/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		requested := filepath.Join(dir, filepath.Clean(r.URL.Path))
		info, err := os.Stat(requested)
		if err != nil || info.IsDir() {
			// Not a real file (or a directory listing) — hand it to the SPA
			// shell rather than 404ing, so client-side routing owns it.
			http.ServeFile(w, r, indexPath)
			return
		}

		fileServer.ServeHTTP(w, r)
	}
}
