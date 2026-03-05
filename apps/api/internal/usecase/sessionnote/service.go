package sessionnote

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domainsessionnote "sessionflow/apps/api/internal/domain/sessionnote"
)

type Repository interface {
	Create(ctx context.Context, in domainsessionnote.Entity) (domainsessionnote.Entity, error)
	Update(ctx context.Context, in domainsessionnote.Entity) (domainsessionnote.Entity, error)
	GetByID(ctx context.Context, tenantID, noteID uuid.UUID) (domainsessionnote.Entity, error)
	ListByAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID) ([]domainsessionnote.Entity, error)
	AppointmentExists(ctx context.Context, tenantID, appointmentID uuid.UUID) (bool, error)
}

type Service struct {
	repo    Repository
	auditor Auditor
	now     func() time.Time
}

type Auditor interface {
	RecordDomainEvent(
		ctx context.Context,
		tenantID uuid.UUID,
		actorUserID uuid.UUID,
		action string,
		entity string,
		entityID *uuid.UUID,
		metadata map[string]any,
	) error
}

type Viewer struct {
	UserID uuid.UUID
	Role   string
}

type CreateInput struct {
	TenantID      uuid.UUID
	AppointmentID uuid.UUID
	AuthorUserID  uuid.UUID
	Body          string
	IsPrivate     bool
}

type UpdateInput struct {
	TenantID    uuid.UUID
	NoteID      uuid.UUID
	ActorUserID uuid.UUID
	ActorRole   string
	Body        string
	IsPrivate   bool
}

func NewService(repo Repository, auditor Auditor) *Service {
	return &Service{repo: repo, auditor: auditor, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domainsessionnote.Entity, error) {
	note, err := domainsessionnote.NewEntity(input.TenantID, input.AppointmentID, input.AuthorUserID, input.Body, input.IsPrivate, s.now())
	if err != nil {
		return domainsessionnote.Entity{}, err
	}

	exists, err := s.repo.AppointmentExists(ctx, input.TenantID, input.AppointmentID)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("check appointment exists: %w", err)
	}
	if !exists {
		return domainsessionnote.Entity{}, fmt.Errorf("appointment not found: %w", domainerrors.ErrNotFound)
	}

	out, err := s.repo.Create(ctx, note)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("create session note: %w", err)
	}
	s.recordAudit(ctx, input.TenantID, input.AuthorUserID, "session_note.create", "session_note", out.ID, map[string]any{})
	return out, nil
}

func (s *Service) Get(ctx context.Context, tenantID, noteID uuid.UUID, viewer Viewer) (domainsessionnote.Entity, error) {
	if tenantID == uuid.Nil {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if noteID == uuid.Nil {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("note_id is required")
	}
	if viewer.UserID == uuid.Nil {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("viewer user_id is required")
	}

	note, err := s.repo.GetByID(ctx, tenantID, noteID)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("get session note: %w", err)
	}
	if !domainsessionnote.CanView(note, viewer.UserID, viewer.Role) {
		return domainsessionnote.Entity{}, domainerrors.ErrForbidden
	}
	return note, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (domainsessionnote.Entity, error) {
	if input.TenantID == uuid.Nil {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if input.NoteID == uuid.Nil {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("note_id is required")
	}
	if input.ActorUserID == uuid.Nil {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("actor user_id is required")
	}

	existing, err := s.repo.GetByID(ctx, input.TenantID, input.NoteID)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("get session note: %w", err)
	}

	if !domainsessionnote.CanEdit(existing, input.ActorUserID, input.ActorRole) {
		return domainsessionnote.Entity{}, domainerrors.ErrForbidden
	}

	body := strings.TrimSpace(input.Body)
	if body == "" {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("body is required")
	}

	existing.Body = body
	existing.IsPrivate = input.IsPrivate
	existing.UpdatedAt = s.now()

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("update session note: %w", err)
	}
	s.recordAudit(ctx, input.TenantID, input.ActorUserID, "session_note.update", "session_note", updated.ID, map[string]any{})
	return updated, nil
}

func (s *Service) ListByAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID, viewer Viewer) ([]domainsessionnote.Entity, error) {
	if tenantID == uuid.Nil {
		return nil, domainerrors.NewValidation("tenant_id is required")
	}
	if appointmentID == uuid.Nil {
		return nil, domainerrors.NewValidation("appointment_id is required")
	}
	if viewer.UserID == uuid.Nil {
		return nil, domainerrors.NewValidation("viewer user_id is required")
	}

	notes, err := s.repo.ListByAppointment(ctx, tenantID, appointmentID)
	if err != nil {
		return nil, fmt.Errorf("list session notes: %w", err)
	}

	visible := make([]domainsessionnote.Entity, 0, len(notes))
	for _, note := range notes {
		if domainsessionnote.CanView(note, viewer.UserID, viewer.Role) {
			visible = append(visible, note)
		}
	}

	return visible, nil
}

func (s *Service) recordAudit(
	ctx context.Context,
	tenantID uuid.UUID,
	actorUserID uuid.UUID,
	action string,
	entity string,
	entityID uuid.UUID,
	metadata map[string]any,
) {
	if s.auditor == nil {
		return
	}
	entityIDCopy := entityID
	_ = s.auditor.RecordDomainEvent(ctx, tenantID, actorUserID, action, entity, &entityIDCopy, metadata)
}
