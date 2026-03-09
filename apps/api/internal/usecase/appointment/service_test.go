package appointment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type fakeRepo struct {
	existsOverlapFn func(ctx context.Context, tenantID uuid.UUID, startsAt, endsAt time.Time, excludeID *uuid.UUID) (bool, error)
	clientExistsFn  func(ctx context.Context, tenantID, clientID uuid.UUID) (bool, error)
	createFn        func(ctx context.Context, in domainappointment.Entity) (domainappointment.Entity, error)
	getByIDFn       func(ctx context.Context, tenantID, appointmentID uuid.UUID) (domainappointment.Entity, error)
	updateFn        func(ctx context.Context, in domainappointment.Entity) (domainappointment.Entity, error)
	listByRangeFn   func(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]domainappointment.Entity, error)
}

type fakeAuditor struct {
	actions []string
}

type fakeMetrics struct {
	createErrors []error
	cancelErrors []error
}

func (f *fakeAuditor) RecordDomainEvent(
	_ context.Context,
	_ uuid.UUID,
	_ uuid.UUID,
	action string,
	_ string,
	_ *uuid.UUID,
	_ map[string]any,
) error {
	f.actions = append(f.actions, action)
	return nil
}

func (f *fakeMetrics) RecordAppointmentCreated(err error) {
	f.createErrors = append(f.createErrors, err)
}

func (f *fakeMetrics) RecordAppointmentCanceled(err error) {
	f.cancelErrors = append(f.cancelErrors, err)
}

func (f fakeRepo) Create(ctx context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
	if f.createFn == nil {
		return in, nil
	}
	return f.createFn(ctx, in)
}
func (f fakeRepo) ListByRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]domainappointment.Entity, error) {
	if f.listByRangeFn == nil {
		return nil, nil
	}
	return f.listByRangeFn(ctx, tenantID, from, to)
}
func (f fakeRepo) GetByID(ctx context.Context, tenantID, appointmentID uuid.UUID) (domainappointment.Entity, error) {
	return f.getByIDFn(ctx, tenantID, appointmentID)
}
func (f fakeRepo) Update(ctx context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
	if f.updateFn == nil {
		return in, nil
	}
	return f.updateFn(ctx, in)
}
func (f fakeRepo) ExistsOverlap(ctx context.Context, tenantID uuid.UUID, startsAt, endsAt time.Time, excludeID *uuid.UUID) (bool, error) {
	if f.existsOverlapFn == nil {
		return false, nil
	}
	return f.existsOverlapFn(ctx, tenantID, startsAt, endsAt, excludeID)
}
func (f fakeRepo) ClientExists(ctx context.Context, tenantID, clientID uuid.UUID) (bool, error) {
	if f.clientExistsFn == nil {
		return true, nil
	}
	return f.clientExistsFn(ctx, tenantID, clientID)
}

func TestCreateReturnsConflictOnOverlap(t *testing.T) {
	svc := NewService(fakeRepo{
		existsOverlapFn: func(context.Context, uuid.UUID, time.Time, time.Time, *uuid.UUID) (bool, error) { return true, nil },
	}, nil)

	_, err := svc.Create(context.Background(), CreateInput{
		TenantID: uuid.New(),
		ClientID: uuid.New(),
		StartsAt: time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 3, 4, 11, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestListByRangeValidatesInput(t *testing.T) {
	svc := NewService(fakeRepo{}, nil)
	at := time.Now()
	_, err := svc.ListByRange(context.Background(), ListInput{TenantID: uuid.New(), From: at, To: at})
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateReturnsForbiddenWhenClientOutsideTenant(t *testing.T) {
	svc := NewService(fakeRepo{
		clientExistsFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return false, nil },
	}, nil)

	_, err := svc.Create(context.Background(), CreateInput{
		TenantID: uuid.New(),
		ClientID: uuid.New(),
		StartsAt: time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 3, 4, 11, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestAppointmentActionsRecordDomainAudit(t *testing.T) {
	tenantID := uuid.New()
	actorID := uuid.New()
	clientID := uuid.New()
	appointmentID := uuid.New()
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	auditor := &fakeAuditor{}

	repo := fakeRepo{
		clientExistsFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return true, nil },
		createFn: func(_ context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
			in.ID = appointmentID
			return in, nil
		},
		getByIDFn: func(_ context.Context, _, _ uuid.UUID) (domainappointment.Entity, error) {
			return domainappointment.Entity{
				ID:       appointmentID,
				TenantID: tenantID,
				ClientID: clientID,
				StartsAt: base,
				EndsAt:   base.Add(time.Hour),
				Status:   domainappointment.StatusScheduled,
			}, nil
		},
		updateFn: func(_ context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
			return in, nil
		},
	}

	svc := NewService(repo, auditor)
	if _, err := svc.Create(context.Background(), CreateInput{
		TenantID:    tenantID,
		ClientID:    clientID,
		ActorUserID: actorID,
		StartsAt:    base,
		EndsAt:      base.Add(time.Hour),
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Update(context.Background(), UpdateInput{
		TenantID:      tenantID,
		AppointmentID: appointmentID,
		ActorUserID:   actorID,
		StartsAt:      base.Add(2 * time.Hour),
		EndsAt:        base.Add(3 * time.Hour),
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := svc.Cancel(context.Background(), tenantID, appointmentID, actorID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	want := []string{"appointment.create", "appointment.update", "appointment.cancel"}
	if len(auditor.actions) != len(want) {
		t.Fatalf("expected %d actions, got %d (%v)", len(want), len(auditor.actions), auditor.actions)
	}
	for i, action := range want {
		if auditor.actions[i] != action {
			t.Fatalf("expected action[%d]=%q, got %q", i, action, auditor.actions[i])
		}
	}
}

func TestUpdateReturnsValidationForCanceledAppointment(t *testing.T) {
	tenantID := uuid.New()
	appointmentID := uuid.New()
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)

	svc := NewService(fakeRepo{
		getByIDFn: func(_ context.Context, _, _ uuid.UUID) (domainappointment.Entity, error) {
			return domainappointment.Entity{
				ID:       appointmentID,
				TenantID: tenantID,
				ClientID: uuid.New(),
				StartsAt: base,
				EndsAt:   base.Add(time.Hour),
				Status:   domainappointment.StatusCanceled,
			}, nil
		},
	}, nil)

	_, err := svc.Update(context.Background(), UpdateInput{
		TenantID:      tenantID,
		AppointmentID: appointmentID,
		ActorUserID:   uuid.New(),
		StartsAt:      base.Add(2 * time.Hour),
		EndsAt:        base.Add(3 * time.Hour),
	})
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCancelReturnsValidationWhenAlreadyCanceled(t *testing.T) {
	tenantID := uuid.New()
	appointmentID := uuid.New()
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)

	svc := NewService(fakeRepo{
		getByIDFn: func(_ context.Context, _, _ uuid.UUID) (domainappointment.Entity, error) {
			return domainappointment.Entity{
				ID:        appointmentID,
				TenantID:  tenantID,
				ClientID:  uuid.New(),
				StartsAt:  base,
				EndsAt:    base.Add(time.Hour),
				Status:    domainappointment.StatusCanceled,
				UpdatedAt: base,
			}, nil
		},
	}, nil)

	_, err := svc.Cancel(context.Background(), tenantID, appointmentID, uuid.New())
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAppointmentMetricsRecordCreateAndCancelOutcomes(t *testing.T) {
	tenantID := uuid.New()
	clientID := uuid.New()
	appointmentID := uuid.New()
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	metrics := &fakeMetrics{}

	repo := fakeRepo{
		clientExistsFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return true, nil },
		createFn: func(_ context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
			in.ID = appointmentID
			return in, nil
		},
		getByIDFn: func(_ context.Context, _, _ uuid.UUID) (domainappointment.Entity, error) {
			return domainappointment.Entity{
				ID:       appointmentID,
				TenantID: tenantID,
				ClientID: clientID,
				StartsAt: base,
				EndsAt:   base.Add(time.Hour),
				Status:   domainappointment.StatusScheduled,
			}, nil
		},
		updateFn: func(_ context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
			return in, nil
		},
	}

	svc := NewService(repo, nil).WithMetrics(metrics)
	if _, err := svc.Create(context.Background(), CreateInput{
		TenantID: tenantID,
		ClientID: clientID,
		StartsAt: base,
		EndsAt:   base.Add(time.Hour),
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Cancel(context.Background(), tenantID, appointmentID, uuid.New()); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if len(metrics.createErrors) != 1 || metrics.createErrors[0] != nil {
		t.Fatalf("expected one successful create metric, got %#v", metrics.createErrors)
	}
	if len(metrics.cancelErrors) != 1 || metrics.cancelErrors[0] != nil {
		t.Fatalf("expected one successful cancel metric, got %#v", metrics.cancelErrors)
	}
}
