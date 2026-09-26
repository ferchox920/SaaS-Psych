package db

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

func TestConcurrentOverlappingAppointmentCreatesCannotBothCommit(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run PostgreSQL appointment concurrency test")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	tenantID, clientID := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'Synthetic scheduling tenant')`, tenantID)
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID) }()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname,contact,notes_public) VALUES($1,$2,'Synthetic client','','')`, clientID, tenantID)

	repo := NewAppointmentRepository(pool)
	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	barrier := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entity, newErr := domainappointment.NewEntity(tenantID, clientID, start, start.Add(time.Hour), "Synthetic room", time.Now().UTC())
			if newErr != nil {
				results <- newErr
				return
			}
			<-barrier
			_, createErr := repo.Create(ctx, entity, uuid.Nil)
			results <- createErr
		}()
	}
	close(barrier)
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, domainerrors.ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected create result: %v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one reservation and one conflict, got successes=%d conflicts=%d", successes, conflicts)
	}
	var stored int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM appointments WHERE tenant_id=$1 AND status='scheduled'`, tenantID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 1 {
		t.Fatalf("expected one durable reservation, got %d", stored)
	}

	var existingID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM appointments WHERE tenant_id=$1 AND status='scheduled'`, tenantID).Scan(&existingID); err != nil {
		t.Fatal(err)
	}
	move, err := repo.GetByID(ctx, tenantID, existingID)
	if err != nil {
		t.Fatal(err)
	}
	move.StartsAt = start.Add(2 * time.Hour)
	move.EndsAt = start.Add(3 * time.Hour)
	createAtTarget, err := domainappointment.NewEntity(tenantID, clientID, move.StartsAt, move.EndsAt, "Synthetic room", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	barrier = make(chan struct{})
	results = make(chan error, 2)
	wg = sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-barrier
		_, updateErr := repo.Update(ctx, move, uuid.Nil, "appointment.update")
		results <- updateErr
	}()
	go func() {
		defer wg.Done()
		<-barrier
		_, createErr := repo.Create(ctx, createAtTarget, uuid.Nil)
		results <- createErr
	}()
	close(barrier)
	wg.Wait()
	close(results)
	successes, conflicts = 0, 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, domainerrors.ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected create versus edit result: %v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one winner for create versus edit, got successes=%d conflicts=%d", successes, conflicts)
	}
	var overlapPairs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM appointments a JOIN appointments b ON a.tenant_id=b.tenant_id AND a.id<b.id AND a.status<>'canceled' AND b.status<>'canceled' AND a.starts_at<b.ends_at AND b.starts_at<a.ends_at WHERE a.tenant_id=$1`, tenantID).Scan(&overlapPairs); err != nil {
		t.Fatal(err)
	}
	if overlapPairs != 0 {
		t.Fatalf("create versus edit committed %d overlapping pairs", overlapPairs)
	}

	cancelStart := start.Add(6 * time.Hour)
	cancelSlot, err := domainappointment.NewEntity(tenantID, clientID, cancelStart, cancelStart.Add(time.Hour), "Synthetic room", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	cancelSlot, err = repo.Create(ctx, cancelSlot, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	cancelSlot.Status = domainappointment.StatusCanceled
	if _, err := repo.Update(ctx, cancelSlot, uuid.Nil, "appointment.cancel"); err != nil {
		t.Fatal(err)
	}
	for _, slot := range [][2]time.Time{
		{cancelStart, cancelStart.Add(time.Hour)},
		{cancelStart.Add(time.Hour), cancelStart.Add(2 * time.Hour)},
	} {
		adjacent, newErr := domainappointment.NewEntity(tenantID, clientID, slot[0], slot[1], "Synthetic room", time.Now().UTC())
		if newErr != nil {
			t.Fatal(newErr)
		}
		if _, createErr := repo.Create(ctx, adjacent, uuid.Nil); createErr != nil {
			t.Fatalf("canceled or adjacent slot was rejected: %v", createErr)
		}
	}
	otherTenantID, otherClientID := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'Other synthetic scheduling tenant')`, otherTenantID)
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, otherTenantID) }()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname,contact,notes_public) VALUES($1,$2,'Other synthetic client','','')`, otherClientID, otherTenantID)
	otherSlot, err := domainappointment.NewEntity(otherTenantID, otherClientID, cancelStart, cancelStart.Add(time.Hour), "Synthetic room", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, otherSlot, uuid.Nil); err != nil {
		t.Fatalf("another tenant must be able to reserve the same time: %v", err)
	}
}

func TestConcurrentAppointmentEditCannotUndoCancellation(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run PostgreSQL appointment version test")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenantID, clientID := uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'Synthetic version tenant')`, tenantID)
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID) }()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO clients(id,tenant_id,fullname,contact,notes_public) VALUES($1,$2,'Synthetic client','','')`, clientID, tenantID)
	repo := NewAppointmentRepository(pool)
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	initial, err := domainappointment.NewEntity(tenantID, clientID, start, start.Add(time.Hour), "Synthetic room", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	created, err := repo.Create(ctx, initial, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	edit, cancellation := created, created
	edit.StartsAt = edit.StartsAt.Add(2 * time.Hour)
	edit.EndsAt = edit.EndsAt.Add(2 * time.Hour)
	cancellation.Status = domainappointment.StatusCanceled

	barrier := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, candidate := range []domainappointment.Entity{edit, cancellation} {
		wg.Add(1)
		go func(candidate domainappointment.Entity) {
			defer wg.Done()
			<-barrier
			_, updateErr := repo.Update(ctx, candidate, uuid.Nil, "appointment.update")
			results <- updateErr
		}(candidate)
	}
	close(barrier)
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, domainerrors.ErrConflict):
			conflicts++
		default:
			t.Fatalf("unexpected update result: %v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one update and one stale conflict, got successes=%d conflicts=%d", successes, conflicts)
	}
	stored, err := repo.GetByID(ctx, tenantID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status == domainappointment.StatusCanceled && !stored.StartsAt.Equal(created.StartsAt) {
		t.Fatal("stale edit changed a cancelled appointment")
	}
	if stored.Status == domainappointment.StatusScheduled && !stored.StartsAt.Equal(edit.StartsAt) {
		t.Fatal("stale cancellation changed an edited appointment")
	}
}
