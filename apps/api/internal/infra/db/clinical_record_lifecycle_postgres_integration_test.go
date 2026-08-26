package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainclient "sessionflow/apps/api/internal/domain/client"
	"sessionflow/apps/api/internal/requestcontext"
)

func TestClientAndAppointmentWritesAreAssignedAndAuditedTransactionallyPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run clinical lifecycle integration test")
	}
	ctx := requestcontext.WithRequestID(context.Background(), "clinical-lifecycle-test-request")
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	defer pool.Close()

	tenantID, actorID := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2)`, tenantID, fmt.Sprintf("lifecycle-%s", tenantID))
	mustExecIntegrationSQL(t, pool, ctx, `
		INSERT INTO users (id, tenant_id, email, password_hash)
		VALUES ($1, $2, $3, 'hash')
	`, actorID, tenantID, fmt.Sprintf("clinician-%s@example.local", actorID))

	now := time.Now().UTC()
	client, err := domainclient.NewEntity(tenantID, "Fictitious Client", "", "", now)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	clientRepo := NewClientRepository(pool).WithTransactionalAudit()
	created, err := clientRepo.Create(ctx, client, actorID)
	if err != nil {
		t.Fatalf("create assigned client: %v", err)
	}
	var activeTreatingAssignments int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM client_clinical_assignments
		WHERE tenant_id = $1 AND client_id = $2 AND user_id = $3
		  AND relationship = 'treating' AND ends_at IS NULL
	`, tenantID, created.ID, actorID).Scan(&activeTreatingAssignments); err != nil || activeTreatingAssignments != 1 {
		t.Fatalf("creator must receive treating assignment atomically, count=%d err=%v", activeTreatingAssignments, err)
	}

	created.FullName = "Fictitious Client Updated"
	created.UpdatedAt = now.Add(time.Minute)
	if _, err := clientRepo.Update(ctx, created, actorID); err != nil {
		t.Fatalf("update client: %v", err)
	}
	if err := clientRepo.Archive(ctx, tenantID, created.ID, actorID, "care completed", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("archive client: %v", err)
	}
	archived, err := clientRepo.ListArchived(ctx, tenantID)
	if err != nil || len(archived) != 1 || archived[0].ID != created.ID || archived[0].ArchiveReason != "care completed" {
		t.Fatalf("archived client history mismatch: %+v err=%v", archived, err)
	}
	if err := clientRepo.Restore(ctx, tenantID, created.ID, actorID, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("restore client: %v", err)
	}

	appointment, err := domainappointment.NewEntity(
		tenantID, created.ID, now.Add(time.Hour), now.Add(2*time.Hour), "Fictitious room", now,
	)
	if err != nil {
		t.Fatalf("new appointment: %v", err)
	}
	appointmentRepo := NewAppointmentRepository(pool).WithTransactionalAudit()
	appointment, err = appointmentRepo.Create(ctx, appointment, actorID)
	if err != nil {
		t.Fatalf("create appointment: %v", err)
	}
	appointment.Location = "Fictitious room 2"
	appointment.UpdatedAt = now.Add(4 * time.Minute)
	if _, err := appointmentRepo.Update(ctx, appointment, actorID, "appointment.update"); err != nil {
		t.Fatalf("update appointment: %v", err)
	}
	appointment.Status = domainappointment.StatusCanceled
	appointment.UpdatedAt = now.Add(5 * time.Minute)
	if _, err := appointmentRepo.Update(ctx, appointment, actorID, "appointment.cancel"); err != nil {
		t.Fatalf("cancel appointment: %v", err)
	}

	var clientAuditCount, appointmentAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE tenant_id = $1 AND entity_id = $2
		  AND action IN ('client.create', 'client.update', 'client.archive', 'client.restore')
	`, tenantID, created.ID).Scan(&clientAuditCount); err != nil || clientAuditCount != 4 {
		t.Fatalf("expected four transactional client audits, count=%d err=%v", clientAuditCount, err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE tenant_id = $1 AND entity_id = $2
		  AND action IN ('appointment.create', 'appointment.update', 'appointment.cancel')
	`, tenantID, appointment.ID).Scan(&appointmentAuditCount); err != nil || appointmentAuditCount != 3 {
		t.Fatalf("expected three transactional appointment audits, count=%d err=%v", appointmentAuditCount, err)
	}
	var correlatedAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE tenant_id = $1 AND metadata->>'request_id' = 'clinical-lifecycle-test-request'
	`, tenantID).Scan(&correlatedAuditCount); err != nil || correlatedAuditCount < 8 {
		t.Fatalf("expected transactional audits correlated by request_id, count=%d err=%v", correlatedAuditCount, err)
	}
}
