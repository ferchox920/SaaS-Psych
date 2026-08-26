package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsOnlyConfiguredWebOriginWithCredentials(t *testing.T) {
	server := NewServer(ServerDeps{WebOrigin: "http://localhost:3000"})

	allowed := httptest.NewRequest(stdhttp.MethodOptions, "/health", nil)
	allowed.Header.Set("Origin", "http://localhost:3000")
	allowed.Header.Set("Access-Control-Request-Method", stdhttp.MethodGet)
	allowedRecorder := httptest.NewRecorder()
	server.ServeHTTP(allowedRecorder, allowed)
	if got := allowedRecorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected explicit allowed origin, got %q", got)
	}
	if got := allowedRecorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentialed CORS, got %q", got)
	}

	denied := httptest.NewRequest(stdhttp.MethodOptions, "/health", nil)
	denied.Header.Set("Origin", "https://untrusted.example")
	denied.Header.Set("Access-Control-Request-Method", stdhttp.MethodGet)
	deniedRecorder := httptest.NewRecorder()
	server.ServeHTTP(deniedRecorder, denied)
	if got := deniedRecorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("untrusted origin must not receive CORS permission, got %q", got)
	}
}
