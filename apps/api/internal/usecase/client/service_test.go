package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type fakeRepository struct {
	createFn func(ctx context.Context, in domainclient.Entity) (domainclient.Entity, error)
	getFn    func(ctx context.Context, tenantID, clientID uuid.UUID) (domainclient.Entity, error)
	updateFn func(ctx context.Context, in domainclient.Entity) (domainclient.Entity, error)
	listFn   func(ctx context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error)
	deleteFn func(ctx context.Context, tenantID, clientID uuid.UUID) error
}

type fakeAuditor struct {
	actions []string
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

func (f fakeRepository) Create(ctx context.Context, in domainclient.Entity) (domainclient.Entity, error) {
	return f.createFn(ctx, in)
}

func (f fakeRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error) {
	if f.listFn == nil {
		return nil, nil
	}
	return f.listFn(ctx, tenantID)
}

func (f fakeRepository) GetByID(ctx context.Context, tenantID, clientID uuid.UUID) (domainclient.Entity, error) {
	return f.getFn(ctx, tenantID, clientID)
}

func (f fakeRepository) Update(ctx context.Context, in domainclient.Entity) (domainclient.Entity, error) {
	return f.updateFn(ctx, in)
}

func (f fakeRepository) Delete(ctx context.Context, tenantID, clientID uuid.UUID) error {
	if f.deleteFn == nil {
		return nil
	}
	return f.deleteFn(ctx, tenantID, clientID)
}

func TestCreateRequiresValidInput(t *testing.T) {
	svc := NewService(fakeRepository{createFn: func(_ context.Context, in domainclient.Entity) (domainclient.Entity, error) {
		return in, nil
	}}, nil)

	_, err := svc.Create(context.Background(), CreateInput{TenantID: uuid.Nil, FullName: ""})
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestUpdateReturnsValidationForNilClientID(t *testing.T) {
	svc := NewService(fakeRepository{}, nil)
	_, err := svc.Update(context.Background(), UpdateInput{TenantID: uuid.New(), ClientID: uuid.Nil})
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestUpdateMutatesEntityAndPersists(t *testing.T) {
	tenantID := uuid.New()
	clientID := uuid.New()
	createdAt := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	existing := domainclient.Entity{
		ID:          clientID,
		TenantID:    tenantID,
		FullName:    "Old",
		Contact:     "old",
		NotesPublic: "old",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}

	svc := NewService(fakeRepository{
		getFn: func(_ context.Context, inTenantID, inClientID uuid.UUID) (domainclient.Entity, error) {
			if inTenantID != tenantID || inClientID != clientID {
				t.Fatalf("unexpected IDs")
			}
			return existing, nil
		},
		updateFn: func(_ context.Context, in domainclient.Entity) (domainclient.Entity, error) {
			return in, nil
		},
	}, nil)
	svc.now = func() time.Time { return updatedAt }

	out, err := svc.Update(context.Background(), UpdateInput{
		TenantID:    tenantID,
		ClientID:    clientID,
		FullName:    " New Name ",
		Contact:     " 123 ",
		NotesPublic: " x ",
	})
	if err != nil {
		t.Fatalf("update client: %v", err)
	}
	if out.FullName != "New Name" || out.Contact != "123" || out.NotesPublic != "x" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if !out.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected updated_at %s, got %s", updatedAt, out.UpdatedAt)
	}
}

func TestClientActionsRecordDomainAudit(t *testing.T) {
	tenantID := uuid.New()
	actorID := uuid.New()
	clientID := uuid.New()
	auditor := &fakeAuditor{}

	repo := fakeRepository{
		createFn: func(_ context.Context, in domainclient.Entity) (domainclient.Entity, error) {
			in.ID = clientID
			return in, nil
		},
		getFn: func(_ context.Context, _, _ uuid.UUID) (domainclient.Entity, error) {
			return domainclient.Entity{ID: clientID, TenantID: tenantID, FullName: "Old"}, nil
		},
		updateFn: func(_ context.Context, in domainclient.Entity) (domainclient.Entity, error) {
			return in, nil
		},
		deleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			return nil
		},
	}

	svc := NewService(repo, auditor)
	if _, err := svc.Create(context.Background(), CreateInput{TenantID: tenantID, ActorUserID: actorID, FullName: "Client 1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Update(context.Background(), UpdateInput{TenantID: tenantID, ActorUserID: actorID, ClientID: clientID, FullName: "Client 1 updated"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := svc.Delete(context.Background(), tenantID, clientID, actorID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if len(auditor.actions) != 3 {
		t.Fatalf("expected 3 audit actions, got %d (%v)", len(auditor.actions), auditor.actions)
	}
	want := []string{"client.create", "client.update", "client.delete"}
	for i, action := range want {
		if auditor.actions[i] != action {
			t.Fatalf("expected action[%d]=%q, got %q", i, action, auditor.actions[i])
		}
	}
}
