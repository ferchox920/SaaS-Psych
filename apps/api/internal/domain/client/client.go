package client

import (
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Entity struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	FullName    string
	Contact     string
	NotesPublic string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewEntity(tenantID uuid.UUID, fullName, contact, notesPublic string, now time.Time) (Entity, error) {
	fullName = strings.TrimSpace(fullName)
	if tenantID == uuid.Nil {
		return Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if fullName == "" {
		return Entity{}, domainerrors.NewValidation("fullname is required")
	}

	trimmedContact := strings.TrimSpace(contact)
	trimmedNotes := strings.TrimSpace(notesPublic)
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return Entity{
		ID:          uuid.New(),
		TenantID:    tenantID,
		FullName:    fullName,
		Contact:     trimmedContact,
		NotesPublic: trimmedNotes,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (e *Entity) Update(fullName, contact, notesPublic string, now time.Time) error {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return domainerrors.NewValidation("fullname is required")
	}

	if now.IsZero() {
		now = time.Now().UTC()
	}

	e.FullName = fullName
	e.Contact = strings.TrimSpace(contact)
	e.NotesPublic = strings.TrimSpace(notesPublic)
	e.UpdatedAt = now
	return nil
}
