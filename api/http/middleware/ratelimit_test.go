package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sriganeshlokesh/forged/api/dto"
	"github.com/sriganeshlokesh/forged/api/http/middleware"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func doRequest(t *testing.T, h http.Handler, remoteAddr string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/evaluations", nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestRateLimitPerIP(t *testing.T) {
	tests := []struct {
		name string
		rpm  int
		run  func(t *testing.T, h http.Handler)
	}{
		{
			name: "requests within limit pass",
			rpm:  3,
			run: func(t *testing.T, h http.Handler) {
				for i := 0; i < 3; i++ {
					res := doRequest(t, h, "10.0.0.1:1234")
					if res.StatusCode != http.StatusOK {
						t.Fatalf("request %d: expected 200, got %d", i+1, res.StatusCode)
					}
				}
			},
		},
		{
			name: "request over limit returns 429 envelope",
			rpm:  2,
			run: func(t *testing.T, h http.Handler) {
				for i := 0; i < 2; i++ {
					_ = doRequest(t, h, "10.0.0.2:1234")
				}
				res := doRequest(t, h, "10.0.0.2:1234")
				if res.StatusCode != http.StatusTooManyRequests {
					t.Fatalf("expected 429, got %d", res.StatusCode)
				}
				var body dto.ErrorResponse
				if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode 429 body: %v", err)
				}
				if body.Code != 10003 {
					t.Errorf("expected error code 10003, got %d", body.Code)
				}
			},
		},
		{
			name: "different IPs have independent buckets",
			rpm:  1,
			run: func(t *testing.T, h http.Handler) {
				if res := doRequest(t, h, "10.0.0.3:1234"); res.StatusCode != http.StatusOK {
					t.Fatalf("first IP: expected 200, got %d", res.StatusCode)
				}
				if res := doRequest(t, h, "10.0.0.4:1234"); res.StatusCode != http.StatusOK {
					t.Fatalf("second IP: expected 200, got %d", res.StatusCode)
				}
				if res := doRequest(t, h, "10.0.0.3:1234"); res.StatusCode != http.StatusTooManyRequests {
					t.Fatalf("first IP again: expected 429, got %d", res.StatusCode)
				}
			},
		},
		{
			name: "rpm 0 disables limiting",
			rpm:  0,
			run: func(t *testing.T, h http.Handler) {
				for i := 0; i < 50; i++ {
					res := doRequest(t, h, "10.0.0.5:1234")
					if res.StatusCode != http.StatusOK {
						t.Fatalf("request %d: expected 200, got %d", i+1, res.StatusCode)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := middleware.RateLimitPerIP(tt.rpm)(okHandler())
			tt.run(t, h)
		})
	}
}
