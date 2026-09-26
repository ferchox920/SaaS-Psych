package http

import (
	"sessionflow/apps/api/internal/http/handlers"
	"strings"
	"testing"
)

func TestStage3BReadRoutesAndNoDirectCRUD(t *testing.T) {
	s := NewServer(ServerDeps{ClinicalLongitudinalHandler: handlers.NewClinicalLongitudinalHandler(nil), TenantMiddleware: passthroughLongitudinalMiddleware, AuthMiddleware: passthroughLongitudinalMiddleware})
	want := map[string]bool{"/api/v1/clients/:id/approved-clinical-context": false, "/api/v1/clients/:id/clinical-runs/:run_id/provenance": false, "/api/v1/clients/:id/clinical-history/:kind/:entity_id": false, "/api/v1/clients/:id/evidence-page": false}
	for _, r := range s.Routes() {
		if _, ok := want[r.Path]; ok {
			if r.Method != "GET" {
				t.Fatal("read projection allows write")
			}
			want[r.Path] = true
		}
		if r.Method == "PUT" || r.Method == "PATCH" || r.Method == "DELETE" {
			for _, prefix := range []string{"/api/v1/processes/", "/api/v1/hypotheses/", "/api/v1/goals/", "/api/v1/giras/"} {
				if strings.HasPrefix(r.Path, prefix) {
					t.Fatal("direct longitudinal mutation route")
				}
			}
		}
		if r.Method == "POST" && strings.Contains(r.Path, "/achieve") {
			t.Fatal("direct goal achieve")
		}
	}
	for path, found := range want {
		if !found {
			t.Fatal("missing read route", path)
		}
	}
}
