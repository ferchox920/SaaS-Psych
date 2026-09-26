package handlers

import (
	"go.yaml.in/yaml/v2"
	"testing"
)

func TestStage3BOpenAPIReadContracts(t *testing.T) {
	var root map[any]any
	if err := yaml.UnmarshalStrict(embeddedOpenAPISpec, &root); err != nil {
		t.Fatal(err)
	}
	paths := root["paths"].(map[any]any)
	for _, path := range []string{"/api/v1/clients/{id}/approved-clinical-context", "/api/v1/clients/{id}/clinical-runs/{run_id}/provenance", "/api/v1/clients/{id}/clinical-history/{kind}/{entity_id}", "/api/v1/clients/{id}/evidence-page"} {
		p, ok := paths[path].(map[any]any)
		if !ok {
			t.Fatal("missing path", path)
		}
		if len(p) != 1 {
			t.Fatal("projection must be GET only")
		}
		op := p["get"].(map[any]any)
		auth := op["security"].([]any)[0].(map[any]any)
		if _, ok := auth["tenantHeader"]; !ok {
			t.Fatal("tenant missing")
		}
		if _, ok := auth["bearerAuth"]; !ok {
			t.Fatal("auth missing")
		}
		responses := op["responses"].(map[any]any)
		if _, ok := responses["200"].(map[any]any)["content"]; !ok {
			t.Fatal("typed response missing")
		}
	}
	schemas := root["components"].(map[any]any)["schemas"].(map[any]any)
	props := schemas["ClinicalRunProvenanceRead"].(map[any]any)["properties"].(map[any]any)
	for _, name := range []string{"parameters_json", "prompt_text", "storage_key", "error_message"} {
		if _, ok := props[name]; ok {
			t.Fatal("unsafe metadata", name)
		}
	}
	for _, name := range []string{"ClinicalEvidencePageRead", "ClinicalHistoryPageRead"} {
		items := schemas[name].(map[any]any)["properties"].(map[any]any)["items"].(map[any]any)
		if items["maxItems"] != 25 {
			t.Fatal("unbounded page")
		}
	}
}
