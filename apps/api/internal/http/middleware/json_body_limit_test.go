package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestJSONBodyLimitRejectsOversizeWithoutRestrictingRawAudio(t *testing.T) {
	e := echo.New()
	e.Use(JSONBodyLimit(1024))
	e.POST("/body", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	for _, tc := range []struct {
		name, contentType string
		want              int
	}{
		{"JSON", "application/json", http.StatusRequestEntityTooLarge},
		{"JSON with charset", "application/json; charset=utf-8", http.StatusRequestEntityTooLarge},
		{"raw audio", "audio/webm", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/body", strings.NewReader(strings.Repeat("x", 1025)))
			req.Header.Set(echo.HeaderContentType, tc.contentType)
			req.ContentLength = -1
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d", rec.Code, tc.want)
			}
		})
	}
}
