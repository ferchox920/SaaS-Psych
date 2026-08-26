package clinicalsession

import (
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

const (
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusVoided     = "voided"
)

type Entity struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	ClientID        uuid.UUID  `json:"client_id"`
	AppointmentID   *uuid.UUID `json:"appointment_id,omitempty"`
	TherapistUserID uuid.UUID  `json:"therapist_user_id"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func New(tenantID, clientID uuid.UUID, appointmentID *uuid.UUID, therapistID uuid.UUID, startedAt, now time.Time) (Entity, error) {
	if tenantID == uuid.Nil || clientID == uuid.Nil || therapistID == uuid.Nil {
		return Entity{}, domainerrors.NewValidation("tenant_id, client_id and therapist_user_id are required")
	}
	if appointmentID != nil && *appointmentID == uuid.Nil {
		return Entity{}, domainerrors.NewValidation("appointment_id must be a valid uuid")
	}
	if startedAt.IsZero() {
		return Entity{}, domainerrors.NewValidation("started_at is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Entity{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, AppointmentID: appointmentID,
		TherapistUserID: therapistID, Status: StatusInProgress, StartedAt: startedAt.UTC(), CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (e *Entity) Complete(at time.Time) error { return e.transition(StatusCompleted, at) }
func (e *Entity) Void(at time.Time) error     { return e.transition(StatusVoided, at) }

func (e *Entity) transition(target string, at time.Time) error {
	if strings.TrimSpace(e.Status) != StatusInProgress {
		return domainerrors.NewValidation("only an in-progress clinical session can transition")
	}
	if target != StatusCompleted && target != StatusVoided {
		return domainerrors.NewValidation("invalid clinical session transition")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if at.Before(e.StartedAt) {
		return domainerrors.NewValidation("ended_at cannot be before started_at")
	}
	e.Status, e.EndedAt, e.UpdatedAt = target, ptrTime(at.UTC()), at.UTC()
	return nil
}

func ptrTime(value time.Time) *time.Time { return &value }
