package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

type projectTestAccess struct {
	allowed bool
	roles   []string
}

func (a *projectTestAccess) CanAccessClient(_ context.Context, _, _, _ uuid.UUID, roles ...string) (bool, error) {
	a.roles = roles
	return a.allowed, nil
}
func TestProjectHTTPAuthorizationAndStrictExportBody(t *testing.T) {
	for _, test := range []struct {
		name, body string
		call       string
		allowed    bool
		want       int
	}{
		{"export denied", `{}`, "export", false, 403},
		{"import denied", `{}`, "import", false, 403},
		{"export unknown field", `{"patient_name":"synthetic"}`, "export", true, 400},
		{"export trailing", `{} {}`, "export", true, 400},
		{"import invalid", `{"schema_version":"wrong"}`, "import", true, 400},
		{"import too large", strings.Repeat("x", 1024*1024+1), "import", true, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			access := &projectTestAccess{allowed: test.allowed}
			service := longitudinal.NewService(nil, access, nil, nil, nil, nil, "", "", nil)
			handler := NewClinicalLongitudinalHandler(service)
			request := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
			ctx := httpmiddleware.WithTenantID(request.Context(), uuid.New())
			ctx = httpmiddleware.WithPrincipal(ctx, httpmiddleware.Principal{UserID: uuid.New(), Role: "member"})
			request = request.WithContext(ctx)
			recorder := httptest.NewRecorder()
			c := echo.New().NewContext(request, recorder)
			c.SetParamNames("id")
			c.SetParamValues(uuid.NewString())
			var err error
			if test.call == "export" {
				err = handler.ExportProject(c)
			} else {
				err = handler.ImportProject(c)
			}
			if err != nil {
				t.Fatal(err)
			}
			if recorder.Code != test.want {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if len(access.roles) > 0 && strings.Join(access.roles, ",") != "treating" {
				t.Fatal("write did not require treating")
			}
		})
	}
}
