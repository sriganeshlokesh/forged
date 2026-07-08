package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sriganeshlokesh/forged/api/http/middleware"
)

func TestCORS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name               string
		allowedOrigins     []string
		method             string
		origin             string
		extraHeaders       map[string]string
		wantStatus         int
		wantACAllowOrigin  bool // Access-Control-Allow-Origin present and matching origin
		wantACAllowMethods bool // Access-Control-Allow-Methods present (preflight only)
		wantNoCORSHeaders  bool // no CORS headers added at all (disabled case)
	}{
		{
			name:              "allowed origin on GET passes through with ACAO header",
			allowedOrigins:    []string{"https://drafted.up.railway.app"},
			method:            http.MethodGet,
			origin:            "https://drafted.up.railway.app",
			wantStatus:        http.StatusOK,
			wantACAllowOrigin: true,
		},
		{
			name:              "allowed origin on POST passes through with ACAO header",
			allowedOrigins:    []string{"https://drafted.up.railway.app"},
			method:            http.MethodPost,
			origin:            "https://drafted.up.railway.app",
			wantStatus:        http.StatusOK,
			wantACAllowOrigin: true,
		},
		{
			name:              "disallowed origin gets no ACAO header",
			allowedOrigins:    []string{"https://drafted.up.railway.app"},
			method:            http.MethodGet,
			origin:            "https://evil.example.com",
			wantStatus:        http.StatusOK,
			wantACAllowOrigin: false,
		},
		{
			name:           "preflight OPTIONS with allowed origin returns 200 and CORS headers",
			allowedOrigins: []string{"https://drafted.up.railway.app"},
			method:         http.MethodOptions,
			origin:         "https://drafted.up.railway.app",
			extraHeaders: map[string]string{
				"Access-Control-Request-Method":  "POST",
				"Access-Control-Request-Headers": "Content-Type",
			},
			wantStatus:         http.StatusOK,
			wantACAllowOrigin:  true,
			wantACAllowMethods: true,
		},
		{
			name:              "empty allowedOrigins disables CORS — request passes through without CORS headers",
			allowedOrigins:    []string{},
			method:            http.MethodGet,
			origin:            "https://drafted.up.railway.app",
			wantStatus:        http.StatusOK,
			wantNoCORSHeaders: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := middleware.CORS(tt.allowedOrigins)(next)

			req := httptest.NewRequest(tt.method, "/v1/evaluations", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			for k, v := range tt.extraHeaders {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			res := rec.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("status: got %d, want %d", res.StatusCode, tt.wantStatus)
			}

			acao := res.Header.Get("Access-Control-Allow-Origin")
			if tt.wantACAllowOrigin {
				if acao != tt.origin {
					t.Errorf("Access-Control-Allow-Origin: got %q, want %q", acao, tt.origin)
				}
			} else if !tt.wantNoCORSHeaders {
				// disallowed origin: header must be absent or not match
				if acao == tt.origin {
					t.Errorf("Access-Control-Allow-Origin: expected mismatch but got %q", acao)
				}
			}

			if tt.wantACAllowMethods {
				if acam := res.Header.Get("Access-Control-Allow-Methods"); acam == "" {
					t.Error("Access-Control-Allow-Methods: expected a value, got empty string")
				}
			}

			if tt.wantNoCORSHeaders {
				if acao != "" {
					t.Errorf("expected no Access-Control-Allow-Origin header (CORS disabled), got %q", acao)
				}
				if acam := res.Header.Get("Access-Control-Allow-Methods"); acam != "" {
					t.Errorf("expected no Access-Control-Allow-Methods header (CORS disabled), got %q", acam)
				}
			}
		})
	}
}
