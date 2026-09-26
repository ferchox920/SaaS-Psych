package clinicalanalysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sessionflow/apps/api/internal/usecase/consent"
	"strings"
	"time"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
	clinicalmemory "sessionflow/apps/api/internal/usecase/clinicalmemory"
)

var (
	ErrProviderUnavailable = errors.New("local inference provider unavailable")
	ErrProviderBusy        = errors.New("local inference provider busy")
	ErrInvalidModelOutput  = errors.New("invalid local model output")
)

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
	RecordClinicalAnalysis(mode string, firstToken, total time.Duration, evalCount int, evalRate float64, repaired bool, contextChars int, err error)
}

type Service struct {
	consent       consent.Authorizer
	provider      ClinicalInferenceProvider
	appointments  AppointmentRepository
	access        ClinicalAccess
	auditor       Auditor
	memory        clinicalMemory
	metrics       Metrics
	riskProtocol  string
	runs          *clinicalairun.Service
	runProvider   string
	runModel      string
	runParameters map[string]any
}

type clinicalMemory interface {
	GetApprovedContext(ctx context.Context, tenantID, clientID uuid.UUID) (clinicalmemory.ApprovedContext, error)
	CreateSuggestion(ctx context.Context, input clinicalmemory.CreateSuggestionInput) (clinicalmemory.Suggestion, error)
}

type AnalyzeLiveInput struct {
	TenantID      uuid.UUID
	ActorUserID   uuid.UUID
	AppointmentID uuid.UUID
	Request       LiveRequest
}

type ReviewSessionInput struct {
	TenantID      uuid.UUID
	ActorUserID   uuid.UUID
	AppointmentID uuid.UUID
	SessionText   string
}

type AnalyzeLiveOutput struct {
	Result        Result            `json:"result"`
	Metrics       GenerationMetrics `json:"metrics"`
	PromptVersion string            `json:"prompt_version"`
	SuggestionID  *uuid.UUID        `json:"suggestion_id,omitempty"`
	RiskProtocol  *string           `json:"risk_protocol,omitempty"`
	AIRunID       *uuid.UUID        `json:"ai_run_id,omitempty"`
}

func NewService(provider ClinicalInferenceProvider, appointments AppointmentRepository, access ClinicalAccess, auditor Auditor) *Service {
	return &Service{provider: provider, appointments: appointments, access: access, auditor: auditor}
}

func (s *Service) WithLongitudinalMemory(memory clinicalMemory) *Service {
	s.memory = memory
	return s
}

func (s *Service) WithMetrics(metrics Metrics) *Service {
	s.metrics = metrics
	return s
}

func (s *Service) WithRiskProtocol(protocol string) *Service {
	s.riskProtocol = strings.TrimSpace(protocol)
	return s
}

func (s *Service) WithRunTracking(runs *clinicalairun.Service, provider, model string, parameters map[string]any) *Service {
	s.runs, s.runProvider, s.runModel, s.runParameters = runs, strings.TrimSpace(provider), strings.TrimSpace(model), parameters
	return s
}

func (s *Service) Status(ctx context.Context) (ProviderStatus, error) {
	if s.provider == nil {
		return ProviderStatus{Message: ErrProviderUnavailable.Error()}, ErrProviderUnavailable
	}
	return s.provider.Health(ctx)
}

func (s *Service) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if s.provider == nil {
		return nil, ErrProviderUnavailable
	}
	return s.provider.ListModels(ctx)
}

func (s *Service) Warm(ctx context.Context) error {
	if s.provider == nil {
		return ErrProviderUnavailable
	}
	return s.provider.WarmModel(ctx)
}

func (s *Service) Unload(ctx context.Context) error {
	if s.provider == nil {
		return ErrProviderUnavailable
	}
	return s.provider.UnloadModel(ctx)
}

func (s *Service) WithConsent(a consent.Authorizer) *Service { s.consent = a; return s }
func (s *Service) AnalyzeLive(ctx context.Context, input AnalyzeLiveInput, onProgress func(GenerationProgress)) (output AnalyzeLiveOutput, err error) {
	contextChars := len([]rune(input.Request.Fragment + input.Request.PreviousIntervention + input.Request.PatientResponse))
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordClinicalAnalysis("live", output.Metrics.FirstToken, output.Metrics.Total, output.Metrics.EvalCount, output.Metrics.EvalRate, output.Metrics.Repaired, contextChars, err)
		}
	}()
	if err := validateAnalyzeInput(input); err != nil {
		return AnalyzeLiveOutput{}, err
	}
	if s.provider == nil || s.appointments == nil || s.access == nil {
		return AnalyzeLiveOutput{}, ErrProviderUnavailable
	}
	appointment, err := s.appointments.GetByID(ctx, input.TenantID, input.AppointmentID)
	if err != nil {
		return AnalyzeLiveOutput{}, fmt.Errorf("get appointment for local analysis: %w", err)
	}
	allowed, err := s.access.CanAccessClient(ctx, input.TenantID, input.ActorUserID, appointment.ClientID, "treating")
	if err != nil {
		return AnalyzeLiveOutput{}, fmt.Errorf("check treating assignment for local analysis: %w", err)
	}
	if !allowed {
		return AnalyzeLiveOutput{}, domainerrors.ErrForbidden
	}
	if s.consent != nil {
		if _, err = s.consent.Authorize(ctx, input.TenantID, appointment.ClientID, consent.LocalAI, "live_analysis", input.AppointmentID); err != nil {
			return AnalyzeLiveOutput{}, err
		}
	}
	request := input.Request
	var runSources []clinicalairun.Source
	if s.memory != nil {
		runSources = []clinicalairun.Source{}
		// Longitudinal context is server-owned. Browser-supplied summaries or anchors
		// are never trusted when the persistent clinical memory is enabled.
		request.ApprovedSummary = ""
		request.RelevantContext = nil
		approved, err := s.memory.GetApprovedContext(ctx, input.TenantID, appointment.ClientID)
		if err != nil && !errors.Is(err, domainerrors.ErrNotFound) {
			return AnalyzeLiveOutput{}, fmt.Errorf("load approved longitudinal context: %w", err)
		}
		if err == nil {
			version := approved.Version
			runSources = append(runSources, clinicalairun.Source{SourceType: clinicalairun.SourceFormulationSnapshot, SourceID: approved.SnapshotID, SourceVersion: &version})
			contextChars += len([]rune(approved.ApprovedSummary))
			request.ApprovedSummary = approved.ApprovedSummary
			request.RelevantContext = make([]ContextItem, 0, len(approved.Anchors))
			for _, anchor := range approved.Anchors {
				if len(request.RelevantContext) == 4 {
					break
				}
				level := EpistemicHypothesis
				if anchor.Kind == "fact" {
					level = EpistemicFact
				}
				request.RelevantContext = append(request.RelevantContext, ContextItem{SourceID: anchor.SourceID, Kind: level, Summary: anchor.Summary})
				runSources = append(runSources, clinicalairun.Source{SourceType: clinicalairun.SourceFormulationAnchor, SourceID: anchor.ID, SourceVersion: &version})
				contextChars += len([]rune(anchor.Summary))
			}
		}
	}
	if err := s.audit(ctx, input, "clinical_ai.analysis.start", map[string]any{
		"mode":           "live",
		"prompt_version": PromptVersion,
		"fragment_chars": len([]rune(input.Request.Fragment)),
	}); err != nil {
		return AnalyzeLiveOutput{}, fmt.Errorf("audit local analysis start: %w", err)
	}
	var runID *uuid.UUID
	if s.runs != nil {
		primaryInput := struct {
			Fragment             string
			PreviousIntervention string
			PreviousAction       NowAction
			PatientResponse      string
		}{request.Fragment, request.PreviousIntervention, request.PreviousAction, request.PatientResponse}
		run, runErr := s.runs.Start(ctx, clinicalairun.StartInput{TenantID: input.TenantID, ClientID: appointment.ClientID, AppointmentID: &input.AppointmentID, CreatedByUserID: input.ActorUserID, Provider: s.runProvider, Model: s.runModel, Operation: "analyze_live", PromptName: "clinical-live", PromptVersion: PromptVersion, Parameters: s.runParameters, Input: primaryInput, Context: struct {
			ApprovedSummary string
			RelevantContext []ContextItem
		}{request.ApprovedSummary, request.RelevantContext}, Sources: runSources})
		if runErr != nil {
			return AnalyzeLiveOutput{}, fmt.Errorf("start clinical AI run: %w", runErr)
		}
		runID = &run.ID
		defer func() {
			if runID != nil && err != nil {
				_, _ = s.runs.Fail(context.WithoutCancel(ctx), input.TenantID, *runID, clinicalRunErrorCode(err), errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
			}
		}()
	}

	providerOut, err := s.provider.AnalyzeLive(ctx, LiveSystemPrompt(), request, onProgress)
	if err != nil {
		_ = s.audit(ctx, input, "clinical_ai.analysis.failed", map[string]any{"mode": "live", "error_class": classifyProviderError(err)})
		return AnalyzeLiveOutput{}, err
	}
	result, err := decodeResult(providerOut.JSON)
	if err != nil {
		_ = s.audit(ctx, input, "clinical_ai.analysis.failed", map[string]any{"mode": "live", "error_class": "invalid_output"})
		return AnalyzeLiveOutput{}, fmt.Errorf("%w: %v", ErrInvalidModelOutput, err)
	}
	result = ApplyDeterministicGuards(request, result)
	if err := ValidateResult(result); err != nil {
		_ = s.audit(ctx, input, "clinical_ai.analysis.failed", map[string]any{"mode": "live", "error_class": "invalid_output"})
		return AnalyzeLiveOutput{}, fmt.Errorf("%w: %v", ErrInvalidModelOutput, err)
	}

	var suggestionID *uuid.UUID
	if s.memory != nil {
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return AnalyzeLiveOutput{}, fmt.Errorf("encode validated suggestion: %w", err)
		}
		suggestion, err := s.memory.CreateSuggestion(ctx, clinicalmemory.CreateSuggestionInput{
			TenantID: input.TenantID, ClientID: appointment.ClientID, AppointmentID: input.AppointmentID,
			ActorUserID: input.ActorUserID, Mode: "live", PromptVersion: PromptVersion, Result: resultJSON, AIRunID: runID,
		})
		if err != nil {
			return AnalyzeLiveOutput{}, fmt.Errorf("persist separate AI suggestion: %w", err)
		}
		suggestionID = &suggestion.ID
	}
	if runID != nil {
		if _, finishErr := s.runs.Succeed(ctx, input.TenantID, *runID, result); finishErr != nil {
			return AnalyzeLiveOutput{}, fmt.Errorf("finish clinical AI run: %w", finishErr)
		}
	}
	if err := s.audit(ctx, input, "clinical_ai.analysis.complete", map[string]any{
		"mode":              "live",
		"prompt_version":    PromptVersion,
		"first_token_ms":    providerOut.Metrics.FirstToken.Milliseconds(),
		"total_ms":          providerOut.Metrics.Total.Milliseconds(),
		"eval_count":        providerOut.Metrics.EvalCount,
		"tokens_per_second": providerOut.Metrics.EvalRate,
		"repaired":          providerOut.Metrics.Repaired,
		"risk_flagged":      result.Risk.Detected,
	}); err != nil {
		return AnalyzeLiveOutput{}, fmt.Errorf("audit local analysis completion: %w", err)
	}
	var riskProtocol *string
	if result.Risk.Detected && s.riskProtocol != "" {
		protocol := s.riskProtocol
		riskProtocol = &protocol
	}
	return AnalyzeLiveOutput{Result: result, Metrics: providerOut.Metrics, PromptVersion: PromptVersion, SuggestionID: suggestionID, RiskProtocol: riskProtocol, AIRunID: runID}, nil
}

func (s *Service) ReviewSession(ctx context.Context, input ReviewSessionInput, onProgress func(GenerationProgress)) (output ReviewOutput, err error) {
	contextChars := len([]rune(input.SessionText))
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordClinicalAnalysis("review", output.Metrics.FirstToken, output.Metrics.Total, output.Metrics.EvalCount, output.Metrics.EvalRate, output.Metrics.Repaired, contextChars, err)
		}
	}()
	if input.TenantID == uuid.Nil || input.ActorUserID == uuid.Nil || input.AppointmentID == uuid.Nil {
		return ReviewOutput{}, domainerrors.NewValidation("tenant_id, actor_user_id and appointment_id are required")
	}
	text := strings.TrimSpace(input.SessionText)
	if length := len([]rune(text)); length < 20 || length > 40000 {
		return ReviewOutput{}, domainerrors.NewValidation("session_text must contain between 20 and 40000 characters")
	}
	if s.provider == nil || s.appointments == nil || s.access == nil {
		return ReviewOutput{}, ErrProviderUnavailable
	}
	appointment, err := s.appointments.GetByID(ctx, input.TenantID, input.AppointmentID)
	if err != nil {
		return ReviewOutput{}, fmt.Errorf("get appointment for local review: %w", err)
	}
	allowed, err := s.access.CanAccessClient(ctx, input.TenantID, input.ActorUserID, appointment.ClientID, "treating")
	if err != nil {
		return ReviewOutput{}, fmt.Errorf("check treating assignment for local review: %w", err)
	}
	if !allowed {
		return ReviewOutput{}, domainerrors.ErrForbidden
	}

	if s.consent != nil {
		if _, err = s.consent.Authorize(ctx, input.TenantID, appointment.ClientID, consent.LocalAI, "session_review", input.AppointmentID); err != nil {
			return ReviewOutput{}, err
		}
	}
	request := ReviewRequest{SessionText: text}
	var runSources []clinicalairun.Source
	if s.memory != nil {
		runSources = []clinicalairun.Source{}
		approved, memoryErr := s.memory.GetApprovedContext(ctx, input.TenantID, appointment.ClientID)
		if memoryErr != nil && !errors.Is(memoryErr, domainerrors.ErrNotFound) {
			return ReviewOutput{}, fmt.Errorf("load approved longitudinal context: %w", memoryErr)
		}
		if memoryErr == nil {
			version := approved.Version
			runSources = append(runSources, clinicalairun.Source{SourceType: clinicalairun.SourceFormulationSnapshot, SourceID: approved.SnapshotID, SourceVersion: &version})
			request.ApprovedSummary = approved.ApprovedSummary
			contextChars += len([]rune(approved.ApprovedSummary))
			for _, anchor := range approved.Anchors {
				if len(request.RelevantContext) == 4 {
					break
				}
				kind := EpistemicHypothesis
				if anchor.Kind == "fact" {
					kind = EpistemicFact
				}
				request.RelevantContext = append(request.RelevantContext, ContextItem{SourceID: anchor.SourceID, Kind: kind, Summary: anchor.Summary})
				runSources = append(runSources, clinicalairun.Source{SourceType: clinicalairun.SourceFormulationAnchor, SourceID: anchor.ID, SourceVersion: &version})
				contextChars += len([]rune(anchor.Summary))
			}
		}
	}
	if err := s.auditReview(ctx, input, "clinical_ai.analysis.start", map[string]any{"mode": "review", "prompt_version": ReviewPromptVersion, "session_chars": len([]rune(text))}); err != nil {
		return ReviewOutput{}, fmt.Errorf("audit local review start: %w", err)
	}
	var runID *uuid.UUID
	if s.runs != nil {
		run, runErr := s.runs.Start(ctx, clinicalairun.StartInput{TenantID: input.TenantID, ClientID: appointment.ClientID, AppointmentID: &input.AppointmentID, CreatedByUserID: input.ActorUserID, Provider: s.runProvider, Model: s.runModel, Operation: "review_session", PromptName: "clinical-review", PromptVersion: ReviewPromptVersion, Parameters: s.runParameters, Input: struct{ SessionText string }{text}, Context: struct {
			ApprovedSummary string
			RelevantContext []ContextItem
		}{request.ApprovedSummary, request.RelevantContext}, Sources: runSources})
		if runErr != nil {
			return ReviewOutput{}, fmt.Errorf("start clinical AI run: %w", runErr)
		}
		runID = &run.ID
		defer func() {
			if runID != nil && err != nil {
				_, _ = s.runs.Fail(context.WithoutCancel(ctx), input.TenantID, *runID, clinicalRunErrorCode(err), errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
			}
		}()
	}
	providerOut, err := s.provider.ReviewSession(ctx, ReviewSystemPrompt(), request, onProgress)
	if err != nil {
		_ = s.auditReview(ctx, input, "clinical_ai.analysis.failed", map[string]any{"mode": "review", "error_class": classifyProviderError(err)})
		return ReviewOutput{}, err
	}
	result, err := decodeReviewResult(providerOut.JSON)
	if err != nil {
		_ = s.auditReview(ctx, input, "clinical_ai.analysis.failed", map[string]any{"mode": "review", "error_class": "invalid_output"})
		return ReviewOutput{}, fmt.Errorf("%w: %v", ErrInvalidModelOutput, err)
	}
	result = applyReviewGuards(request, result)
	encoded, err := json.Marshal(result)
	if err != nil {
		return ReviewOutput{}, fmt.Errorf("encode validated review: %w", err)
	}
	var suggestionID *uuid.UUID
	if s.memory != nil {
		suggestion, createErr := s.memory.CreateSuggestion(ctx, clinicalmemory.CreateSuggestionInput{TenantID: input.TenantID, ClientID: appointment.ClientID, AppointmentID: input.AppointmentID, ActorUserID: input.ActorUserID, Mode: "review", PromptVersion: ReviewPromptVersion, Result: encoded, AIRunID: runID})
		if createErr != nil {
			return ReviewOutput{}, fmt.Errorf("persist separate review suggestion: %w", createErr)
		}
		suggestionID = &suggestion.ID
	}
	if runID != nil {
		if _, finishErr := s.runs.Succeed(ctx, input.TenantID, *runID, result); finishErr != nil {
			return ReviewOutput{}, fmt.Errorf("finish clinical AI run: %w", finishErr)
		}
	}
	if err := s.auditReview(ctx, input, "clinical_ai.analysis.complete", map[string]any{"mode": "review", "prompt_version": ReviewPromptVersion, "first_token_ms": providerOut.Metrics.FirstToken.Milliseconds(), "total_ms": providerOut.Metrics.Total.Milliseconds(), "eval_count": providerOut.Metrics.EvalCount, "tokens_per_second": providerOut.Metrics.EvalRate, "repaired": providerOut.Metrics.Repaired, "risk_flagged": result.Risk.Detected}); err != nil {
		return ReviewOutput{}, fmt.Errorf("audit local review completion: %w", err)
	}
	var riskProtocol *string
	if result.Risk.Detected && s.riskProtocol != "" {
		protocol := s.riskProtocol
		riskProtocol = &protocol
	}
	return ReviewOutput{Result: result, Metrics: providerOut.Metrics, PromptVersion: ReviewPromptVersion, SuggestionID: suggestionID, RiskProtocol: riskProtocol, AIRunID: runID}, nil
}

func clinicalRunErrorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrProviderBusy):
		return "provider_busy"
	case errors.Is(err, ErrProviderUnavailable):
		return "provider_unavailable"
	case errors.Is(err, ErrInvalidModelOutput):
		return "invalid_output"
	default:
		return "pipeline_failed"
	}
}

func (s *Service) auditReview(ctx context.Context, input ReviewSessionInput, action string, metadata map[string]any) error {
	if s.auditor == nil {
		return nil
	}
	appointmentID := input.AppointmentID
	return s.auditor.RecordDomainEvent(ctx, input.TenantID, input.ActorUserID, action, "appointment", &appointmentID, metadata)
}

func decodeReviewResult(raw []byte) (ReviewResult, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var result ReviewResult
	if err := decoder.Decode(&result); err != nil {
		return ReviewResult{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ReviewResult{}, errors.New("model output contains trailing data")
	}
	if result.Mode != "review" || strings.TrimSpace(result.EmergingFormulation) == "" {
		return ReviewResult{}, errors.New("invalid review mode or empty formulation")
	}
	if len(result.EffectiveInterventions) > 6 || len(result.WeakOrRiskyInterventions) > 6 || len(result.TherapistPatterns) > 6 || len(result.NextFocus) > 6 || len(result.Evidence) > 8 {
		return ReviewResult{}, errors.New("review output exceeds bounded lists")
	}
	return result, nil
}

func applyReviewGuards(request ReviewRequest, result ReviewResult) ReviewResult {
	allowed := map[string]bool{"current_session": true}
	for _, item := range request.RelevantContext {
		allowed[item.SourceID] = true
	}
	filtered := result.Evidence[:0]
	for _, evidence := range result.Evidence {
		if allowed[evidence.SourceID] && (evidence.Kind == EpistemicFact || evidence.Kind == EpistemicInference) {
			filtered = append(filtered, evidence)
		}
	}
	result.Evidence = filtered
	category, detected := DetectRisk(request.SessionText)
	if detected || result.Risk.Detected || result.Risk.RequiresHumanAssessment {
		result.Risk.Detected = true
		result.Risk.RequiresHumanAssessment = true
		if category != "" {
			result.Risk.Category = &category
		}
		result.Caution = "Posible indicador de riesgo: requiere evaluación humana según el protocolo configurado; no está confirmado ni descartado."
		result.NextFocus = []string{"Evaluar riesgo, regulación y protección antes de profundizar interpretaciones."}
	}
	return result
}

func validateAnalyzeInput(input AnalyzeLiveInput) error {
	if input.TenantID == uuid.Nil {
		return domainerrors.NewValidation("tenant_id is required")
	}
	if input.ActorUserID == uuid.Nil {
		return domainerrors.NewValidation("actor_user_id is required")
	}
	if input.AppointmentID == uuid.Nil {
		return domainerrors.NewValidation("appointment_id is required")
	}
	fragmentLength := len([]rune(strings.TrimSpace(input.Request.Fragment)))
	if fragmentLength == 0 || fragmentLength > 12000 {
		return domainerrors.NewValidation("fragment must contain between 1 and 12000 characters")
	}
	if len([]rune(input.Request.PreviousIntervention)) > 2000 {
		return domainerrors.NewValidation("previous_intervention cannot exceed 2000 characters")
	}
	if len([]rune(input.Request.PatientResponse)) > 2000 {
		return domainerrors.NewValidation("patient_response cannot exceed 2000 characters")
	}
	if len(input.Request.RelevantContext) > 4 {
		return domainerrors.NewValidation("relevant_context cannot contain more than 4 items")
	}
	return nil
}

func decodeResult(raw []byte) (Result, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var result Result
	if err := decoder.Decode(&result); err != nil {
		return Result{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Result{}, errors.New("model output contains trailing data")
	}
	return result, nil
}

func (s *Service) audit(ctx context.Context, input AnalyzeLiveInput, action string, metadata map[string]any) error {
	if s.auditor == nil {
		return nil
	}
	appointmentID := input.AppointmentID
	return s.auditor.RecordDomainEvent(ctx, input.TenantID, input.ActorUserID, action, "appointment", &appointmentID, metadata)
}

func classifyProviderError(err error) string {
	switch {
	case errors.Is(err, ErrProviderBusy):
		return "busy"
	case errors.Is(err, ErrProviderUnavailable):
		return "unavailable"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "provider_error"
	}
}
