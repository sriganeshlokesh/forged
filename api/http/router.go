package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/sriganeshlokesh/forged/api/http/handle"
	"github.com/sriganeshlokesh/forged/api/http/middleware"
)

// NewRouter constructs a chi router with the standard middleware stack and all routes registered.
// Middleware order: RequestID → RealIP → RequestLogger → Recoverer.
// RequestLogger is placed before Recoverer so that panics are logged as 500s with full duration.
func NewRouter(logger *slog.Logger, health *handle.HealthHandler, eval *handle.EvaluationHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP) //nolint:staticcheck // trusted behind Railway's edge proxy, which always sets X-Forwarded-For
	r.Use(middleware.RequestLogger(logger))
	r.Use(chimw.Recoverer)

	r.Get("/health", health.Health)
	r.Post("/v1/evaluations", eval.Evaluate)

	return r
}
