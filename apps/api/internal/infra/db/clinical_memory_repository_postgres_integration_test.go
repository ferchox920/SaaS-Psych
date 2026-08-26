package db

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"

	clinicalmemory "sessionflow/apps/api/internal/usecase/clinicalmemory"
)

func TestClinicalMemoryLifecyclePostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run clinical memory integration test")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenantID, actorID, clientID, appointmentID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants (id, name) VALUES ($1, 'Memory fixture')`, tenantID)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users (id, tenant_id, email, password_hash) VALUES ($1, $2, $3, 'hash')`, actorID, tenantID, actorID.String()+"@example.local")
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients (id, tenant_id, fullname) VALUES ($1, $2, 'Fictitious person')`, clientID, tenantID)
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO appointments (id, tenant_id, client_id, starts_at, ends_at, status, location) VALUES ($1, $2, $3, NOW(), NOW() + INTERVAL '1 hour', 'scheduled', '')`, appointmentID, tenantID, clientID)

	repo := NewClinicalMemoryRepository(pool)
	result := json.RawMessage(`{"mode":"live","node":"Ambivalencia ficticia"}`)
	suggestion, err := repo.CreateSuggestion(ctx, clinicalmemory.CreateSuggestionInput{
		TenantID: tenantID, ClientID: clientID, AppointmentID: appointmentID, ActorUserID: actorID,
		Mode: "live", PromptVersion: "test-v1", Result: result,
	})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := repo.DecideSuggestion(ctx, tenantID, suggestion.ID, actorID, "accepted", "", "")
	if err != nil || accepted.Disposition != "accepted" {
		t.Fatalf("accept suggestion: %+v err=%v", accepted, err)
	}
	green := "green"
	snapshot, err := repo.CreateSnapshot(ctx, tenantID, clientID, actorID, "Resumen aprobado ficticio.", []clinicalmemory.Anchor{
		{SourceID: "session_fact_1", Kind: "fact", Summary: "Hecho ficticio expresado directamente."},
		{SourceID: "accepted_suggestion_1", Kind: "hypothesis", Summary: "Hipótesis ficticia revisada.", TrafficLight: &green, SourceSuggestionID: &suggestion.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	approved, err := repo.ApproveSnapshot(ctx, tenantID, clientID, snapshot.ID, actorID)
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approve snapshot: %+v err=%v", approved, err)
	}
	contextItem, err := repo.GetApprovedContext(ctx, tenantID, clientID)
	if err != nil || contextItem.Version != 1 || len(contextItem.Anchors) != 2 {
		t.Fatalf("approved context mismatch: %+v err=%v", contextItem, err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1 AND action IN ('clinical_ai.suggestion.create', 'clinical_ai.suggestion.accepted', 'clinical_formulation.create_draft', 'clinical_formulation.approve')`, tenantID).Scan(&auditCount); err != nil || auditCount != 4 {
		t.Fatalf("expected four transactional audits, count=%d err=%v", auditCount, err)
	}
}
