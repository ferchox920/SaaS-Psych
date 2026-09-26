package longitudinal

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/clinicalairun"
	"sessionflow/apps/api/internal/usecase/clinicalanalysis"
)

const (
	GIRAPromptName    = "clinical-gira-builder"
	GIRAPromptVersion = "clinical-gira-builder-v1.1"
)

type GIRABuilderInput struct {
	SchemaVersion      string                `json:"schema_version"`
	SelectedProcess    Process               `json:"selected_process"`
	ApprovedEvidence   []Evidence            `json:"approved_evidence"`
	ApprovedEvents     []Event               `json:"approved_events"`
	ApprovedHypotheses []Hypothesis          `json:"approved_hypotheses"`
	CurrentStrategy    TherapeuticStrategy   `json:"current_strategy"`
	ApproachRegistry   []ApproachDefinition  `json:"approach_registry"`
	TechniqueRegistry  []TechniqueDefinition `json:"technique_registry"`
	StateVersion       int64                 `json:"state_version"`
}

type GIRABuilderProvider interface {
	BuildGIRA(context.Context, string, GIRAProviderRequest, func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error)
}

// GIRAProviderRequest contains only the allow-listed remote context and the
// frozen semantic schema. Candidate validation executes locally and exposes no
// UUID map or domain snapshot to provider adapters.
type GIRAProviderRequest struct {
	Context  RemoteGIRAContext `json:"context"`
	Schema   map[string]any    `json:"schema"`
	validate func([]byte) error
}

func (r GIRAProviderRequest) ValidateCandidate(candidate []byte) error {
	if r.validate == nil {
		return domainerrors.NewValidation("GIRA provider candidate validator is required")
	}
	return r.validate(candidate)
}

func NewGIRAProviderRequest(generation GIRAGenerationRequest) GIRAProviderRequest {
	request := GIRAProviderRequest{Context: generation.RemoteContext, Schema: GIRASemanticJSONSchema()}
	request.validate = func(candidate []byte) error {
		semantic, err := DecodeGIRASemanticProposal(candidate)
		if err != nil {
			return err
		}
		_, err = CompileGIRASemanticProposal(generation, semantic, uuid.New)
		return err
	}
	return request
}

type GIRAAnalysisOutput struct {
	Diff Diff `json:"diff"`
}

func (s *Service) WithGIRABuilder(provider GIRABuilderProvider) *Service {
	s.giraProvider = provider
	return s
}

func (s *Service) WithConfiguredGIRABuilder(provider GIRABuilderProvider, providerName, model string, parameters map[string]any) *Service {
	s.giraProvider = provider
	s.giraProviderName = providerName
	s.giraModel = model
	s.giraParameters = parameters
	return s
}

func (s *Service) BuildGIRA(ctx context.Context, tenantID, processID, actorID uuid.UUID) (out GIRAAnalysisOutput, err error) {
	started := s.now()
	defer func() {
		if s.metrics != nil {
			result := "success"
			if err != nil {
				result = "error"
			}
			s.metrics.RecordGIRABuild(result, s.now().Sub(started))
		}
	}()
	if s.giraProvider == nil || s.runs == nil {
		return out, domainerrors.NewValidation("GIRA builder is not configured")
	}
	process, err := s.repo.GetProcess(ctx, tenantID, processID)
	if err != nil {
		return out, err
	}
	if process.ApprovalStatus != "approved" {
		return out, domainerrors.NewValidation("GIRA builder requires an approved clinical process")
	}
	if err := s.require(ctx, tenantID, actorID, process.ClientID, "treating"); err != nil {
		return out, err
	}
	state, err := s.repo.State(ctx, tenantID, process.ClientID)
	if err != nil {
		return out, err
	}
	if err := validateGIRAOutboundOwnership(tenantID, process.ClientID, process, state); err != nil {
		return out, err
	}
	approaches, err := s.repo.ListApproaches(ctx)
	if err != nil {
		return out, err
	}
	techniques, err := s.repo.ListTechniques(ctx)
	if err != nil {
		return out, err
	}
	strategy := TherapeuticStrategy{Targets: []Target{}, Goals: []Goal{}, Rationales: []TherapeuticRationale{}, GIRAs: []GIRA{}}
	if process.TherapeuticStrategy != nil {
		strategy = *process.TherapeuticStrategy
	}
	contextStarted := s.now()
	input := GIRABuilderInput{
		SchemaVersion: GIRAPromptVersion, SelectedProcess: process,
		ApprovedEvidence:   append([]Evidence(nil), state.ActiveEvidence...),
		ApprovedEvents:     append([]Event(nil), state.RecentEvents...),
		ApprovedHypotheses: append([]Hypothesis(nil), process.Hypotheses...),
		CurrentStrategy:    strategy, ApproachRegistry: approaches, TechniqueRegistry: techniques,
		StateVersion: state.StateVersion,
	}
	input = BoundGIRAContext(input)
	generationRequest := BuildGIRAGenerationRequest(input)
	providerRequest := NewGIRAProviderRequest(generationRequest)
	providerName, model, parameters := s.providerName, s.model, s.parameters
	if s.giraProviderName != "" {
		providerName, model, parameters = s.giraProviderName, s.giraModel, s.giraParameters
	}
	if s.metrics != nil {
		s.metrics.RecordGIRAStage("context_build", s.now().Sub(contextStarted))
		for component, size := range MeasureGIRAContext(generationRequest.RemoteContext) {
			s.metrics.RecordGIRAContextSize(component, size)
		}
	}
	privacyAudit := map[string]any{"provider": providerName, "model": model, "field_counts": generationRequest.Privacy.FieldCounts, "payload_bytes": generationRequest.Privacy.PayloadBytes, "minimization_policy_version": generationRequest.Privacy.PolicyVersion, "risk_flags": generationRequest.Privacy.ResidualRiskFlags}
	if s.auditor != nil {
		if err = s.auditor.RecordDomainEvent(ctx, tenantID, actorID, "remote_gira.request_prepared", "clinical_process", &processID, privacyAudit); err != nil {
			return out, fmt.Errorf("audit prepared GIRA request: %w", err)
		}
	}
	sources := giraSources(tenantID, input)
	run, err := s.runs.Start(ctx, clinicalairun.StartInput{TenantID: tenantID, ClientID: process.ClientID, CreatedByUserID: actorID, Provider: providerName, Model: model, Operation: "gira_build", PromptName: GIRAPromptName, PromptVersion: GIRAPromptVersion, Parameters: parameters, Input: generationRequest.RemoteContext, Context: generationRequest.RemoteContext, Sources: sources})
	if err != nil {
		return out, fmt.Errorf("start GIRA AI run: %w", err)
	}
	finished := false
	defer func() {
		if !finished && err != nil {
			cancelled := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
			_, _ = s.runs.Fail(context.WithoutCancel(ctx), tenantID, run.ID, longitudinalErrorCode(err), cancelled)
		}
	}()
	providerStarted := s.now()
	if s.auditor != nil {
		if err = s.auditor.RecordDomainEvent(ctx, tenantID, actorID, "remote_gira.request_sent", "clinical_process", &processID, privacyAudit); err != nil {
			return out, fmt.Errorf("audit sent GIRA request: %w", err)
		}
	}
	providerOut, err := s.giraProvider.BuildGIRA(ctx, GIRASystemPromptV1(), providerRequest, nil)
	if s.metrics != nil {
		s.metrics.RecordGIRAStage("provider", s.now().Sub(providerStarted))
		if providerOut.Metrics.RepairDuration > 0 {
			s.metrics.RecordGIRAStage("repair", providerOut.Metrics.RepairDuration)
		}
	}
	if err != nil {
		if s.auditor != nil {
			failedAudit := cloneAuditMetadata(privacyAudit)
			failedAudit["normalized_error_code"] = longitudinalErrorCode(err)
			_ = s.auditor.RecordDomainEvent(context.WithoutCancel(ctx), tenantID, actorID, "remote_gira.request_failed", "clinical_process", &processID, failedAudit)
		}
		return out, err
	}
	if s.auditor != nil {
		receivedAudit := cloneAuditMetadata(privacyAudit)
		for key, value := range providerOut.Metadata.SafeMap() {
			receivedAudit[key] = value
		}
		if err = s.auditor.RecordDomainEvent(ctx, tenantID, actorID, "remote_gira.response_received", "clinical_process", &processID, receivedAudit); err != nil {
			return out, fmt.Errorf("audit received GIRA response: %w", err)
		}
	}
	validationStarted := s.now()
	semantic, err := DecodeGIRASemanticProposal(providerOut.JSON)
	if s.metrics != nil {
		s.metrics.RecordGIRAStage("validation", s.now().Sub(validationStarted))
	}
	if err != nil {
		return out, fmt.Errorf("invalid GIRA builder output: %w", err)
	}
	compileStarted := s.now()
	result, err := CompileGIRASemanticProposal(generationRequest, semantic, uuid.New)
	if s.metrics != nil {
		s.metrics.RecordGIRAStage("diff_compile", s.now().Sub(compileStarted))
	}
	if err != nil {
		return out, fmt.Errorf("invalid GIRA builder semantics: %w", err)
	}
	operations := make([]Operation, 0, len(result.Operations))
	for i, proposal := range result.Operations {
		operations = append(operations, Operation{ID: uuid.New(), TenantID: tenantID, ClientID: process.ClientID, Sequence: i + 1, OperationType: proposal.OperationType, TargetEntityID: *proposal.TargetEntityID, ExpectedEntityVersion: proposal.ExpectedEntityVersion, OriginalProposal: proposal.Proposal, ReviewStatus: "pending"})
	}
	metadata := map[string]any{"generation_repaired": providerOut.Metrics.Repaired, "repair_class": providerOut.Metrics.RepairReason, "primary_duration_ms": providerOut.Metrics.PrimaryDuration.Milliseconds(), "repair_duration_ms": providerOut.Metrics.RepairDuration.Milliseconds(), "provider_duration_ms": providerOut.Metrics.Total.Milliseconds(), "eval_count": providerOut.Metrics.EvalCount, "output_bytes": len(providerOut.JSON), "minimization_policy_version": generationRequest.Privacy.PolicyVersion, "remote_payload_bytes": generationRequest.Privacy.PayloadBytes, "residual_risk_flags": generationRequest.Privacy.ResidualRiskFlags}
	for key, value := range providerOut.Metadata.SafeMap() {
		metadata[key] = value
	}
	diff, err := s.repo.CreateDiff(ctx, CreateDiffInput{TenantID: tenantID, ClientID: process.ClientID, RunID: run.ID, ActorID: actorID, BaseStateVersion: state.StateVersion, Operations: operations, Uncertainties: result.Uncertainties, OutputHash: clinicalairun.Hash(result), RunMetadata: metadata})
	if err != nil {
		return out, err
	}
	finished = true
	s.runs.RecordPersistedCompletion("gira_build", clinicalairun.StatusSucceeded)
	if s.metrics != nil {
		s.metrics.RecordClinicalDiffCreated("success")
		for _, op := range operations {
			s.metrics.RecordGIRADiffOperation(op.OperationType)
		}
	}
	return GIRAAnalysisOutput{Diff: diff}, nil
}

func cloneAuditMetadata(input map[string]any) map[string]any {
	out := make(map[string]any, len(input)+1)
	for key, value := range input {
		out[key] = value
	}
	return out
}

func validateGIRAOutboundOwnership(tenantID, clientID uuid.UUID, process Process, state State) error {
	invalid := func(t, c uuid.UUID) bool {
		return (t != uuid.Nil && t != tenantID) || (c != uuid.Nil && c != clientID)
	}
	if invalid(process.TenantID, process.ClientID) || (state.ClientID != uuid.Nil && state.ClientID != clientID) {
		return domainerrors.NewValidation("GIRA context ownership mismatch")
	}
	for _, item := range state.ActiveEvidence {
		if invalid(item.TenantID, item.ClientID) {
			return domainerrors.NewValidation("GIRA evidence ownership mismatch")
		}
	}
	for _, item := range state.RecentEvents {
		if invalid(item.TenantID, item.ClientID) {
			return domainerrors.NewValidation("GIRA event ownership mismatch")
		}
	}
	for _, item := range process.Hypotheses {
		if invalid(item.TenantID, item.ClientID) {
			return domainerrors.NewValidation("GIRA hypothesis ownership mismatch")
		}
	}
	if process.TherapeuticStrategy != nil {
		for _, item := range process.TherapeuticStrategy.Targets {
			if invalid(item.TenantID, item.ClientID) {
				return domainerrors.NewValidation("GIRA target ownership mismatch")
			}
		}
		for _, item := range process.TherapeuticStrategy.Goals {
			if invalid(item.TenantID, item.ClientID) {
				return domainerrors.NewValidation("GIRA goal ownership mismatch")
			}
		}
		for _, item := range process.TherapeuticStrategy.Rationales {
			if invalid(item.TenantID, item.ClientID) {
				return domainerrors.NewValidation("GIRA rationale ownership mismatch")
			}
		}
		for _, item := range process.TherapeuticStrategy.GIRAs {
			if invalid(item.TenantID, item.ClientID) {
				return domainerrors.NewValidation("GIRA ownership mismatch")
			}
		}
	}
	return nil
}

func giraSources(t uuid.UUID, in GIRABuilderInput) []clinicalairun.Source {
	out := []clinicalairun.Source{}
	add := func(kind string, id uuid.UUID, version int) {
		v := version
		out = appendSourceUnique(out, clinicalairun.Source{TenantID: t, SourceType: kind, SourceID: id, SourceVersion: &v})
	}
	add(clinicalairun.SourceClinicalProcess, in.SelectedProcess.ID, in.SelectedProcess.Version)
	for _, e := range in.ApprovedEvidence {
		add(clinicalairun.SourceClinicalEvidence, e.ID, e.Version)
	}
	for _, e := range in.ApprovedEvents {
		add(clinicalairun.SourceClinicalEvent, e.ID, e.Version)
	}
	for _, h := range in.ApprovedHypotheses {
		add(clinicalairun.SourceClinicalHypothesis, h.ID, h.Version)
	}
	for _, target := range in.CurrentStrategy.Targets {
		add(clinicalairun.SourceClinicalTarget, target.ID, target.Version)
	}
	for _, goal := range in.CurrentStrategy.Goals {
		add(clinicalairun.SourceClinicalGoal, goal.ID, goal.Version)
		for _, indicator := range goal.Indicators {
			add(clinicalairun.SourceGoalIndicator, indicator.ID, indicator.Version)
		}
	}
	for _, rationale := range in.CurrentStrategy.Rationales {
		add(clinicalairun.SourceTherapeuticRationale, rationale.ID, rationale.Version)
	}
	for _, gira := range in.CurrentStrategy.GIRAs {
		add(clinicalairun.SourceGIRA, gira.ID, gira.EntityVersion)
		for _, phase := range gira.Phases {
			add(clinicalairun.SourceGIRAPhase, phase.ID, phase.Version)
		}
	}
	for _, approach := range in.ApproachRegistry {
		add(clinicalairun.SourceApproachDefinition, approach.ID, approach.Version)
	}
	for _, technique := range in.TechniqueRegistry {
		add(clinicalairun.SourceTechniqueDefinition, technique.ID, technique.Version)
	}
	return out
}
