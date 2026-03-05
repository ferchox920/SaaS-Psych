package client

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

func TestNewEntityRequiresTenantAndFullName(t *testing.T) {
	_, err := NewEntity(uuid.Nil, "", "", "", time.Time{})
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestEntityUpdateTrimsFieldsAndUpdatesTimestamp(t *testing.T) {
	now := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	entity, err := NewEntity(uuid.New(), "Alice", "123", "old", now)
	if err != nil {
		t.Fatalf("new entity: %v", err)
	}

	next := now.Add(time.Hour)
	if err := entity.Update("  Alice Doe  ", "  +54 11  ", "  notes  ", next); err != nil {
		t.Fatalf("update entity: %v", err)
	}

	if entity.FullName != "Alice Doe" {
		t.Fatalf("unexpected fullname %q", entity.FullName)
	}
	if entity.Contact != "+54 11" {
		t.Fatalf("unexpected contact %q", entity.Contact)
	}
	if entity.NotesPublic != "notes" {
		t.Fatalf("unexpected notes %q", entity.NotesPublic)
	}
	if !entity.UpdatedAt.Equal(next) {
		t.Fatalf("expected updated_at %s, got %s", next, entity.UpdatedAt)
	}
}
