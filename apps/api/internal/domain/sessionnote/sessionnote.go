package sessionnote

import (
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Entity struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	AppointmentID uuid.UUID
	AuthorUserID  uuid.UUID
	Body          string
	IsPrivate     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
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
		ID:            uuid.New(),
		TenantID:      tenantID,
		AppointmentID: appointmentID,
		AuthorUserID:  authorUserID,
		Body:          body,
		IsPrivate:     isPrivate,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func CanView(note Entity, requesterUserID uuid.UUID, requesterRole string) bool {
	if !note.IsPrivate {
		return true
	}
	if note.AuthorUserID == requesterUserID {
		return true
	}
	return requesterRole == "owner" || requesterRole == "admin"
}

func CanEdit(note Entity, requesterUserID uuid.UUID, requesterRole string) bool {
	if note.AuthorUserID == requesterUserID {
		return true
	}
	return requesterRole == "owner" || requesterRole == "admin"
}
