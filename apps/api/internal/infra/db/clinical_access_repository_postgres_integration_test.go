package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	clinicalaccessusecase "sessionflow/apps/api/internal/usecase/clinicalaccess"
)

func TestClinicalAccessRepositoryRequiresActiveTenantAwareAssignmentPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run clinical access integration test")
	}

	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	defer pool.Close()

	tenantA, tenantB := uuid.New(), uuid.New()
	treatingUser, supervisorUser, foreignUser, emergencyUser := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	clientID, appointmentID := uuid.New(), uuid.New()
	now := time.Now().UTC()

	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2), ($3, $4)`,
		tenantA, fmt.Sprintf("clinical-a-%s", tenantA), tenantB, fmt.Sprintf("clinical-b-%s", tenantB),
	)
	mustExecIntegrationSQL(t, pool, ctx, `
		INSERT INTO users (id, tenant_id, email, password_hash) VALUES
			($1, $2, $3, 'hash'),
			($4, $2, $5, 'hash'),
			($6, $7, $8, 'hash'),
			($9, $2, $10, 'hash')
	`,
		treatingUser, tenantA, fmt.Sprintf("treating-%s@example.local", treatingUser),
		supervisorUser, fmt.Sprintf("supervisor-%s@example.local", supervisorUser),
		foreignUser, tenantB, fmt.Sprintf("foreign-%s@example.local", foreignUser),
		emergencyUser, fmt.Sprintf("emergency-%s@example.local", emergencyUser),
	)
	mustExecIntegrationSQL(t, pool, ctx, `
		INSERT INTO clients (id, tenant_id, fullname, contact, notes_public)
		VALUES ($1, $2, 'Fictitious Client', '', '')
	`, clientID, tenantA)
	mustExecIntegrationSQL(t, pool, ctx, `
		INSERT INTO appointments (id, tenant_id, client_id, starts_at, ends_at, status, location)
		VALUES ($1, $2, $3, $4, $5, 'scheduled', '')
	`, appointmentID, tenantA, clientID, now.Add(time.Hour), now.Add(2*time.Hour))
	mustExecIntegrationSQL(t, pool, ctx, `
		INSERT INTO client_clinical_assignments
			(tenant_id, client_id, user_id, relationship, granted_by_user_id, starts_at)
		VALUES
			($1, $2, $3, 'treating', $3, $4),
			($1, $2, $5, 'supervisor', $3, $4)
	`, tenantA, clientID, treatingUser, now.Add(-time.Hour), supervisorUser)

	repo := NewClinicalAccessRepository(pool)
	repo.now = func() time.Time { return now }

	allowed, err := repo.CanAccessAppointment(ctx, tenantA, treatingUser, appointmentID, "treating")
	if err != nil || !allowed {
		t.Fatalf("active treating assignment should grant access: allowed=%v err=%v", allowed, err)
	}
	allowed, err = repo.CanAccessAppointment(ctx, tenantA, supervisorUser, appointmentID, "treating", "supervisor")
	if err != nil || !allowed {
		t.Fatalf("active supervisor assignment should grant read access: allowed=%v err=%v", allowed, err)
	}
	allowed, err = repo.CanAccessAppointment(ctx, tenantA, supervisorUser, appointmentID, "treating")
	if err != nil || allowed {
		t.Fatalf("supervisor assignment must not grant treating authority: allowed=%v err=%v", allowed, err)
	}
	allowed, err = repo.CanAccessAppointment(ctx, tenantB, foreignUser, appointmentID, "treating", "supervisor")
	if err != nil || allowed {
		t.Fatalf("cross-tenant access must be denied: allowed=%v err=%v", allowed, err)
	}

	assignments, err := repo.List(ctx, tenantA, clientID, supervisorUser)
	if err != nil || len(assignments) != 2 {
		t.Fatalf("expected assignment history, items=%d err=%v", len(assignments), err)
	}
	var treatingAssignmentID uuid.UUID
	for _, assignment := range assignments {
		if assignment.UserID == treatingUser && assignment.Relationship == "treating" {
			treatingAssignmentID = assignment.ID
		}
	}
	if treatingAssignmentID == uuid.Nil {
		t.Fatal("missing treating assignment in history")
	}
	if err := repo.End(ctx, tenantA, clientID, treatingAssignmentID, treatingUser, "care transferred", now); err != nil {
		t.Fatalf("end treating assignment: %v", err)
	}
	allowed, err = repo.CanAccessAppointment(ctx, tenantA, treatingUser, appointmentID, "treating")
	if err != nil || allowed {
		t.Fatalf("ended assignment must not grant access: allowed=%v err=%v", allowed, err)
	}
	granted, err := repo.Grant(ctx, clinicalaccessusecase.Assignment{
		ID: uuid.New(), TenantID: tenantA, ClientID: clientID, UserID: treatingUser,
		Relationship: "treating", GrantedByUserID: supervisorUser,
		StartsAt: now.Add(time.Second), CreatedAt: now.Add(time.Second),
	})
	if err != nil || granted.ID == uuid.Nil {
		t.Fatalf("grant replacement treating assignment: %+v err=%v", granted, err)
	}
	var assignmentAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE tenant_id = $1 AND entity = 'clinical_assignment'
		  AND action IN ('clinical_assignment.grant', 'clinical_assignment.end', 'clinical_assignment.list')
	`, tenantA).Scan(&assignmentAuditCount); err != nil || assignmentAuditCount != 3 {
		t.Fatalf("expected assignment audit events, count=%d err=%v", assignmentAuditCount, err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO client_clinical_assignments
			(tenant_id, client_id, user_id, relationship, granted_by_user_id)
		VALUES ($1, $2, $3, 'treating', $4)
	`, tenantA, clientID, foreignUser, treatingUser); err == nil {
		t.Fatal("expected database to reject a cross-tenant clinical assignment")
	}

	allowed, err = repo.CanAccessAppointment(ctx, tenantA, emergencyUser, appointmentID, "treating", "supervisor")
	if err != nil || allowed {
		t.Fatalf("unassigned user must start without access: allowed=%v err=%v", allowed, err)
	}
	exception, err := repo.GrantException(ctx, clinicalaccessusecase.AccessException{
		ID: uuid.New(), TenantID: tenantA, ClientID: clientID, UserID: emergencyUser,
		GrantedByUserID: treatingUser, Reason: "urgent continuity review", Purpose: "read-only chart review",
		StartsAt: now, ExpiresAt: now.Add(2 * time.Hour), CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("grant access exception: %v", err)
	}
	allowed, err = repo.CanAccessAppointment(ctx, tenantA, emergencyUser, appointmentID, "treating")
	if err != nil || allowed {
		t.Fatalf("exception must never grant treating/write authority: allowed=%v err=%v", allowed, err)
	}
	allowed, err = repo.CanAccessAppointment(ctx, tenantA, emergencyUser, appointmentID, "treating", "supervisor")
	if err != nil || !allowed {
		t.Fatalf("active exception should grant bounded read access: allowed=%v err=%v", allowed, err)
	}
	if err := repo.RevokeException(ctx, tenantA, clientID, exception.ID, treatingUser, "review completed", now.Add(time.Minute)); err != nil {
		t.Fatalf("revoke access exception: %v", err)
	}
	allowed, err = repo.CanAccessAppointment(ctx, tenantA, emergencyUser, appointmentID, "treating", "supervisor")
	if err != nil || allowed {
		t.Fatalf("revoked exception must stop access immediately: allowed=%v err=%v", allowed, err)
	}
}
