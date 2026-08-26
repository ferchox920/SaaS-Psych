package clinicalmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Anchor struct {
	ID                 uuid.UUID  `json:"id"`
	SourceID           string     `json:"source_id"`
	Kind               string     `json:"kind"`
	Summary            string     `json:"summary"`
	TrafficLight       *string    `json:"traffic_light"`
	SourceSuggestionID *uuid.UUID `json:"source_suggestion_id"`
}

type ApprovedContext struct {
	SnapshotID      uuid.UUID `json:"snapshot_id"`
	Version         int       `json:"version"`
	ApprovedSummary string    `json:"approved_summary"`
	Anchors         []Anchor  `json:"anchors"`
}

type Snapshot struct {
	ID               uuid.UUID  `json:"id"`
	TenantID         uuid.UUID  `json:"tenant_id"`
	ClientID         uuid.UUID  `json:"client_id"`
	Version          int        `json:"version"`
	ApprovedSummary  string     `json:"approved_summary"`
	Status           string     `json:"status"`
	CreatedByUserID  uuid.UUID  `json:"created_by_user_id"`
	ApprovedByUserID *uuid.UUID `json:"approved_by_user_id"`
	ApprovedAt       *time.Time `json:"approved_at"`
	CreatedAt        time.Time  `json:"created_at"`
	Anchors          []Anchor   `json:"anchors"`
}

type Suggestion struct {
	ID              uuid.UUID       `json:"id"`
	TenantID        uuid.UUID       `json:"tenant_id"`
	ClientID        uuid.UUID       `json:"client_id"`
	AppointmentID   uuid.UUID       `json:"appointment_id"`
	CreatedByUserID uuid.UUID       `json:"created_by_user_id"`
	Mode            string          `json:"mode"`
	PromptVersion   string          `json:"prompt_version"`
	Result          json.RawMessage `json:"result"`
	Disposition     string          `json:"disposition"`
	CorrectionText  string          `json:"correction_text"`
	DecisionReason  string          `json:"decision_reason"`
	DecidedByUserID *uuid.UUID      `json:"decided_by_user_id"`
	DecidedAt       *time.Time      `json:"decided_at"`
	CreatedAt       time.Time       `json:"created_at"`
	AIRunID         *uuid.UUID      `json:"ai_run_id,omitempty"`
}

type CreateSuggestionInput struct {
	TenantID      uuid.UUID
	ClientID      uuid.UUID
	AppointmentID uuid.UUID
	ActorUserID   uuid.UUID
	Mode          string
	PromptVersion string
	Result        json.RawMessage
	AIRunID       *uuid.UUID
}

type Repository interface {
	CreateSuggestion(ctx context.Context, input CreateSuggestionInput) (Suggestion, error)
	ClientIDForAppointment(ctx context.Context, tenantID, appointmentID uuid.UUID) (uuid.UUID, error)
	SuggestionProvenance(ctx context.Context, tenantID, suggestionID uuid.UUID) (uuid.UUID, string, error)
	ListSuggestions(ctx context.Context, tenantID, appointmentID uuid.UUID) ([]Suggestion, error)
	DecideSuggestion(ctx context.Context, tenantID, suggestionID, actorUserID uuid.UUID, disposition, correction, reason string) (Suggestion, error)
	CreateSnapshot(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID, summary string, anchors []Anchor) (Snapshot, error)
	ListSnapshots(ctx context.Context, tenantID, clientID uuid.UUID) ([]Snapshot, error)
	GetApprovedContext(ctx context.Context, tenantID, clientID uuid.UUID) (ApprovedContext, error)
	ApproveSnapshot(ctx context.Context, tenantID, clientID, snapshotID, actorUserID uuid.UUID) (Snapshot, error)
}

type ClinicalAccess interface {
	CanAccessClient(ctx context.Context, tenantID, userID, clientID uuid.UUID, relationships ...string) (bool, error)
}

type Auditor interface {
	RecordDomainEvent(ctx context.Context, tenantID, actorUserID uuid.UUID, action, entity string, entityID *uuid.UUID, metadata map[string]any) error
}

type Metrics interface {
	RecordSuggestionDecision(disposition string)
}

type Service struct {
	repo    Repository
	access  ClinicalAccess
	auditor Auditor
	metrics Metrics
}

func (s *Service) WithMetrics(metrics Metrics) *Service {
	s.metrics = metrics
	return s
}

func NewService(repo Repository, access ClinicalAccess, auditors ...Auditor) *Service {
	service := &Service{repo: repo, access: access}
	if len(auditors) > 0 {
		service.auditor = auditors[0]
	}
	return service
}

func (s *Service) ListSuggestions(ctx context.Context, tenantID, appointmentID, actorUserID uuid.UUID) ([]Suggestion, error) {
	clientID, err := s.repo.ClientIDForAppointment(ctx, tenantID, appointmentID)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, tenantID, actorUserID, clientID, "treating", "supervisor"); err != nil {
		return nil, err
	}
	items, err := s.repo.ListSuggestions(ctx, tenantID, appointmentID)
	if err != nil {
		return nil, err
	}
	if err := s.auditRead(ctx, tenantID, actorUserID, "clinical_ai.suggestion.list", "appointment", appointmentID); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) DecideSuggestion(ctx context.Context, tenantID, suggestionID, actorUserID uuid.UUID, disposition, correction, reason string) (Suggestion, error) {
	clientID, _, err := s.repo.SuggestionProvenance(ctx, tenantID, suggestionID)
	if err != nil {
		return Suggestion{}, err
	}
	if err := s.require(ctx, tenantID, actorUserID, clientID, "treating"); err != nil {
		return Suggestion{}, err
	}
	disposition = strings.TrimSpace(disposition)
	correction = strings.TrimSpace(correction)
	reason = strings.TrimSpace(reason)
	if disposition != "accepted" && disposition != "corrected" && disposition != "discarded" && disposition != "postponed" {
		return Suggestion{}, domainerrors.NewValidation("invalid suggestion disposition")
	}
	if disposition == "corrected" && correction == "" {
		return Suggestion{}, domainerrors.NewValidation("correction_text is required when correcting")
	}
	if disposition != "corrected" {
		correction = ""
	}
	item, err := s.repo.DecideSuggestion(ctx, tenantID, suggestionID, actorUserID, disposition, correction, reason)
	if err == nil && s.metrics != nil {
		s.metrics.RecordSuggestionDecision(disposition)
	}
	return item, err
}

func (s *Service) CreateSnapshot(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID, summary string, anchors []Anchor) (Snapshot, error) {
	if err := s.require(ctx, tenantID, actorUserID, clientID, "treating"); err != nil {
		return Snapshot{}, err
	}
	summary = strings.TrimSpace(summary)
	if len([]rune(summary)) > 4000 {
		return Snapshot{}, domainerrors.NewValidation("approved_summary cannot exceed 4000 characters")
	}
	if len(anchors) < 2 || len(anchors) > 4 {
		return Snapshot{}, domainerrors.NewValidation("a formulation requires between 2 and 4 concrete anchors")
	}
	seen := make(map[string]bool)
	for index := range anchors {
		anchors[index].SourceID = strings.TrimSpace(anchors[index].SourceID)
		anchors[index].Summary = strings.TrimSpace(anchors[index].Summary)
		if anchors[index].SourceID == "" || anchors[index].Summary == "" || seen[anchors[index].SourceID] {
			return Snapshot{}, domainerrors.NewValidation("anchors require unique source_id and non-empty summary")
		}
		seen[anchors[index].SourceID] = true
		if anchors[index].Kind != "fact" && anchors[index].Kind != "hypothesis" {
			return Snapshot{}, domainerrors.NewValidation("anchor kind must be fact or hypothesis")
		}
		if anchors[index].Kind == "hypothesis" {
			if anchors[index].TrafficLight == nil || (*anchors[index].TrafficLight != "green" && *anchors[index].TrafficLight != "yellow" && *anchors[index].TrafficLight != "red") {
				return Snapshot{}, domainerrors.NewValidation("hypothesis anchors require a traffic light")
			}
		} else {
			anchors[index].TrafficLight = nil
		}
		if anchors[index].SourceSuggestionID != nil {
			suggestionClientID, disposition, err := s.repo.SuggestionProvenance(ctx, tenantID, *anchors[index].SourceSuggestionID)
			if err != nil {
				return Snapshot{}, fmt.Errorf("get source suggestion: %w", err)
			}
			if suggestionClientID != clientID || (disposition != "accepted" && disposition != "corrected") {
				return Snapshot{}, domainerrors.NewValidation("only accepted or corrected suggestions for this client can be cited")
			}
		}
	}
	return s.repo.CreateSnapshot(ctx, tenantID, clientID, actorUserID, summary, anchors)
}

func (s *Service) ListSnapshots(ctx context.Context, tenantID, clientID, actorUserID uuid.UUID) ([]Snapshot, error) {
	if err := s.require(ctx, tenantID, actorUserID, clientID, "treating", "supervisor"); err != nil {
		return nil, err
	}
	items, err := s.repo.ListSnapshots(ctx, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	if err := s.auditRead(ctx, tenantID, actorUserID, "clinical_formulation.list", "client", clientID); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) auditRead(ctx context.Context, tenantID, actorUserID uuid.UUID, action, entity string, entityID uuid.UUID) error {
	if s.auditor == nil {
		return nil
	}
	return s.auditor.RecordDomainEvent(ctx, tenantID, actorUserID, action, entity, &entityID, map[string]any{})
}

func (s *Service) ApproveSnapshot(ctx context.Context, tenantID, clientID, snapshotID, actorUserID uuid.UUID) (Snapshot, error) {
	if err := s.require(ctx, tenantID, actorUserID, clientID, "treating"); err != nil {
		return Snapshot{}, err
	}
	return s.repo.ApproveSnapshot(ctx, tenantID, clientID, snapshotID, actorUserID)
}

func (s *Service) require(ctx context.Context, tenantID, actorUserID, clientID uuid.UUID, relationships ...string) error {
	if tenantID == uuid.Nil || actorUserID == uuid.Nil || clientID == uuid.Nil {
		return domainerrors.NewValidation("tenant_id, actor_user_id and client_id are required")
	}
	allowed, err := s.access.CanAccessClient(ctx, tenantID, actorUserID, clientID, relationships...)
	if err != nil {
		return fmt.Errorf("check clinical memory access: %w", err)
	}
	if !allowed {
		return domainerrors.ErrForbidden
	}
	return nil
}
