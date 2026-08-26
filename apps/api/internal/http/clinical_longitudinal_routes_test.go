package http

import (
	"testing"

	"github.com/labstack/echo/v4"
	"sessionflow/apps/api/internal/http/handlers"
)

func passthroughLongitudinalMiddleware(next echo.HandlerFunc) echo.HandlerFunc { return next }

func TestClinicalLongitudinalRoutesAreRegistered(t *testing.T) {
	server := NewServer(ServerDeps{ClinicalLongitudinalHandler: handlers.NewClinicalLongitudinalHandler(nil), TenantMiddleware: passthroughLongitudinalMiddleware, AuthMiddleware: passthroughLongitudinalMiddleware})
	want := map[string]bool{"POST /api/v1/clinical-sessions/:id/longitudinal-analysis": false, "GET /api/v1/clients/:id/longitudinal-state": false, "GET /api/v1/clients/:id/evidence": false, "GET /api/v1/clients/:id/events": false, "GET /api/v1/clients/:id/processes": false, "GET /api/v1/clients/:id/hypotheses": false, "GET /api/v1/clients/:id/clinical-diffs": false, "GET /api/v1/clinical-diffs/:id": false, "PUT /api/v1/clinical-diffs/:id/operations/:operation_id/decision": false, "POST /api/v1/clinical-diffs/:id/merge": false}
	for _, route := range server.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("missing route %s", route)
		}
	}
}
