package httpapi

import (
	"net/http"

	"count-hours/backend/internal/db/sqlc"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	pool      *pgxpool.Pool
	queries   *sqlc.Queries
	jwtSecret string
	origins   []string
}

func New(pool *pgxpool.Pool, jwtSecret string, allowedOrigins []string) *API {
	return &API{
		pool:      pool,
		queries:   sqlc.New(pool),
		jwtSecret: jwtSecret,
		origins:   allowedOrigins,
	}
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   a.origins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", a.handleHealth)

		r.Post("/auth/register", a.handleRegister)
		r.Post("/auth/login", a.handleLogin)

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)

			r.Get("/active", a.handleGetActiveSession)

			r.Get("/projects", a.handleListProjects)
			r.Post("/projects", a.handleCreateProject)
			r.Patch("/projects/{id}", a.handleUpdateProject)
			r.Delete("/projects/{id}", a.handleDeleteProject)

			r.Get("/sessions", a.handleListSessions)
			r.Post("/sessions", a.handleStartSession)
			r.Post("/sessions/{id}/pause", a.handlePauseSession)
			r.Post("/sessions/{id}/resume", a.handleResumeSession)
			r.Post("/sessions/{id}/stop", a.handleStopSession)

			r.Get("/dashboard", a.handleDashboard)
		})
	})

	return r
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}