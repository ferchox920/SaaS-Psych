package consent

import (
	"context"
	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"time"
)

const (
	LocalAI        = "LOCAL_AI_PROCESSING"
	Audio          = "AUDIO_RECORDING"
	Transcription  = "LOCAL_TRANSCRIPTION"
	ExternalManual = "EXTERNAL_MANUAL_AI_PROCESSING"
)

type Grant struct {
	ID                uuid.UUID  `json:"id"`
	TenantID          uuid.UUID  `json:"tenant_id"`
	ClientID          uuid.UUID  `json:"client_id"`
	Scope             string     `json:"scope"`
	DefinitionVersion int        `json:"definition_version"`
	Status            string     `json:"status"`
	GrantedBy         uuid.UUID  `json:"granted_by_user_id"`
	GrantedAt         time.Time  `json:"granted_at"`
	EffectiveFrom     time.Time  `json:"effective_from"`
	RevokedBy         *uuid.UUID `json:"revoked_by_user_id,omitempty"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
}
type Repository interface {
	List(context.Context, uuid.UUID, uuid.UUID) ([]Grant, error)
	Grant(context.Context, Grant) (Grant, error)
	Revoke(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (Grant, error)
	Authorize(context.Context, uuid.UUID, uuid.UUID, string, string, uuid.UUID) (Grant, error)
}
type Access interface {
	CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error)
}
type Authorizer interface {
	Authorize(context.Context, uuid.UUID, uuid.UUID, string, string, uuid.UUID) (Grant, error)
}
type Service struct {
	repo   Repository
	access Access
}

func NewService(r Repository, a Access) *Service { return &Service{r, a} }
func ValidScope(s string) bool {
	return s == LocalAI || s == Audio || s == Transcription || s == ExternalManual
}
func (s *Service) require(ctx context.Context, t, c, a uuid.UUID, roles ...string) error {
	if t == uuid.Nil || c == uuid.Nil || a == uuid.Nil {
		return domainerrors.NewValidation("tenant, client and human actor are required")
	}
	allowed, err := s.access.CanAccessClient(ctx, t, a, c, roles...)
	if err != nil {
		return err
	}
	if !allowed {
		return domainerrors.ErrForbidden
	}
	return nil
}
func (s *Service) List(ctx context.Context, t, c, a uuid.UUID) ([]Grant, error) {
	if err := s.require(ctx, t, c, a, "treating", "supervisor"); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, t, c)
}
func (s *Service) Grant(ctx context.Context, t, c, a uuid.UUID, scope string, version int) (Grant, error) {
	if err := s.require(ctx, t, c, a, "treating"); err != nil {
		return Grant{}, err
	}
	if !ValidScope(scope) || version != 1 {
		return Grant{}, domainerrors.NewValidation("unknown consent definition or scope")
	}
	return s.repo.Grant(ctx, Grant{ID: uuid.New(), TenantID: t, ClientID: c, GrantedBy: a, Scope: scope, DefinitionVersion: version})
}
func (s *Service) Revoke(ctx context.Context, t, c, a, id uuid.UUID) (Grant, error) {
	if err := s.require(ctx, t, c, a, "treating"); err != nil {
		return Grant{}, err
	}
	return s.repo.Revoke(ctx, t, c, a, id)
}
