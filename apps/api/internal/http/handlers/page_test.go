package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestParseVisibleListPage(t *testing.T) {
	for _, tc := range []struct {
		query         string
		limit, offset int
		invalid       bool
	}{
		{query: "", limit: 50},
		{query: "?limit=100&offset=200", limit: 100, offset: 200},
		{query: "?limit=0", invalid: true},
		{query: "?limit=101", invalid: true},
		{query: "?offset=-1", invalid: true},
		{query: "?offset=1000001", invalid: true},
		{query: "?offset=abc", invalid: true},
	} {
		e := echo.New()
		c := e.NewContext(httptest.NewRequest("GET", "/clients"+tc.query, nil), httptest.NewRecorder())
		limit, offset, valid := parsePage(c)
		if valid == tc.invalid || (!tc.invalid && (limit != tc.limit || offset != tc.offset)) || (tc.invalid && c.Response().Status != 400) {
			t.Errorf("query %q: limit=%d offset=%d valid=%v status=%d", tc.query, limit, offset, valid, c.Response().Status)
		}
	}
}
