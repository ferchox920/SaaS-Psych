package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
)

func TestClinicalStrategyHandlersRejectMalformedIdentifiers(t *testing.T) {
	handler := NewClinicalLongitudinalHandler(nil)
	for _, tc := range []struct {
		name, method, path, param string
		call                      func(echo.Context) error
	}{
		{name: "gira analysis", method: http.MethodPost, path: "/api/v1/processes/not-a-uuid/gira-analysis", param: "not-a-uuid", call: handler.AnalyzeGIRA},
		{name: "get gira", method: http.MethodGet, path: "/api/v1/giras/not-a-uuid", param: "not-a-uuid", call: handler.GetGIRA},
		{name: "list targets", method: http.MethodGet, path: "/api/v1/clients/not-a-uuid/targets", param: "not-a-uuid", call: handler.Targets},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			ctx := httpmiddleware.WithTenantID(req.Context(), uuid.New())
			ctx = httpmiddleware.WithPrincipal(ctx, httpmiddleware.Principal{UserID: uuid.New(), Role: "member"})
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("id")
			c.SetParamValues(tc.param)
			if err := tc.call(c); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
