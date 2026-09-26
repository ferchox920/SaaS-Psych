package http

import (
	"sessionflow/apps/api/internal/http/handlers"
	"testing"
)

func TestClinicalProjectRoutesRegistered(t *testing.T) {
	server := NewServer(ServerDeps{ClinicalLongitudinalHandler: handlers.NewClinicalLongitudinalHandler(nil), TenantMiddleware: passthroughLongitudinalMiddleware, AuthMiddleware: passthroughLongitudinalMiddleware})
	want := map[string]bool{"POST /api/v1/clients/:id/project-exports": false, "GET /api/v1/clients/:id/project-exports/:export_id": false, "POST /api/v1/clients/:id/project-imports": false, "GET /api/v1/clients/:id/project-proposals/:proposal_id": false}
	for _, route := range server.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	foundHistory := false
	for _, route := range server.Routes() {
		if route.Path == "/api/v1/clients/:id/project-exports" {
			if route.Method == "GET" {
				foundHistory = true
			}
			if route.Method != "GET" && route.Method != "POST" {
				t.Fatal("mutable export route")
			}
		}
	}
	if !foundHistory {
		t.Fatal("missing export history")
	}
	for route, found := range want {
		if !found {
			t.Errorf("missing route: %s", route)
		}
	}
}
