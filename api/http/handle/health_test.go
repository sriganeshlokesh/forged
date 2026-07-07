package handle_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sriganeshlokesh/forged/api/dto"
	"github.com/sriganeshlokesh/forged/api/http/handle"
	"github.com/sriganeshlokesh/forged/config"
)

func TestHealthHandler(t *testing.T) {
	cfg := &config.Config{
		ServiceName: "forged",
		Version:     "test",
	}
	h := handle.NewHealthHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	ct := res.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	var body dto.HealthResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", body.Status)
	}
	if body.Service != "forged" {
		t.Errorf("expected service 'forged', got %q", body.Service)
	}
}
