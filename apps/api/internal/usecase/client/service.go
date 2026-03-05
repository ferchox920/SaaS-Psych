package client

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Repository interface {
	Create(ctx context.Context, in domainclient.Entity) (domainclient.Entity, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error)
	GetByID(ctx context.Context, tenantID, clientID uuid.UUID) (domainclient.Entity, error)
	Update(ctx context.Context, in domainclient.Entity) (domainclient.Entity, error)
	Delete(ctx context.Context, tenantID, clientID uuid.UUID) error
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

func (s *Service) Create(ctx context.Context, input CreateInput) (domainclient.Entity, error) {
	entity, err := domainclient.NewEntity(input.TenantID, input.FullName, input.Contact, input.NotesPublic, s.now())
	if err != nil {
		return domainclient.Entity{}, err
	}

	created, err := s.repo.Create(ctx, entity)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("create client: %w", err)
	}
	s.recordAudit(ctx, input.TenantID, input.ActorUserID, "client.create", "client", created.ID, map[string]any{})

	return created, nil
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error) {
	if tenantID == uuid.Nil {
		return nil, domainerrors.NewValidation("tenant_id is required")
	}

	items, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}

	return items, nil
}

func (s *Service) Get(ctx context.Context, tenantID, clientID uuid.UUID) (domainclient.Entity, error) {
	if tenantID == uuid.Nil {
		return domainclient.Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if clientID == uuid.Nil {
		return domainclient.Entity{}, domainerrors.NewValidation("client_id is required")
	}

	entity, err := s.repo.GetByID(ctx, tenantID, clientID)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("get client: %w", err)
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

	if err := existing.Update(input.FullName, input.Contact, input.NotesPublic, s.now()); err != nil {
		return domainclient.Entity{}, err
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return domainclient.Entity{}, fmt.Errorf("update client: %w", err)
	}
	s.recordAudit(ctx, input.TenantID, input.ActorUserID, "client.update", "client", updated.ID, map[string]any{})

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domainerrors.NewValidation("tenant_id is required")
	}
	if clientID == uuid.Nil {
		return domainerrors.NewValidation("client_id is required")
	}

	if err := s.repo.Delete(ctx, tenantID, clientID); err != nil {
		return fmt.Errorf("delete client: %w", err)
	}
	s.recordAudit(ctx, tenantID, actorUserID, "client.delete", "client", clientID, map[string]any{})

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
) {
	if s.auditor == nil {
		return
	}
	entityIDCopy := entityID
	_ = s.auditor.RecordDomainEvent(ctx, tenantID, actorUserID, action, entity, &entityIDCopy, metadata)
}
