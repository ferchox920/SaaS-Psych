package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
	clinicalmemory "sessionflow/apps/api/internal/usecase/clinicalmemory"
)

func TestClinicalAIRunProvenanceAndSuggestionLinkPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, other, user, client, appointment := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'run tenant'),($2,'other run tenant')`, tenant, other)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'hash')`, user, tenant, user.String()+"@run.test")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'run client')`, client, tenant)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments(id,tenant_id,client_id,starts_at,ends_at,status) VALUES($1,$2,$3,NOW(),NOW()+INTERVAL '1 hour','scheduled')`, appointment, tenant, client)
	memory := NewClinicalMemoryRepository(pool)
	snapshot, err := memory.CreateSnapshot(ctx, tenant, client, user, "approved fixture", []clinicalmemory.Anchor{{SourceID: "fact-001", Kind: "fact", Summary: "approved fixture fact"}})
	if err != nil {
		t.Fatal(err)
	}
	approved, err := memory.ApproveSnapshot(ctx, tenant, client, snapshot.ID, user)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewClinicalAIRunRepository(pool)
	service := clinicalairun.NewService(repo).WithBuildInfo("2a.1-test", "deadbeef")
	version := approved.Version
	run, err := service.Start(ctx, clinicalairun.StartInput{TenantID: tenant, ClientID: client, AppointmentID: &appointment, CreatedByUserID: user, Provider: "ollama", Model: "fixture", Operation: "review_session", PromptName: "clinical-review", PromptVersion: "clinical-review-v1", Parameters: map[string]any{"temperature": 0.1}, Input: "ephemeral clinical input", Sources: []clinicalairun.Source{{SourceType: clinicalairun.SourceFormulationSnapshot, SourceID: approved.ID, SourceVersion: &version}, {SourceType: clinicalairun.SourceFormulationAnchor, SourceID: approved.Anchors[0].ID, SourceVersion: &version}}})
	if err != nil {
		t.Fatal(err)
	}
	if run.AppVersion != "2a.1-test" || run.BuildRevision != "deadbeef" {
		t.Fatalf("build provenance=%#v", run)
	}
	sources, err := service.ListSources(ctx, tenant, run.ID)
	if err != nil || len(sources) != 2 {
		t.Fatalf("sources=%#v err=%v", sources, err)
	}
	foreignSources, err := service.ListSources(ctx, other, run.ID)
	if err != nil || len(foreignSources) != 0 {
		t.Fatalf("cross-tenant sources=%#v err=%v", foreignSources, err)
	}
	draftSnapshot, err := memory.CreateSnapshot(ctx, tenant, client, user, "draft fixture", []clinicalmemory.Anchor{})
	if err != nil {
		t.Fatal(err)
	}
	draftVersion := draftSnapshot.Version
	if _, err := service.Start(ctx, clinicalairun.StartInput{TenantID: tenant, ClientID: client, AppointmentID: &appointment, CreatedByUserID: user, Provider: "ollama", Model: "fixture", Operation: "draft_source_rejection", PromptName: "fixture", PromptVersion: "v1", Parameters: map[string]any{}, Input: "ephemeral", Sources: []clinicalairun.Source{{SourceType: clinicalairun.SourceFormulationSnapshot, SourceID: draftSnapshot.ID, SourceVersion: &draftVersion}}}); err == nil {
		t.Fatal("draft formulation must not be persisted as an approved run source")
	}
	var provenanceContentColumns int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='clinical_ai_run_sources' AND column_name IN ('content','report_json','transcript','prompt')`).Scan(&provenanceContentColumns); err != nil || provenanceContentColumns != 0 {
		t.Fatalf("provenance duplicates clinical content: count=%d err=%v", provenanceContentColumns, err)
	}
	if _, err := repo.Finish(ctx, other, run.ID, clinicalairun.StatusFailed, nil, stringPointer("cross_tenant"), time.Now().UTC()); err == nil {
		t.Fatal("cross tenant finish must fail")
	}
	finished, err := service.Succeed(ctx, tenant, run.ID, map[string]any{"mode": "review"})
	if err != nil || finished.OutputHash == nil {
		t.Fatalf("finished=%#v err=%v", finished, err)
	}
	suggestion, err := memory.CreateSuggestion(ctx, clinicalmemory.CreateSuggestionInput{TenantID: tenant, ClientID: client, AppointmentID: appointment, ActorUserID: user, Mode: "review", PromptVersion: "clinical-review-v1", Result: []byte(`{"mode":"review"}`), AIRunID: &run.ID})
	if err != nil {
		t.Fatal(err)
	}
	if suggestion.AIRunID == nil || *suggestion.AIRunID != run.ID {
		t.Fatalf("missing run link: %#v", suggestion)
	}
	var leaked int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE tenant_id=$1 AND entity_id=$2 AND (metadata::text ILIKE '%ephemeral clinical input%' OR metadata::text ILIKE '%prompt completo%')`, tenant, run.ID).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("clinical content leaked to audit: count=%d err=%v", leaked, err)
	}
}
func stringPointer(value string) *string { return &value }
