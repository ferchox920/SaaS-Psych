package transcription

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
)

var (
	ErrDisabled    = errors.New("local transcription disabled")
	ErrUnavailable = errors.New("local transcription unavailable")
	ErrBusy        = errors.New("local transcription busy")
)

type Status struct {
	Enabled     bool   `json:"enabled"`
	Available   bool   `json:"available"`
	Busy        bool   `json:"busy"`
	Engine      string `json:"engine,omitempty"`
	Model       string `json:"model,omitempty"`
	Device      string `json:"device,omitempty"`
	ComputeType string `json:"compute_type,omitempty"`
}

type Result struct {
	Text                 string  `json:"text"`
	Language             string  `json:"language"`
	DurationSeconds      float64 `json:"duration_seconds"`
	TranscriptionSeconds float64 `json:"transcription_seconds"`
	RealTimeFactor       float64 `json:"real_time_factor"`
	Engine               string  `json:"engine"`
	Model                string  `json:"model"`
}

type Provider interface {
	Health(ctx context.Context) (Status, error)
	Transcribe(ctx context.Context, audio []byte, format string) (Result, error)
}

type AppointmentRepository interface {
	GetByID(ctx context.Context, tenantID, appointmentID uuid.UUID) (domainappointment.Entity, error)
}

type ClinicalAccess interface {
	CanAccessClient(ctx context.Context, tenantID, userID, clientID uuid.UUID, relationships ...string) (bool, error)
}

type Auditor interface {
	RecordDomainEvent(ctx context.Context, tenantID, actorUserID uuid.UUID, action, entity string, entityID *uuid.UUID, metadata map[string]any) error
}

type Metrics interface {
	RecordTranscription(result, reason, engine, model string, seconds, realTimeFactor float64)
}

type Service struct {
	consent      consent.Authorizer
	enabled      bool
	maxAudioSize int
	provider     Provider
	appointments AppointmentRepository
	access       ClinicalAccess
	auditor      Auditor
	metrics      Metrics
}

type TranscribeInput struct {
	TenantID        uuid.UUID
	ActorUserID     uuid.UUID
	AppointmentID   uuid.UUID
	ExplicitConsent bool
	Format          string
	Audio           []byte
}

func (s *Service) WithConsent(a consent.Authorizer) *Service { s.consent = a; return s }

func NewService(enabled bool, maxAudioSize int, provider Provider, appointments AppointmentRepository, access ClinicalAccess, auditor Auditor) *Service {
	return &Service{enabled: enabled, maxAudioSize: maxAudioSize, provider: provider, appointments: appointments, access: access, auditor: auditor}
}

func (s *Service) WithMetrics(metrics Metrics) *Service {
	s.metrics = metrics
	return s
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	if !s.enabled {
		return Status{Enabled: false}, nil
	}
	status, err := s.provider.Health(ctx)
	status.Enabled = true
	return status, err
}

func (s *Service) Transcribe(ctx context.Context, input TranscribeInput) (result Result, err error) {
	defer func() {
		if s.metrics == nil {
			return
		}
		if err != nil {
			s.metrics.RecordTranscription("error", classifyMetricError(err), "", "", 0, 0)
			return
		}
		s.metrics.RecordTranscription("success", "none", result.Engine, result.Model, result.TranscriptionSeconds, result.RealTimeFactor)
	}()
	if !s.enabled {
		return Result{}, ErrDisabled
	}
	if !input.ExplicitConsent {
		return Result{}, domainerrors.NewValidation("explicit audio consent is required")
	}
	if input.TenantID == uuid.Nil || input.ActorUserID == uuid.Nil || input.AppointmentID == uuid.Nil {
		return Result{}, domainerrors.NewValidation("tenant_id, actor_user_id and appointment_id are required")
	}
	format := strings.ToLower(strings.TrimSpace(input.Format))
	if format != "wav" && format != "webm" && format != "ogg" && format != "mp4" && format != "m4a" {
		return Result{}, domainerrors.NewValidation("unsupported audio format")
	}
	if len(input.Audio) == 0 || len(input.Audio) > s.maxAudioSize {
		return Result{}, domainerrors.NewValidation("audio size is outside the configured ephemeral limit")
	}
	appointment, err := s.appointments.GetByID(ctx, input.TenantID, input.AppointmentID)
	if err != nil {
		return Result{}, fmt.Errorf("get appointment for transcription: %w", err)
	}
	allowed, err := s.access.CanAccessClient(ctx, input.TenantID, input.ActorUserID, appointment.ClientID, "treating")
	if err != nil {
		return Result{}, fmt.Errorf("check transcription access: %w", err)
	}
	if !allowed {
		return Result{}, domainerrors.ErrForbidden
	}
	if s.consent != nil {
		for _, scope := range []string{consent.Audio, consent.Transcription} {
			if _, err = s.consent.Authorize(ctx, input.TenantID, appointment.ClientID, scope, "ephemeral_transcription", input.AppointmentID); err != nil {
				return Result{}, err
			}
		}
	}
	if err := s.audit(ctx, input, "clinical_transcription.start", map[string]any{"format": format, "audio_bytes": len(input.Audio), "retained": false}); err != nil {
		return Result{}, fmt.Errorf("audit transcription start: %w", err)
	}
	result, err = s.provider.Transcribe(ctx, input.Audio, format)
	if err != nil {
		_ = s.audit(ctx, input, "clinical_transcription.failed", map[string]any{"error_class": classifyError(err), "retained": false})
		return Result{}, err
	}
	if err := s.audit(ctx, input, "clinical_transcription.complete", map[string]any{
		"duration_seconds": result.DurationSeconds, "transcription_seconds": result.TranscriptionSeconds,
		"real_time_factor": result.RealTimeFactor, "engine": result.Engine, "model": result.Model, "retained": false,
	}); err != nil {
		return Result{}, fmt.Errorf("audit transcription completion: %w", err)
	}
	return result, nil
}

func classifyMetricError(err error) string {
	switch {
	case errors.Is(err, ErrDisabled):
		return "disabled"
	case errors.Is(err, ErrBusy):
		return "busy"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, domainerrors.ErrValidation):
		return "validation"
	case errors.Is(err, domainerrors.ErrForbidden):
		return "forbidden"
	default:
		return "unavailable"
	}
}

func (s *Service) audit(ctx context.Context, input TranscribeInput, action string, metadata map[string]any) error {
	if s.auditor == nil {
		return nil
	}
	appointmentID := input.AppointmentID
	return s.auditor.RecordDomainEvent(ctx, input.TenantID, input.ActorUserID, action, "appointment", &appointmentID, metadata)
}

func classifyError(err error) string {
	switch {
	case errors.Is(err, ErrBusy):
		return "busy"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "unavailable"
	}
}
