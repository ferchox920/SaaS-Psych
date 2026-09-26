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
	"sessionflow/apps/api/internal/usecase/approvedcontext"
	"sessionflow/apps/api/internal/usecase/longitudinal"
	"strings"
	"testing"
	"time"
)

func TestStage3BReadProjectionHTTPPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newStrategyFixture(t, pool)
	repo := NewClinicalLongitudinalRepository(pool)
	run := startAcceptanceRun(t, pool, f.base)
	access := NewClinicalAccessRepository(pool)
	approved := approvedcontext.NewService(NewClinicalMemoryRepository(pool), NewSessionReportRepository(pool), access)
	svc := longitudinal.NewService(repo, access, approved, nil, nil, nil, "ollama", "synthetic", nil)
	h := handlers.NewClinicalLongitudinalHandler(svc)
	roles := []struct {
		actor uuid.UUID
		role  string
	}{{f.base.user, "treating"}, {uuid.New(), "supervisor"}, {uuid.New(), "unassigned"}}
	for i, r := range roles {
		if i > 0 {
			mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'synthetic')`, r.actor, f.base.tenant, r.actor.String()+"@example.invalid")
		}
		if r.role != "unassigned" {
			mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO client_clinical_assignments(id,tenant_id,client_id,user_id,relationship,granted_by_user_id,starts_at) VALUES($1,$2,$3,$4,$5,$6,NOW()-INTERVAL '1 hour')`, uuid.New(), f.base.tenant, f.base.client, r.actor, r.role, f.base.user)
		}
	}
	call := func(tenant, client, actor uuid.UUID, kind string, id uuid.UUID, fn func(echo.Context) error, status int) []byte {
		t.Helper()
		req := httptest.NewRequest("GET", "/?offset=0", nil)
		req = req.WithContext(middleware.WithPrincipal(middleware.WithTenantID(req.Context(), tenant), middleware.Principal{UserID: actor, Role: "member"}))
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)
		c.SetParamNames("id", "kind", "entity_id", "run_id")
		c.SetParamValues(client.String(), kind, id.String(), id.String())
		if e := fn(c); e != nil {
			t.Fatal(e)
		}
		if rec.Code != status {
			t.Fatalf("code=%d expected=%d body=%s", rec.Code, status, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	for _, r := range roles {
		t.Run(r.role, func(t *testing.T) {
			code := 200
			if r.role == "unassigned" {
				code = 403
			}
			call(f.base.tenant, f.base.client, r.actor, "process", f.process, h.History, code)
			call(f.base.tenant, f.base.client, r.actor, "", run, h.RunProvenance, code)
			call(f.base.tenant, f.base.client, r.actor, "", run, h.ApprovedContext, code)
			call(f.base.tenant, f.base.client, r.actor, "", run, h.EvidencePage, code)
		})
	}
	raw := call(f.base.tenant, f.base.client, f.base.user, "", run, h.RunProvenance, 200)
	for _, forbidden := range []string{"parameters_json", "storage_key", "prompt_text", "error_message"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatal("unsafe provenance field")
		}
	}
	var provenance longitudinal.RunProvenance
	if err = json.Unmarshal(raw, &provenance); err != nil || provenance.ID != run || provenance.PromptVersion != "clinical-longitudinal-interpreter-v1" {
		t.Fatal("wrong provenance")
	}
	other := newStrategyFixture(t, pool)
	call(f.base.tenant, f.base.client, f.base.user, "process", other.process, h.History, 404)
	call(f.base.tenant, f.base.client, f.base.user, "", startAcceptanceRun(t, pool, other.base), h.RunProvenance, 404)
	call(other.base.tenant, other.base.client, f.base.user, "process", other.process, h.History, 403)
	for i := 0; i < 27; i++ {
		_, report := f.base.addApprovedSessionReport(t, pool)
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,status,created_by_user_id,created_at) VALUES($1,$2,$3,'session_report',$4,1,'fact-002','patient_report','La persona describió temor al rechazo.','active',$5,$6)`, uuid.New(), f.base.tenant, f.base.client, report, f.base.user, time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC))
	}
	first, e := repo.EvidencePage(ctx, f.base.tenant, f.base.client, 0)
	if e != nil || len(first.Items) != 25 || !first.HasMore {
		t.Fatalf("page boundary: %+v %v", first, e)
	}
	second, e := repo.EvidencePage(ctx, f.base.tenant, f.base.client, 25)
	if e != nil || len(second.Items) != 3 || second.HasMore {
		t.Fatal("last page")
	}
	seen := map[uuid.UUID]bool{}
	for _, x := range first.Items {
		seen[x.ID] = true
	}
	for _, x := range second.Items {
		if seen[x.ID] || x.ClientID != f.base.client {
			t.Fatal("duplicate or foreign evidence")
		}
	}
	history, e := repo.HistoryPage(ctx, f.base.tenant, f.base.client, f.process, "process", 0)
	if e != nil || len(history.Items) != 1 || history.Items[0].ToVersion != 1 || history.Items[0].MergedByUserID == nil {
		t.Fatalf("history versions/actor: %+v %v", history, e)
	}
	payload, _ := json.Marshal(history)
	t.Logf("synthetic process history bytes=%d; evidence page bytes bounded by 25 entities", len(payload))
	started := time.Now()
	state, e := repo.State(ctx, f.base.tenant, f.base.client)
	if e != nil {
		t.Fatal(e)
	}
	stateBytes, _ := json.Marshal(state)
	t.Logf("synthetic overview processes=%d evidence=%d bytes=%d latency=%s", len(state.Processes), len(state.ActiveEvidence), len(stateBytes), time.Since(started))
}
