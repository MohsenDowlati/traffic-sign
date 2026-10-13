package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	limiterHttp "github.com/MohsenDowlati/traffic-sign/internal/transport/http"
)

func TestKeyFuncStrategies(t *testing.T) {
	t.Run("KeyByIP extracts correct IP from headers and remote addr", func(t *testing.T) {
		fn := limiterHttp.KeyByIP()

		req1 := httptest.NewRequest(http.MethodGet, "/", nil)
		req1.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
		if key := fn(req1); key != "ip:203.0.113.195" {
			t.Errorf("expected ip:203.0.113.195, got %s", key)
		}

		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.RemoteAddr = "192.168.1.50:54321"
		if key := fn(req2); key != "ip:192.168.1.50" {
			t.Errorf("expected ip:192.168.1.50, got %s", key)
		}
	})

	t.Run("KeyByHeaderAPIKey extracts token or falls back to IP", func(t *testing.T) {
		fn := limiterHttp.KeyByHeaderAPIKey("X-API-Key")

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "secret-token-abc")
		if key := fn(req); key != "apikey:secret-token-abc" {
			t.Errorf("expected apikey:secret-token-abc, got %s", key)
		}

		reqNoHeader := httptest.NewRequest(http.MethodGet, "/", nil)
		reqNoHeader.RemoteAddr = "10.0.0.1:8080"
		if key := fn(reqNoHeader); key != "ip:10.0.0.1" {
			t.Errorf("expected fallback to IP, got %s", key)
		}
	})

	t.Run("KeyByUserIDFromContext extracts user or falls back to IP", func(t *testing.T) {
		fn := limiterHttp.KeyByUserIDFromContext(limiterHttp.UserIDContextKey)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := limiterHttp.WithUserID(req.Context(), "user-uuid-99")
		req = req.WithContext(ctx)

		if key := fn(req); key != "user:user-uuid-99" {
			t.Errorf("expected user:user-uuid-99, got %s", key)
		}

		reqGuest := httptest.NewRequest(http.MethodGet, "/", nil)
		reqGuest.RemoteAddr = "172.16.0.5:1234"
		if key := fn(reqGuest); key != "ip:172.16.0.5" {
			t.Errorf("expected guest to fallback to ip, got %s", key)
		}
	})

	t.Run("KeyByRouteAndMethod extracts method and path", func(t *testing.T) {
		fn := limiterHttp.KeyByRouteAndMethod()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		if key := fn(req); key != "route:POST:/api/v1/auth/login" {
			t.Errorf("expected route:POST:/api/v1/auth/login, got %s", key)
		}
	})

	t.Run("KeyFuncFromConfig factory selects correct strategy", func(t *testing.T) {
		fnAPI := limiterHttp.KeyFuncFromConfig("api_key", "X-Custom-Key")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Custom-Key", "my-key")
		if key := fnAPI(req); key != "apikey:my-key" {
			t.Errorf("factory failed for api_key, got %s", key)
		}

		fnDefault := limiterHttp.KeyFuncFromConfig("unknown_value", "")
		reqDefault := httptest.NewRequest(http.MethodGet, "/", nil)
		reqDefault.RemoteAddr = "1.2.3.4:80"
		if key := fnDefault(reqDefault); key != "ip:1.2.3.4" {
			t.Errorf("factory fallback to IP failed, got %s", key)
		}
	})
}
