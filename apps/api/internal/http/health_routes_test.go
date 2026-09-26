package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLivenessAndReadinessAreSeparateContracts(t *testing.T) {
	server := NewServer(ServerDeps{})

	live := httptest.NewRecorder()
	server.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/health", nil))
	if live.Code != http.StatusOK || !strings.Contains(live.Body.String(), `"status":"ok"`) {
		t.Fatalf("liveness must only report the running process, got %d %s", live.Code, live.Body.String())
	}

	ready := httptest.NewRecorder()
	server.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if ready.Code != http.StatusServiceUnavailable || !strings.Contains(ready.Body.String(), `"status":"not_ready"`) {
		t.Fatalf("unconfigured dependencies must not report ready, got %d %s", ready.Code, ready.Body.String())
	}
}
