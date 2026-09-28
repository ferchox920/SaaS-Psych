package clinicalaccess

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

const (
	RelationshipTreating   = "treating"
	RelationshipSupervisor = "supervisor"
)

type Assignment struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ClientID        uuid.UUID
	UserID          uuid.UUID
	Relationship    string
	GrantedByUserID uuid.UUID
	StartsAt        time.Time
	EndsAt          *time.Time
	EndedByUserID   *uuid.UUID
	EndReason       string
	CreatedAt       time.Time
}

type TenantUser struct {
	ID    uuid.UUID
	Email string
}

type AccessException struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ClientID        uuid.UUID
	UserID          uuid.UUID
	GrantedByUserID uuid.UUID
	Reason          string
	Purpose         string
	StartsAt        time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
	RevokedByUserID *uuid.UUID
	RevokeReason    string
	CreatedAt       time.Time
}

type Repository interface {
	Grant(ctx context.Context, assignment Assignment) (Assignment, error)
	List(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID) ([]Assignment, error)
	End(ctx context.Context, tenantID, clientID, assignmentID, actorUserID uuid.UUID, reason string, endedAt time.Time) error
	GrantException(ctx context.Context, exception AccessException) (AccessException, error)
	ListExceptions(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID) ([]AccessException, error)
	RevokeException(ctx context.Context, tenantID, clientID, exceptionID, actorUserID uuid.UUID, reason string, revokedAt time.Time) error
}

type Service struct {
	repo Repository
	now  func() time.Time
}

type GrantInput struct {
	TenantID        uuid.UUID
	ClientID        uuid.UUID
	UserID          uuid.UUID
	Relationship    string
	GrantedByUserID uuid.UUID
}

type GrantExceptionInput struct {
	TenantID        uuid.UUID
	ClientID        uuid.UUID
	UserID          uuid.UUID
	GrantedByUserID uuid.UUID
	Reason          string
	Purpose         string
	ExpiresAt       time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Grant(ctx context.Context, input GrantInput) (Assignment, error) {
	relationship := strings.ToLower(strings.TrimSpace(input.Relationship))
	if input.TenantID == uuid.Nil || input.ClientID == uuid.Nil || input.UserID == uuid.Nil || input.GrantedByUserID == uuid.Nil {
		return Assignment{}, domainerrors.NewValidation("tenant_id, client_id, user_id and granted_by_user_id are required")
	}
	if relationship != RelationshipTreating && relationship != RelationshipSupervisor {
		return Assignment{}, domainerrors.NewValidation("relationship must be treating or supervisor")
	}
	now := s.now()
	return s.repo.Grant(ctx, Assignment{
		ID: uuid.New(), TenantID: input.TenantID, ClientID: input.ClientID,
		UserID: input.UserID, Relationship: relationship, GrantedByUserID: input.GrantedByUserID,
		StartsAt: now, CreatedAt: now,
	})
}

func (s *Service) List(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID) ([]Assignment, error) {
	if tenantID == uuid.Nil || clientID == uuid.Nil || actorUserID == uuid.Nil {
		return nil, domainerrors.NewValidation("tenant_id, client_id and actor_user_id are required")
	}
	return s.repo.List(ctx, tenantID, clientID, actorUserID)
}

func (s *Service) End(ctx context.Context, tenantID, clientID, assignmentID, actorUserID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if tenantID == uuid.Nil || clientID == uuid.Nil || assignmentID == uuid.Nil || actorUserID == uuid.Nil {
		return domainerrors.NewValidation("tenant_id, client_id, assignment_id and actor_user_id are required")
	}
	if reason == "" {
		return domainerrors.NewValidation("reason is required")
	}
	return s.repo.End(ctx, tenantID, clientID, assignmentID, actorUserID, reason, s.now())
}

func (s *Service) GrantException(ctx context.Context, input GrantExceptionInput) (AccessException, error) {
	reason, purpose := strings.TrimSpace(input.Reason), strings.TrimSpace(input.Purpose)
	if input.TenantID == uuid.Nil || input.ClientID == uuid.Nil || input.UserID == uuid.Nil || input.GrantedByUserID == uuid.Nil {
		return AccessException{}, domainerrors.NewValidation("tenant_id, client_id, user_id and granted_by_user_id are required")
	}
	if reason == "" || purpose == "" {
		return AccessException{}, domainerrors.NewValidation("reason and purpose are required")
	}
	now := s.now()
	expiresAt := input.ExpiresAt.UTC()
	if !expiresAt.After(now) || expiresAt.After(now.Add(24*time.Hour)) {
		return AccessException{}, domainerrors.NewValidation("expires_at must be in the future and within 24 hours")
	}
	return s.repo.GrantException(ctx, AccessException{
		ID: uuid.New(), TenantID: input.TenantID, ClientID: input.ClientID, UserID: input.UserID,
		GrantedByUserID: input.GrantedByUserID, Reason: reason, Purpose: purpose,
		StartsAt: now, ExpiresAt: expiresAt, CreatedAt: now,
	})
}

func (s *Service) ListExceptions(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID) ([]AccessException, error) {
	if tenantID == uuid.Nil || clientID == uuid.Nil || actorUserID == uuid.Nil {
		return nil, domainerrors.NewValidation("tenant_id, client_id and actor_user_id are required")
	}
	return s.repo.ListExceptions(ctx, tenantID, clientID, actorUserID)
}

func (s *Service) RevokeException(ctx context.Context, tenantID, clientID, exceptionID, actorUserID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if tenantID == uuid.Nil || clientID == uuid.Nil || exceptionID == uuid.Nil || actorUserID == uuid.Nil {
		return domainerrors.NewValidation("tenant_id, client_id, exception_id and actor_user_id are required")
	}
	if reason == "" {
		return domainerrors.NewValidation("reason is required")
	}
	return s.repo.RevokeException(ctx, tenantID, clientID, exceptionID, actorUserID, reason, s.now())
}
