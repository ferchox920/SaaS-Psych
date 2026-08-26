package clinicalaccess

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type fakeRepository struct {
	granted Assignment
	ended   bool
}

func (f *fakeRepository) Grant(_ context.Context, assignment Assignment) (Assignment, error) {
	f.granted = assignment
	return assignment, nil
}

func (f *fakeRepository) List(_ context.Context, _, _, _ uuid.UUID) ([]Assignment, error) {
	return []Assignment{f.granted}, nil
}

func (f *fakeRepository) End(_ context.Context, _, _, _, _ uuid.UUID, reason string, _ time.Time) error {
	f.ended = reason != ""
	return nil
}

func (f *fakeRepository) GrantException(_ context.Context, exception AccessException) (AccessException, error) {
	return exception, nil
}

func (f *fakeRepository) ListExceptions(_ context.Context, _, _, _ uuid.UUID) ([]AccessException, error) {
	return nil, nil
}

func (f *fakeRepository) RevokeException(_ context.Context, _, _, _, _ uuid.UUID, _ string, _ time.Time) error {
	return nil
}

func TestGrantValidatesRelationshipAndNormalizesIt(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)
	base := GrantInput{TenantID: uuid.New(), ClientID: uuid.New(), UserID: uuid.New(), GrantedByUserID: uuid.New()}
	invalid := base
	invalid.Relationship = "administrator"
	if _, err := svc.Grant(context.Background(), invalid); !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	valid := base
	valid.Relationship = " Treating "
	out, err := svc.Grant(context.Background(), valid)
	if err != nil || out.Relationship != RelationshipTreating {
		t.Fatalf("expected normalized treating assignment, out=%+v err=%v", out, err)
	}
}

func TestEndRequiresDocumentedReason(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)
	tenantID, clientID, assignmentID, actorID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := svc.End(context.Background(), tenantID, clientID, assignmentID, actorID, "  "); !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected reason validation, got %v", err)
	}
	if err := svc.End(context.Background(), tenantID, clientID, assignmentID, actorID, "care transferred"); err != nil || !repo.ended {
		t.Fatalf("expected assignment to end with reason, ended=%v err=%v", repo.ended, err)
	}
}

func TestAccessExceptionIsReadOnlyAndExpiresWithin24Hours(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	input := GrantExceptionInput{
		TenantID: uuid.New(), ClientID: uuid.New(), UserID: uuid.New(), GrantedByUserID: uuid.New(),
		Reason: "urgent continuity review", Purpose: "read-only chart review",
	}
	input.ExpiresAt = now.Add(25 * time.Hour)
	if _, err := svc.GrantException(context.Background(), input); !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected maximum duration validation, got %v", err)
	}
	input.ExpiresAt = now.Add(2 * time.Hour)
	out, err := svc.GrantException(context.Background(), input)
	if err != nil || !out.ExpiresAt.Equal(input.ExpiresAt) {
		t.Fatalf("expected bounded exception, out=%+v err=%v", out, err)
	}
}
