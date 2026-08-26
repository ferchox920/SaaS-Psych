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

type LifecycleRepository interface {
	Sign(ctx context.Context, tenantID, noteID uuid.UUID, expectedVersion int, signedAt time.Time) (domainsessionnote.Entity, error)
	Addendum(ctx context.Context, in domainsessionnote.Entity, actorUserID uuid.UUID, reason string) (domainsessionnote.Entity, error)
	ListVersions(ctx context.Context, tenantID, noteID uuid.UUID) ([]domainsessionnote.Version, error)
}

type Service struct {
	repo    Repository
	auditor Auditor
	access  ClinicalAccess
	now     func() time.Time
}

type ClinicalAccess interface {
	CanAccessAppointment(ctx context.Context, tenantID, userID, appointmentID uuid.UUID, relationships ...string) (bool, error)
}

type TransactionalWriteAuditor interface {
	WritesAreTransactionallyAudited() bool
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
	IsPrivate   *bool
}

type AddendumInput struct {
	TenantID    uuid.UUID
	NoteID      uuid.UUID
	ActorUserID uuid.UUID
	ActorRole   string
	Body        string
	Reason      string
}

func NewService(repo Repository, auditor Auditor) *Service {
	return &Service{repo: repo, auditor: auditor, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) WithClinicalAccess(access ClinicalAccess) *Service {
	s.access = access
	return s
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
	if err := s.requireAppointmentAccess(ctx, input.TenantID, input.AuthorUserID, input.AppointmentID, "treating"); err != nil {
		return domainsessionnote.Entity{}, err
	}

	out, err := s.repo.Create(ctx, note)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("create session note: %w", err)
	}
	if err := s.recordWriteAudit(ctx, input.TenantID, input.AuthorUserID, "session_note.create", out.ID, map[string]any{}); err != nil {
		return domainsessionnote.Entity{}, err
	}
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
	if s.access == nil && !domainsessionnote.CanView(note, viewer.UserID, viewer.Role) {
		return domainsessionnote.Entity{}, domainerrors.ErrForbidden
	}
	if err := s.requireAppointmentAccess(ctx, tenantID, viewer.UserID, note.AppointmentID, "treating", "supervisor"); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if err := s.recordAudit(ctx, tenantID, viewer.UserID, "session_note.read", "session_note", note.ID, map[string]any{}); err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("audit session note read: %w", err)
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
	if err := s.requireAppointmentAccess(ctx, input.TenantID, input.ActorUserID, existing.AppointmentID, "treating"); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if existing.Status != "draft" {
		return domainsessionnote.Entity{}, domainerrors.ErrConflict
	}

	body := strings.TrimSpace(input.Body)
	if body == "" {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("body is required")
	}

	existing.Body = body
	if input.IsPrivate != nil {
		existing.IsPrivate = *input.IsPrivate
	}
	existing.UpdatedAt = s.now()

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("update session note: %w", err)
	}
	if err := s.recordWriteAudit(ctx, input.TenantID, input.ActorUserID, "session_note.update", updated.ID, map[string]any{}); err != nil {
		return domainsessionnote.Entity{}, err
	}
	return updated, nil
}

func (s *Service) Sign(
	ctx context.Context,
	tenantID, noteID, actorUserID uuid.UUID,
	actorRole string,
) (domainsessionnote.Entity, error) {
	if tenantID == uuid.Nil || noteID == uuid.Nil || actorUserID == uuid.Nil {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("tenant_id, note_id and actor_user_id are required")
	}
	existing, err := s.repo.GetByID(ctx, tenantID, noteID)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("get session note: %w", err)
	}
	if !domainsessionnote.CanEdit(existing, actorUserID, actorRole) {
		return domainsessionnote.Entity{}, domainerrors.ErrForbidden
	}
	if err := s.requireAppointmentAccess(ctx, tenantID, actorUserID, existing.AppointmentID, "treating"); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if existing.Status != "draft" {
		return domainsessionnote.Entity{}, domainerrors.ErrConflict
	}
	lifecycleRepo, ok := s.repo.(LifecycleRepository)
	if !ok {
		return domainsessionnote.Entity{}, fmt.Errorf("session note lifecycle repository unavailable")
	}
	signed, err := lifecycleRepo.Sign(ctx, tenantID, noteID, existing.CurrentVersion, s.now())
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("sign session note: %w", err)
	}
	if err := s.recordWriteAudit(ctx, tenantID, actorUserID, "session_note.sign", signed.ID, map[string]any{}); err != nil {
		return domainsessionnote.Entity{}, err
	}
	return signed, nil
}

func (s *Service) Addendum(ctx context.Context, input AddendumInput) (domainsessionnote.Entity, error) {
	if input.TenantID == uuid.Nil || input.NoteID == uuid.Nil || input.ActorUserID == uuid.Nil {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("tenant_id, note_id and actor_user_id are required")
	}
	body := strings.TrimSpace(input.Body)
	reason := strings.TrimSpace(input.Reason)
	if body == "" {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("body is required")
	}
	if reason == "" {
		return domainsessionnote.Entity{}, domainerrors.NewValidation("reason is required")
	}
	existing, err := s.repo.GetByID(ctx, input.TenantID, input.NoteID)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("get session note: %w", err)
	}
	if !domainsessionnote.CanEdit(existing, input.ActorUserID, input.ActorRole) {
		return domainsessionnote.Entity{}, domainerrors.ErrForbidden
	}
	if err := s.requireAppointmentAccess(ctx, input.TenantID, input.ActorUserID, existing.AppointmentID, "treating"); err != nil {
		return domainsessionnote.Entity{}, err
	}
	if existing.Status != "signed" {
		return domainsessionnote.Entity{}, domainerrors.ErrConflict
	}
	lifecycleRepo, ok := s.repo.(LifecycleRepository)
	if !ok {
		return domainsessionnote.Entity{}, fmt.Errorf("session note lifecycle repository unavailable")
	}
	existing.Body = body
	existing.UpdatedAt = s.now()
	updated, err := lifecycleRepo.Addendum(ctx, existing, input.ActorUserID, reason)
	if err != nil {
		return domainsessionnote.Entity{}, fmt.Errorf("add session note addendum: %w", err)
	}
	if err := s.recordWriteAudit(ctx, input.TenantID, input.ActorUserID, "session_note.addendum", updated.ID, map[string]any{"reason_recorded": true}); err != nil {
		return domainsessionnote.Entity{}, err
	}
	return updated, nil
}

func (s *Service) ListVersions(
	ctx context.Context,
	tenantID, noteID uuid.UUID,
	viewer Viewer,
) ([]domainsessionnote.Version, error) {
	if tenantID == uuid.Nil || noteID == uuid.Nil || viewer.UserID == uuid.Nil {
		return nil, domainerrors.NewValidation("tenant_id, note_id and viewer user_id are required")
	}
	note, err := s.repo.GetByID(ctx, tenantID, noteID)
	if err != nil {
		return nil, fmt.Errorf("get session note: %w", err)
	}
	if s.access == nil && !domainsessionnote.CanView(note, viewer.UserID, viewer.Role) {
		return nil, domainerrors.ErrForbidden
	}
	if err := s.requireAppointmentAccess(ctx, tenantID, viewer.UserID, note.AppointmentID, "treating", "supervisor"); err != nil {
		return nil, err
	}
	lifecycleRepo, ok := s.repo.(LifecycleRepository)
	if !ok {
		return nil, fmt.Errorf("session note lifecycle repository unavailable")
	}
	versions, err := lifecycleRepo.ListVersions(ctx, tenantID, noteID)
	if err != nil {
		return nil, fmt.Errorf("list session note versions: %w", err)
	}
	if err := s.recordAudit(ctx, tenantID, viewer.UserID, "session_note.versions.read", "session_note", note.ID, map[string]any{}); err != nil {
		return nil, fmt.Errorf("audit session note version read: %w", err)
	}
	return versions, nil
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

	if err := s.requireAppointmentAccess(ctx, tenantID, viewer.UserID, appointmentID, "treating", "supervisor"); err != nil {
		return nil, err
	}

	visible := make([]domainsessionnote.Entity, 0, len(notes))
	for _, note := range notes {
		if s.access != nil || domainsessionnote.CanView(note, viewer.UserID, viewer.Role) {
			visible = append(visible, note)
		}
	}
	if err := s.recordAudit(ctx, tenantID, viewer.UserID, "session_note.list", "appointment", appointmentID, map[string]any{}); err != nil {
		return nil, fmt.Errorf("audit session note list: %w", err)
	}

	return visible, nil
}

func (s *Service) requireAppointmentAccess(
	ctx context.Context,
	tenantID, userID, appointmentID uuid.UUID,
	relationships ...string,
) error {
	if s.access == nil {
		return nil
	}
	allowed, err := s.access.CanAccessAppointment(ctx, tenantID, userID, appointmentID, relationships...)
	if err != nil {
		return fmt.Errorf("check clinical access: %w", err)
	}
	if !allowed {
		return domainerrors.ErrForbidden
	}
	return nil
}

func (s *Service) recordWriteAudit(
	ctx context.Context,
	tenantID, actorUserID uuid.UUID,
	action string,
	entityID uuid.UUID,
	metadata map[string]any,
) error {
	if repo, ok := s.repo.(TransactionalWriteAuditor); ok && repo.WritesAreTransactionallyAudited() {
		return nil
	}
	if err := s.recordAudit(ctx, tenantID, actorUserID, action, "session_note", entityID, metadata); err != nil {
		return fmt.Errorf("audit session note write: %w", err)
	}
	return nil
}

func (s *Service) recordAudit(
	ctx context.Context,
	tenantID uuid.UUID,
	actorUserID uuid.UUID,
	action string,
	entity string,
	entityID uuid.UUID,
	metadata map[string]any,
) error {
	if s.auditor == nil {
		return nil
	}
	entityIDCopy := entityID
	return s.auditor.RecordDomainEvent(ctx, tenantID, actorUserID, action, entity, &entityIDCopy, metadata)
}
