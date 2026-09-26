package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestDocsUI(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := DocsUI(c); err != nil {
		t.Fatalf("docs ui: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body == "" {
		t.Fatalf("expected docs html body")
	}
}

func TestLongitudinalOpenAPIReflectsRuntimeStatusAndSchemas(t *testing.T) {
	spec := string(embeddedOpenAPISpec)
	for _, required := range []string{`"400": {description: La sesión no está completada`, `"404": {description: Sesión o SessionReport aprobado no encontrado}`, `ClinicalEvidenceList:`, `ClinicalEventList:`, `ClinicalProcessList:`, `ClinicalHypothesisList:`, `ClinicalDiffList:`, `/api/v1/processes/{id}/gira-analysis:`, `ClinicalTargetList:`, `ClinicalGoalList:`, `GIRAList:`, `ApproachDefinitionList:`, `TechniqueDefinitionList:`, `GIRAAnalysisResponse:`} {
		if !strings.Contains(spec, required) {
			t.Fatalf("OpenAPI longitudinal contract missing %q", required)
		}
	}
}

func TestOpenAPISpec(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := OpenAPISpec(c); err != nil {
		t.Fatalf("openapi spec: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); contentType == "" {
		t.Fatalf("expected content-type")
	}
	if rec.Body.Len() == 0 {
		t.Fatalf("expected spec body")
	}
}

func TestOpenAPISpecDoesNotDependOnSourceTree(t *testing.T) {
	path := canonicalOpenAPISpecPath(t)
	backup := path + ".test-backup"
	if err := os.Rename(path, backup); err != nil {
		t.Fatalf("make source spec unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Rename(backup, path); err != nil {
			t.Errorf("restore source spec: %v", err)
		}
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := OpenAPISpec(c); err != nil {
		t.Fatalf("embedded openapi spec: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without source tree, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "openapi:") {
		t.Fatalf("expected embedded OpenAPI document, got %q", rec.Body.String())
	}
}

func TestEmbeddedOpenAPISpecMatchesCanonicalDocument(t *testing.T) {
	canonical, err := os.ReadFile(canonicalOpenAPISpecPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, embeddedOpenAPISpec) {
		t.Fatal("embedded OpenAPI spec is stale; copy docs/openapi.yaml to internal/http/handlers/openapi.yaml")
	}
}

func canonicalOpenAPISpecPath(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve docs_test.go path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..", "..", "docs", "openapi.yaml"))
}
