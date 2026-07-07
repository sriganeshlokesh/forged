package handle

import (
	"encoding/json"
	"net/http"

	"github.com/sriganeshlokesh/forged/api/dto"
	"github.com/sriganeshlokesh/forged/config"
)

// HealthHandler handles the GET /health endpoint.
// This handler must never grow dependencies — it gates Railway deploy health checks.
// A dependency failure here would block all future deploys.
type HealthHandler struct {
	service string
	version string
}

// NewHealthHandler constructs a HealthHandler from the application config.
func NewHealthHandler(cfg *config.Config) *HealthHandler {
	return &HealthHandler{
		service: cfg.ServiceName,
		version: cfg.Version,
	}
}

// Health writes a 200 OK response with a JSON HealthResponse body.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	resp := dto.HealthResponse{
		Status:  "ok",
		Service: h.service,
		Version: h.version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
