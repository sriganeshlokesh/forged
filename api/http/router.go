package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/sriganeshlokesh/forged/api/http/middleware"
	"github.com/sriganeshlokesh/forged/config"
)

// HealthRoutes is what the router needs from the health handler.
// Declared here, at the consumer; satisfied implicitly by *handle.HealthHandler.
type HealthRoutes interface {
	Health(w http.ResponseWriter, r *http.Request)
}

// EvaluationRoutes is what the router needs from the evaluation handler.
// Satisfied implicitly by *handle.EvaluationHandler.
type EvaluationRoutes interface {
	Evaluate(w http.ResponseWriter, r *http.Request)
}

// NewRouter constructs a chi router with the standard middleware stack and all routes registered.
// Middleware order: RequestID → RealIP → RequestLogger → Recoverer → CORS.
// CORS is placed after Recoverer so that preflight OPTIONS requests receive
// the correct headers even when other middleware panics or rejects the request.
// RequestLogger is placed before Recoverer so that panics are logged as 500s with full duration.
func NewRouter(cfg *config.Config, logger *slog.Logger, health HealthRoutes, eval EvaluationRoutes) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP) //nolint:staticcheck // trusted behind Railway's edge proxy, which always sets X-Forwarded-For
	r.Use(middleware.RequestLogger(logger))
	r.Use(chimw.Recoverer)
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	// /health stays outside the rate limiter — Railway healthchecks must never 429.
	r.Get("/health", health.Health)
	r.With(middleware.RateLimitPerIP(cfg.RateLimitPerIPRPM)).
		Post("/v1/evaluations", eval.Evaluate)

	return r
}
