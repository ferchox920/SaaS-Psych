package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

type fakeRateLimitStore struct {
	count int64
	err   error
}

func (f *fakeRateLimitStore) Increment(_ context.Context, _ string, _ time.Duration) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.count++
	return f.count, nil
}

func TestRequireLoginRateLimitBlocksWhenExceeded(t *testing.T) {
	e := echo.New()
	store := &fakeRateLimitStore{}
	mw := RequireLoginRateLimit(store, 2, time.Minute)

	handler := mw(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		if err := handler(e.NewContext(req, rec)); err != nil {
			t.Fatalf("handler run %d: %v", i+1, err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 before limit, got %d", rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	if err := handler(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handler run over limit: %v", err)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in payload")
	}
	if errObj["code"] != "too_many_requests" {
		t.Fatalf("expected code too_many_requests, got %v", errObj["code"])
	}
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected details object in payload")
	}
	if details["field"] != "ip" {
		t.Fatalf("expected details.field ip, got %v", details["field"])
	}
	if details["reason"] != "limit_exceeded" {
		t.Fatalf("expected details.reason limit_exceeded, got %v", details["reason"])
	}
}

func TestRequireLoginRateLimitFailsOpenOnStoreDisabled(t *testing.T) {
	e := echo.New()
	mw := RequireLoginRateLimit(nil, 0, 0)
	handler := mw(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	if err := handler(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handler run: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestRequireLoginRateLimitReturns503OnStoreError(t *testing.T) {
	e := echo.New()
	mw := RequireLoginRateLimit(&fakeRateLimitStore{err: errors.New("redis down")}, 5, time.Minute)
	handler := mw(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	if err := handler(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handler run: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
