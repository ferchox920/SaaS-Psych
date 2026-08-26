package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Repository interface {
	Create(ctx context.Context, in domainclient.Entity, actorUserID uuid.UUID) (domainclient.Entity, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error)
	ListArchived(ctx context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error)
	GetByID(ctx context.Context, tenantID, clientID uuid.UUID) (domainclient.Entity, error)
	Update(ctx context.Context, in domainclient.Entity, actorUserID uuid.UUID) (domainclient.Entity, error)
	Archive(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID, reason string, archivedAt time.Time) error
	Restore(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID, restoredAt time.Time) error
}

type Service struct {
	repo    Repository
	auditor Auditor
	access  ClinicalAccess
	now     func() time.Time
}

type ClinicalAccess interface {
	CanAccessClient(ctx context.Context, tenantID, userID, clientID uuid.UUID, relationships ...string) (bool, error)
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

type CreateInput struct {
	TenantID    uuid.UUID
	ActorUserID uuid.UUID
	FullName    string
	Contact     string
	NotesPublic string
}

type UpdateInput struct {
	TenantID    uuid.UUID
	ActorUserID uuid.UUID
	ClientID    uuid.UUID
	FullName    string
	Contact     string
	NotesPublic string
}

func NewService(repo Repository, auditor Auditor) *Service {
	return &Service{repo: repo, auditor: auditor, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) WithClinicalAccess(access ClinicalAccess) *Service {
	s.access = access
	return s
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domainclient.Entity, error) {
	entity, err := domainclient.NewEntity(input.TenantID, input.FullName, input.Contact, input.NotesPublic, s.now())
	if err != nil {
		return domainclient.Entity{}, err
	}

	created, err := s.repo.Create(ctx, entity, input.ActorUserID)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("create client: %w", err)
	}
	if err := s.recordWriteAudit(ctx, input.TenantID, input.ActorUserID, "client.create", created.ID, map[string]any{}); err != nil {
		return domainclient.Entity{}, err
	}

	return created, nil
}

func (s *Service) List(ctx context.Context, tenantID, viewerUserID uuid.UUID) ([]domainclient.Entity, error) {
	if tenantID == uuid.Nil {
		return nil, domainerrors.NewValidation("tenant_id is required")
	}
	if viewerUserID == uuid.Nil {
		return nil, domainerrors.NewValidation("viewer_user_id is required")
	}

	items, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}

	if s.access == nil {
		if err := s.recordAudit(ctx, tenantID, viewerUserID, "client.list", "client", uuid.Nil, map[string]any{}); err != nil {
			return nil, fmt.Errorf("audit client list: %w", err)
		}
		return items, nil
	}
	visible := make([]domainclient.Entity, 0, len(items))
	for _, item := range items {
		allowed, err := s.access.CanAccessClient(ctx, tenantID, viewerUserID, item.ID, "treating", "supervisor")
		if err != nil {
			return nil, fmt.Errorf("check clinical client access: %w", err)
		}
		if allowed {
			visible = append(visible, item)
		}
	}
	if err := s.recordAudit(ctx, tenantID, viewerUserID, "client.list", "client", uuid.Nil, map[string]any{}); err != nil {
		return nil, fmt.Errorf("audit client list: %w", err)
	}
	return visible, nil
}

func (s *Service) ListArchived(ctx context.Context, tenantID, viewerUserID uuid.UUID) ([]domainclient.Entity, error) {
	if tenantID == uuid.Nil || viewerUserID == uuid.Nil {
		return nil, domainerrors.NewValidation("tenant_id and viewer_user_id are required")
	}
	items, err := s.repo.ListArchived(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list archived clients: %w", err)
	}
	if s.access == nil {
		if err := s.recordAudit(ctx, tenantID, viewerUserID, "client.archived.list", "client", uuid.Nil, map[string]any{}); err != nil {
			return nil, fmt.Errorf("audit archived client list: %w", err)
		}
		return items, nil
	}
	visible := make([]domainclient.Entity, 0, len(items))
	for _, item := range items {
		allowed, err := s.access.CanAccessClient(ctx, tenantID, viewerUserID, item.ID, "treating", "supervisor")
		if err != nil {
			return nil, fmt.Errorf("check archived client access: %w", err)
		}
		if allowed {
			visible = append(visible, item)
		}
	}
	if err := s.recordAudit(ctx, tenantID, viewerUserID, "client.archived.list", "client", uuid.Nil, map[string]any{}); err != nil {
		return nil, fmt.Errorf("audit archived client list: %w", err)
	}
	return visible, nil
}

func (s *Service) Get(ctx context.Context, tenantID, clientID, viewerUserID uuid.UUID) (domainclient.Entity, error) {
	if tenantID == uuid.Nil {
		return domainclient.Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if clientID == uuid.Nil {
		return domainclient.Entity{}, domainerrors.NewValidation("client_id is required")
	}
	if viewerUserID == uuid.Nil {
		return domainclient.Entity{}, domainerrors.NewValidation("viewer_user_id is required")
	}
	entity, err := s.repo.GetByID(ctx, tenantID, clientID)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("get client: %w", err)
	}
	if err := s.requireClinicalAccess(ctx, tenantID, viewerUserID, clientID, "treating", "supervisor"); err != nil {
		return domainclient.Entity{}, err
	}
	if err := s.recordAudit(ctx, tenantID, viewerUserID, "client.read", "client", clientID, map[string]any{}); err != nil {
		return domainclient.Entity{}, fmt.Errorf("audit client read: %w", err)
	}

	return entity, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (domainclient.Entity, error) {
	if input.TenantID == uuid.Nil {
		return domainclient.Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if input.ClientID == uuid.Nil {
		return domainclient.Entity{}, domainerrors.NewValidation("client_id is required")
	}

	existing, err := s.repo.GetByID(ctx, input.TenantID, input.ClientID)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("get client: %w", err)
	}
	if err := s.requireClinicalAccess(ctx, input.TenantID, input.ActorUserID, input.ClientID, "treating"); err != nil {
		return domainclient.Entity{}, err
	}

	if err := existing.Update(input.FullName, input.Contact, input.NotesPublic, s.now()); err != nil {
		return domainclient.Entity{}, err
	}

	updated, err := s.repo.Update(ctx, existing, input.ActorUserID)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("update client: %w", err)
	}
	if err := s.recordWriteAudit(ctx, input.TenantID, input.ActorUserID, "client.update", updated.ID, map[string]any{}); err != nil {
		return domainclient.Entity{}, err
	}

	return updated, nil
}

func (s *Service) Archive(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID, reason string) error {
	if tenantID == uuid.Nil {
		return domainerrors.NewValidation("tenant_id is required")
	}
	if clientID == uuid.Nil {
		return domainerrors.NewValidation("client_id is required")
	}
	if actorUserID == uuid.Nil {
		return domainerrors.NewValidation("actor_user_id is required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return domainerrors.NewValidation("archive reason is required")
	}
	if err := s.requireClinicalAccess(ctx, tenantID, actorUserID, clientID, "treating"); err != nil {
		return err
	}

	if err := s.repo.Archive(ctx, tenantID, clientID, actorUserID, reason, s.now()); err != nil {
		return fmt.Errorf("archive client: %w", err)
	}
	if err := s.recordWriteAudit(ctx, tenantID, actorUserID, "client.archive", clientID, map[string]any{"reason_recorded": true}); err != nil {
		return err
	}

	return nil
}

func (s *Service) Restore(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID) error {
	if tenantID == uuid.Nil || clientID == uuid.Nil || actorUserID == uuid.Nil {
		return domainerrors.NewValidation("tenant_id, client_id and actor_user_id are required")
	}
	if err := s.requireClinicalAccess(ctx, tenantID, actorUserID, clientID, "treating"); err != nil {
		return err
	}
	if err := s.repo.Restore(ctx, tenantID, clientID, actorUserID, s.now()); err != nil {
		return fmt.Errorf("restore client: %w", err)
	}
	if err := s.recordWriteAudit(ctx, tenantID, actorUserID, "client.restore", clientID, map[string]any{}); err != nil {
		return err
	}
	return nil
}

func (s *Service) requireClinicalAccess(ctx context.Context, tenantID, userID, clientID uuid.UUID, relationships ...string) error {
	if s.access == nil {
		return nil
	}
	allowed, err := s.access.CanAccessClient(ctx, tenantID, userID, clientID, relationships...)
	if err != nil {
		return fmt.Errorf("check clinical client access: %w", err)
	}
	if !allowed {
		return domainerrors.ErrForbidden
	}
	return nil
}

func (s *Service) recordWriteAudit(ctx context.Context, tenantID, actorUserID uuid.UUID, action string, entityID uuid.UUID, metadata map[string]any) error {
	if repo, ok := s.repo.(TransactionalWriteAuditor); ok && repo.WritesAreTransactionallyAudited() {
		return nil
	}
	if err := s.recordAudit(ctx, tenantID, actorUserID, action, "client", entityID, metadata); err != nil {
		return fmt.Errorf("audit client write: %w", err)
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
