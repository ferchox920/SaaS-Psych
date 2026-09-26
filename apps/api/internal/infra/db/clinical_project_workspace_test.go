package db

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	"os"
	"sessionflow/apps/api/internal/http/handlers"
	middleware "sessionflow/apps/api/internal/http/middleware"
	"sessionflow/apps/api/internal/usecase/longitudinal"
	"strings"
	"testing"
)

func TestStage3CExportHistoryHTTPPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newStrategyFixture(t, pool)
	service, repo, _ := newLongitudinalAcceptanceService(pool, nil, true)
	for i := 0; i < 27; i++ {
		if _, err = service.ExportProject(ctx, f.base.tenant, f.base.client, f.base.user, f.process); err != nil {
			t.Fatal(err)
		}
	}
	access := NewClinicalAccessRepository(pool)
	svc := longitudinal.NewService(repo, access, nil, nil, nil, nil, "none", "synthetic", nil).WithConsent(NewClinicalConsentRepository(pool))
	h := handlers.NewClinicalLongitudinalHandler(svc)
	actors := []uuid.UUID{f.base.user, uuid.New(), uuid.New()}
	for i, a := range actors {
		if i > 0 {
			mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'synthetic')`, a, f.base.tenant, a.String()+"@example.invalid")
		}
		if i < 2 {
			role := "treating"
			if i == 1 {
				role = "supervisor"
			}
			mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO client_clinical_assignments(id,tenant_id,client_id,user_id,relationship,granted_by_user_id,starts_at) VALUES($1,$2,$3,$4,$5,$6,NOW()-INTERVAL '1 hour')`, uuid.New(), f.base.tenant, f.base.client, a, role, f.base.user)
		}
	}
	call := func(actor, tenant, client uuid.UUID, query string, code int) longitudinal.ProjectExportPage {
		t.Helper()
		req := httptest.NewRequest("GET", "/?"+query, nil)
		req = req.WithContext(middleware.WithPrincipal(middleware.WithTenantID(req.Context(), tenant), middleware.Principal{UserID: actor, Role: "member"}))
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(client.String())
		if e := h.ProjectExports(c); e != nil {
			t.Fatal(e)
		}
		if rec.Code != code {
			t.Fatalf("status %d want %d", rec.Code, code)
		}
		var p longitudinal.ProjectExportPage
		if code == 200 {
			if e := json.Unmarshal(rec.Body.Bytes(), &p); e != nil {
				t.Fatal(e)
			}
			for _, secret := range []string{"artifact_json", "local_snapshot", "statement", "tenant_id", "client_id", "generated_by_user_id"} {
				if strings.Contains(rec.Body.String(), secret) {
					t.Fatal("history exposed hidden data")
				}
			}
		}
		return p
	}
	first := call(actors[0], f.base.tenant, f.base.client, "offset=0", 200)
	second := call(actors[1], f.base.tenant, f.base.client, "offset=25", 200)
	if !first.RequiresExternalConsent || len(first.Items) != 25 || !first.HasMore || len(second.Items) != 2 || second.HasMore {
		t.Fatal("pagination/policy")
	}
	ids := map[uuid.UUID]bool{}
	for _, x := range first.Items {
		ids[x.ID] = true
		if x.ProcessRef != "process_1" || x.ContentHash == "" {
			t.Fatal("metadata")
		}
	}
	for _, x := range second.Items {
		if ids[x.ID] {
			t.Fatal("overlap")
		}
	}
	call(actors[2], f.base.tenant, f.base.client, "", 403)
	call(actors[0], uuid.New(), f.base.client, "", 403)
	call(actors[0], f.base.tenant, uuid.New(), "", 403)
	call(actors[0], f.base.tenant, f.base.client, "offset=-1", 400)
	other, err := repo.ProjectExports(ctx, f.base.tenant, uuid.New(), 0)
	if err != nil || len(other.Items) != 0 {
		t.Fatal("client isolation")
	}
	if _, err = svc.GetProjectExport(ctx, f.base.tenant, f.base.client, actors[1], first.Items[0].ID); err == nil {
		t.Fatal("artifact retrieval bypassed external consent")
	}
}
