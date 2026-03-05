package handlers

import (
	"net/http"
	"net/http/httptest"
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
