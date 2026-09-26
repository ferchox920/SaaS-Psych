package http

import (
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
)

func TestServerDoesNotTrustUnconfiguredForwardedIP(t *testing.T) {
	e := NewServer(ServerDeps{})
	req := httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "198.51.100.17:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.44")
	req.Header.Set("X-Real-IP", "203.0.113.45")
	if got := e.NewContext(req, httptest.NewRecorder()).RealIP(); got != "198.51.100.17" {
		t.Fatalf("untrusted headers spoofed client IP: %q", got)
	}
}

func TestServerTimeoutsAllowClinicalSSE(t *testing.T) {
	e := NewServer(ServerDeps{})
	if e.Server.ReadHeaderTimeout != 10*time.Second || e.Server.ReadTimeout != 60*time.Second || e.Server.IdleTimeout != 120*time.Second {
		t.Fatalf("unexpected HTTP timeouts: header=%v read=%v idle=%v", e.Server.ReadHeaderTimeout, e.Server.ReadTimeout, e.Server.IdleTimeout)
	}
	if e.Server.WriteTimeout != 0 {
		t.Fatalf("write timeout %v would interrupt long-running SSE responses", e.Server.WriteTimeout)
	}
}

func TestSharedModelControlsRequireAdministrativeRole(t *testing.T) {
	auth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(httpmiddleware.WithPrincipal(c.Request().Context(), httpmiddleware.Principal{UserID: uuid.New(), Role: "member"})))
			return next(c)
		}
	}
	identity := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	e := NewServer(ServerDeps{TenantMiddleware: identity, AuthMiddleware: auth, ClinicalAnalysisHandler: handlers.NewClinicalAnalysisHandler(nil)})
	for _, path := range []string{"/api/v1/clinical-ai/warm", "/api/v1/clinical-ai/unload"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("POST", path, nil))
		if rec.Code != 403 {
			t.Fatalf("%s: member got %d, want 403", path, rec.Code)
		}
	}
}

func TestServerTrustsForwardedIPOnlyFromConfiguredProxy(t *testing.T) {
	_, proxyRange, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	e := NewServer(ServerDeps{TrustedProxyRanges: []*net.IPNet{proxyRange}})
	for _, tc := range []struct{ remote, want string }{
		{"192.0.2.10:4321", "203.0.113.44"},
		{"198.51.100.17:4321", "198.51.100.17"},
	} {
		req := httptest.NewRequest("GET", "/health", nil)
		req.RemoteAddr = tc.remote
		req.Header.Set("X-Forwarded-For", "203.0.113.44")
		if got := e.NewContext(req, httptest.NewRecorder()).RealIP(); got != tc.want {
			t.Fatalf("remote %s: got %q, want %q", tc.remote, got, tc.want)
		}
	}
}
