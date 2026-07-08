package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS returns a middleware that adds Access-Control-Allow-* headers and
// handles preflight OPTIONS requests for the given allowed origins.
// It must run early in the stack so that preflight OPTIONS is handled before
// any route-level middleware (e.g. rate limiting).
// An empty allowedOrigins slice disables CORS entirely — no headers are added
// and OPTIONS requests are passed through to the router unchanged.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	})
}
