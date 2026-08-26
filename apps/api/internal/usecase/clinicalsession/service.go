package clinicalsession

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domainclinicalsession "sessionflow/apps/api/internal/domain/clinicalsession"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Repository interface {
	ClientExists(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	AppointmentClient(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error)
	Create(context.Context, domainclinicalsession.Entity, uuid.UUID) (domainclinicalsession.Entity, error)
	GetByID(context.Context, uuid.UUID, uuid.UUID) (domainclinicalsession.Entity, error)
	ListByClient(context.Context, uuid.UUID, uuid.UUID) ([]domainclinicalsession.Entity, error)
	Transition(context.Context, domainclinicalsession.Entity, uuid.UUID) (domainclinicalsession.Entity, error)
}
type ClinicalAccess interface {
	CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error)
}
type Metrics interface {
	RecordClinicalSessionTransition(string, error)
}

type Service struct {
	repo    Repository
	access  ClinicalAccess
	metrics Metrics
	now     func() time.Time
}
type CreateInput struct {
	TenantID, ClientID, TherapistUserID uuid.UUID
	AppointmentID                       *uuid.UUID
	StartedAt                           time.Time
}

func NewService(repo Repository, access ClinicalAccess) *Service {
	return &Service{repo: repo, access: access, now: func() time.Time { return time.Now().UTC() }}
}
func (s *Service) WithMetrics(metrics Metrics) *Service { s.metrics = metrics; return s }

func (s *Service) Create(ctx context.Context, input CreateInput) (domainclinicalsession.Entity, error) {
	entity, err := domainclinicalsession.New(input.TenantID, input.ClientID, input.AppointmentID, input.TherapistUserID, input.StartedAt, s.now())
	if err != nil {
		s.metric("created", err)
		return domainclinicalsession.Entity{}, err
	}
	if err = s.require(ctx, input.TenantID, input.TherapistUserID, input.ClientID, "treating"); err != nil {
		s.metric("created", err)
		return domainclinicalsession.Entity{}, err
	}
	exists, err := s.repo.ClientExists(ctx, input.TenantID, input.ClientID)
	if err != nil {
		return domainclinicalsession.Entity{}, fmt.Errorf("check clinical session client: %w", err)
	}
	if !exists {
		return domainclinicalsession.Entity{}, domainerrors.ErrNotFound
	}
	if input.AppointmentID != nil {
		appointmentClient, appointmentErr := s.repo.AppointmentClient(ctx, input.TenantID, *input.AppointmentID)
		if appointmentErr != nil {
			return domainclinicalsession.Entity{}, appointmentErr
		}
		if appointmentClient != input.ClientID {
			return domainclinicalsession.Entity{}, domainerrors.ErrForbidden
		}
	}
	out, err := s.repo.Create(ctx, entity, input.TherapistUserID)
	s.metric("created", err)
	return out, err
}

func (s *Service) Get(ctx context.Context, tenantID, sessionID, actorID uuid.UUID) (domainclinicalsession.Entity, error) {
	item, err := s.repo.GetByID(ctx, tenantID, sessionID)
	if err != nil {
		return domainclinicalsession.Entity{}, err
	}
	if err := s.require(ctx, tenantID, actorID, item.ClientID, "treating", "supervisor"); err != nil {
		return domainclinicalsession.Entity{}, err
	}
	return item, nil
}

func (s *Service) ListByClient(ctx context.Context, tenantID, clientID, actorID uuid.UUID) ([]domainclinicalsession.Entity, error) {
	if err := s.require(ctx, tenantID, actorID, clientID, "treating", "supervisor"); err != nil {
		return nil, err
	}
	return s.repo.ListByClient(ctx, tenantID, clientID)
}

func (s *Service) Complete(ctx context.Context, tenantID, sessionID, actorID uuid.UUID) (domainclinicalsession.Entity, error) {
	return s.transition(ctx, tenantID, sessionID, actorID, domainclinicalsession.StatusCompleted)
}
func (s *Service) Void(ctx context.Context, tenantID, sessionID, actorID uuid.UUID) (domainclinicalsession.Entity, error) {
	return s.transition(ctx, tenantID, sessionID, actorID, domainclinicalsession.StatusVoided)
}
func (s *Service) transition(ctx context.Context, tenantID, sessionID, actorID uuid.UUID, target string) (domainclinicalsession.Entity, error) {
	item, err := s.repo.GetByID(ctx, tenantID, sessionID)
	if err != nil {
		s.metric(target, err)
		return domainclinicalsession.Entity{}, err
	}
	if err = s.require(ctx, tenantID, actorID, item.ClientID, "treating"); err != nil {
		s.metric(target, err)
		return domainclinicalsession.Entity{}, err
	}
	if target == domainclinicalsession.StatusCompleted {
		err = item.Complete(s.now())
	} else {
		err = item.Void(s.now())
	}
	if err != nil {
		s.metric(target, err)
		return domainclinicalsession.Entity{}, err
	}
	out, err := s.repo.Transition(ctx, item, actorID)
	s.metric(target, err)
	return out, err
}
func (s *Service) require(ctx context.Context, tenantID, actorID, clientID uuid.UUID, relationships ...string) error {
	if tenantID == uuid.Nil || actorID == uuid.Nil || clientID == uuid.Nil {
		return domainerrors.NewValidation("tenant_id, actor_user_id and client_id are required")
	}
	allowed, err := s.access.CanAccessClient(ctx, tenantID, actorID, clientID, relationships...)
	if err != nil {
		return fmt.Errorf("check clinical session access: %w", err)
	}
	if !allowed {
		return domainerrors.ErrForbidden
	}
	return nil
}
func (s *Service) metric(action string, err error) {
	if s.metrics != nil {
		s.metrics.RecordClinicalSessionTransition(action, err)
	}
}
