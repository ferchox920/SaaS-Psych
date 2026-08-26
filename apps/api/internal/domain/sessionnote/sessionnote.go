package sessionnote

import (
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Entity struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	AppointmentID  uuid.UUID
	AuthorUserID   uuid.UUID
	Body           string
	IsPrivate      bool
	Status         string
	CurrentVersion int
	SignedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Version struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	NoteID       uuid.UUID
	Version      int
	Body         string
	IsPrivate    bool
	ChangeKind   string
	ChangeReason string
	ActorUserID  uuid.UUID
	CreatedAt    time.Time
}

func NewEntity(tenantID, appointmentID, authorUserID uuid.UUID, body string, isPrivate bool, now time.Time) (Entity, error) {
	body = strings.TrimSpace(body)
	if tenantID == uuid.Nil {
		return Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if appointmentID == uuid.Nil {
		return Entity{}, domainerrors.NewValidation("appointment_id is required")
	}
	if authorUserID == uuid.Nil {
		return Entity{}, domainerrors.NewValidation("author_user_id is required")
	}
	if body == "" {
		return Entity{}, domainerrors.NewValidation("body is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return Entity{
		ID:             uuid.New(),
		TenantID:       tenantID,
		AppointmentID:  appointmentID,
		AuthorUserID:   authorUserID,
		Body:           body,
		IsPrivate:      isPrivate,
		Status:         "draft",
		CurrentVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func CanView(note Entity, requesterUserID uuid.UUID, requesterRole string) bool {
	if !note.IsPrivate {
		return true
	}
	return note.AuthorUserID == requesterUserID
}

func CanEdit(note Entity, requesterUserID uuid.UUID, requesterRole string) bool {
	return note.AuthorUserID == requesterUserID
}
