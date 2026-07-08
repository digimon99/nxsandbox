package api

import (
	"net/http"

	"github.com/digimon99/nxsandbox/internal/auth"
	"github.com/digimon99/nxsandbox/internal/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type RouterConfig struct {
	AuthHandlers *AuthHandlers
	AuthService  *auth.Service
	AppsHandlers *AppsHandlers
	WebHandler   *web.Handler
}

func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.StripSlashes)
	r.Use(RecoveryMiddleware)
	r.Use(LoggingMiddleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		Success(w, map[string]interface{}{"status": "ok"})
	})

	r.Post("/api/auth/signin", cfg.AuthHandlers.SignIn)
	r.Post("/api/auth/verify", cfg.AuthHandlers.Verify)
	r.Post("/api/auth/refresh", cfg.AuthHandlers.Refresh)
	r.Post("/api/auth/signoff", cfg.AuthHandlers.SignOff)

	r.Group(func(r chi.Router) {
		r.Use(auth.SessionMiddleware(cfg.AuthService))
		r.Use(auth.RequireAuth)
		r.Get("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetClaims(r)
			Success(w, map[string]interface{}{
				"id":    claims.Subject,
				"email": claims.Email,
				"role":  claims.Role,
			})
		})

		r.Get("/api/apps", cfg.AppsHandlers.ListApps)
		r.Post("/api/apps", cfg.AppsHandlers.CreateApp)
		r.Get("/api/apps/{id}/deployments", cfg.AppsHandlers.ListDeployments)
	})

	if cfg.WebHandler != nil {
		cfg.WebHandler.Mount(r)
	}

	return r
}
