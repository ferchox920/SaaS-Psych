package handlers

import (
	"go.yaml.in/yaml/v2"
	"testing"
)

func TestIngestionOpenAPICompleteContracts(t *testing.T) {
	var root map[any]any
	if err := yaml.UnmarshalStrict(embeddedOpenAPISpec, &root); err != nil {
		t.Fatal(err)
	}
	paths := root["paths"].(map[any]any)
	for _, route := range []struct{ path, method string }{
		{"/api/v1/clients/{id}/consents", "get"},
		{"/api/v1/clients/{id}/consents", "post"},
		{"/api/v1/clients/{id}/consents/{grant_id}/revoke", "post"},
		{"/api/v1/clinical-sessions/{id}/audio", "post"},
		{"/api/v1/clinical-sessions/{id}/artifacts", "get"},
		{"/api/v1/clinical-sessions/{id}/transcriptions", "post"},
		{"/api/v1/clinical-sessions/{id}/transcripts", "get"},
		{"/api/v1/clinical-sessions/{id}/jobs", "get"},
		{"/api/v1/clinical-sessions/{id}/analysis-jobs", "post"},
		{"/api/v1/clinical-transcripts/{id}", "get"},
		{"/api/v1/clinical-transcripts/{id}", "delete"},
		{"/api/v1/clinical-transcripts/{id}/revisions", "post"},
		{"/api/v1/clinical-artifacts/{id}", "delete"},
		{"/api/v1/clinical-jobs/{id}", "get"},
		{"/api/v1/clinical-jobs/{id}/retry", "post"},
		{"/api/v1/clinical-jobs/{id}/cancel", "post"},
		{"/api/v1/clinical-ingestion/health", "get"},
	} {
		p, ok := paths[route.path].(map[any]any)
		if !ok {
			t.Fatalf("undocumented path %s", route.path)
		}
		operation, ok := p[route.method].(map[any]any)
		if !ok {
			t.Fatalf("undocumented method %s %s", route.method, route.path)
		}
		security, ok := operation["security"].([]any)
		if !ok || len(security) != 1 {
			t.Fatal("missing authenticated tenant boundary")
		}
		auth := security[0].(map[any]any)
		if _, ok = auth["bearerAuth"]; !ok {
			t.Fatal("missing bearer")
		}
		if _, ok = auth["tenantHeader"]; !ok {
			t.Fatal("missing tenant")
		}
		responses := operation["responses"].(map[any]any)
		success := false
		for _, code := range []string{"200", "201", "202"} {
			if r, ok := responses[code].(map[any]any); ok {
				success = true
				if _, ok = r["content"]; !ok {
					t.Fatalf("missing typed success %s", route.path)
				}
			}
		}
		if !success {
			t.Fatal("no success response")
		}
	}
	schemas := root["components"].(map[any]any)["schemas"].(map[any]any)
	artifact := schemas["ClinicalAudioArtifact"].(map[any]any)["properties"].(map[any]any)
	if artifact["created_at"].(map[any]any)["format"] != "date-time" {
		t.Fatal("artifact creation timestamp contract missing")
	}
	sessionList := paths["/api/v1/clients/{client_id}/clinical-sessions"].(map[any]any)["get"].(map[any]any)["responses"].(map[any]any)["200"].(map[any]any)["content"].(map[any]any)["application/json"].(map[any]any)["schema"].(map[any]any)
	if sessionList["properties"].(map[any]any)["can_write"].(map[any]any)["type"] != "boolean" {
		t.Fatal("server write capability contract missing")
	}
	for _, name := range []string{"ClinicalConsentCommand", "ClinicalTranscriptionCommand", "ClinicalAnalysisJobCommand", "ClinicalTranscriptCorrectionCommand"} {
		schema := schemas[name].(map[any]any)
		if schema["additionalProperties"] != false || len(schema["required"].([]any)) == 0 {
			t.Fatalf("untyped command %s", name)
		}
	}
}
