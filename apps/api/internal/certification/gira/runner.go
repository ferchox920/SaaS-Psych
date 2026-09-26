package gira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
)

type Fixture struct {
	Name                      string
	Synthetic                 bool
	Input                     longitudinal.GIRABuilderInput
	AllowBackendRejection     bool
	ForbiddenOutboundLiterals []string
	Assert                    func(longitudinal.InterpreterResult) error
}

type Outcome struct {
	Result       longitudinal.InterpreterResult
	Diff         longitudinal.Diff
	Provider     clinicalanalysis.ProviderOutput
	PayloadBytes int
	AIRun        clinicalairun.Run
}

type Runner struct {
	Provider     longitudinal.GIRABuilderProvider
	ProviderName string
	Model        string
}

func (r Runner) Run(ctx context.Context, fixture Fixture) (Outcome, error) {
	if !fixture.Synthetic {
		return Outcome{}, errors.New("GIRA certification refuses non-synthetic fixture")
	}
	if r.Provider == nil {
		return Outcome{}, errors.New("GIRA certification provider is required")
	}
	if err := validateCertificationOutbound(longitudinal.BuildGIRAGenerationRequest(fixture.Input).RemoteContext, fixture.ForbiddenOutboundLiterals); err != nil {
		return Outcome{}, err
	}
	repo := &runnerRepo{input: fixture.Input}
	runs := &runnerRunRepo{}
	recording := &recordingProvider{provider: r.Provider, forbidden: fixture.ForbiddenOutboundLiterals}
	service := longitudinal.NewService(repo, runnerAccess{}, nil, clinicalairun.NewService(runs), nil, nil, "certification", "unset", map[string]any{"synthetic_fixture_only": true}).WithConfiguredGIRABuilder(recording, r.ProviderName, r.Model, map[string]any{"synthetic_fixture_only": true})
	out, err := service.BuildGIRA(ctx, fixture.Input.SelectedProcess.TenantID, fixture.Input.SelectedProcess.ID, uuid.New())
	if err != nil {
		return Outcome{Provider: recording.out, PayloadBytes: longitudinal.BuildGIRAGenerationRequest(fixture.Input).Privacy.PayloadBytes, AIRun: runs.run}, err
	}
	result := longitudinal.InterpreterResult{Uncertainties: out.Diff.Uncertainties}
	for _, operation := range out.Diff.Operations {
		id := operation.TargetEntityID
		result.Operations = append(result.Operations, longitudinal.ProposedOperation{ID: operation.ID.String(), OperationType: operation.OperationType, TargetEntityID: &id, ExpectedEntityVersion: operation.ExpectedEntityVersion, Proposal: operation.OriginalProposal})
	}
	if fixture.Assert != nil {
		if err := fixture.Assert(result); err != nil {
			return Outcome{Result: result, Diff: out.Diff, Provider: recording.out, PayloadBytes: longitudinal.BuildGIRAGenerationRequest(fixture.Input).Privacy.PayloadBytes, AIRun: runs.run}, err
		}
	}
	return Outcome{Result: result, Diff: out.Diff, Provider: recording.out, PayloadBytes: longitudinal.BuildGIRAGenerationRequest(fixture.Input).Privacy.PayloadBytes, AIRun: runs.run}, nil
}

type recordingProvider struct {
	provider  longitudinal.GIRABuilderProvider
	out       clinicalanalysis.ProviderOutput
	forbidden []string
}

func (p *recordingProvider) BuildGIRA(ctx context.Context, prompt string, request longitudinal.GIRAProviderRequest, progress func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	if err := validateCertificationOutbound(request.Context, p.forbidden); err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	out, err := p.provider.BuildGIRA(ctx, prompt, request, progress)
	p.out = out
	return out, err
}

func validateCertificationOutbound(context longitudinal.RemoteGIRAContext, forbidden []string) error {
	raw, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("certification outbound serialization: %w", err)
	}
	serialized := string(raw)
	for _, literal := range forbidden {
		if literal != "" && strings.Contains(serialized, literal) {
			return errors.New("GIRA certification blocked a forbidden outbound literal")
		}
	}
	for _, pattern := range certificationForbiddenPatterns {
		if pattern.Match(raw) {
			return errors.New("GIRA certification blocked a direct identifier pattern")
		}
	}
	return nil
}

var certificationForbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`),
	regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`),
	regexp.MustCompile(`(?i)\bTEST-DNI-[A-Z0-9-]+\b`),
}

type runnerRepo struct {
	input longitudinal.GIRABuilderInput
}

func (r *runnerRepo) GetProcess(context.Context, uuid.UUID, uuid.UUID) (longitudinal.Process, error) {
	return r.input.SelectedProcess, nil
}
func (r *runnerRepo) State(context.Context, uuid.UUID, uuid.UUID) (longitudinal.State, error) {
	return longitudinal.State{ClientID: r.input.SelectedProcess.ClientID, StateVersion: r.input.StateVersion, ActiveEvidence: r.input.ApprovedEvidence, RecentEvents: r.input.ApprovedEvents, Processes: []longitudinal.Process{r.input.SelectedProcess}}, nil
}
func (r *runnerRepo) ListApproaches(context.Context) ([]longitudinal.ApproachDefinition, error) {
	return r.input.ApproachRegistry, nil
}
func (r *runnerRepo) ListTechniques(context.Context) ([]longitudinal.TechniqueDefinition, error) {
	return r.input.TechniqueRegistry, nil
}
func (r *runnerRepo) CreateDiff(_ context.Context, in longitudinal.CreateDiffInput) (longitudinal.Diff, error) {
	return longitudinal.Diff{ID: uuid.New(), TenantID: in.TenantID, ClientID: in.ClientID, BaseStateVersion: in.BaseStateVersion, Revision: 1, Status: "pending_review", Operations: in.Operations, Uncertainties: in.Uncertainties}, nil
}
func (*runnerRepo) SessionAnalysis(context.Context, uuid.UUID, uuid.UUID) (longitudinal.SessionAnalysis, error) {
	return longitudinal.SessionAnalysis{}, domainerrors.ErrNotFound
}
func (*runnerRepo) FindOpenDiff(context.Context, uuid.UUID, uuid.UUID, int64) (longitudinal.Diff, error) {
	return longitudinal.Diff{}, domainerrors.ErrNotFound
}
func (*runnerRepo) ListEvidence(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Evidence, error) {
	return nil, nil
}
func (*runnerRepo) ListEvents(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Event, error) {
	return nil, nil
}
func (*runnerRepo) ListProcesses(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Process, error) {
	return nil, nil
}
func (*runnerRepo) ListHypotheses(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Hypothesis, error) {
	return nil, nil
}
func (*runnerRepo) ListTargets(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Target, error) {
	return nil, nil
}
func (*runnerRepo) ListGoals(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Goal, error) {
	return nil, nil
}
func (*runnerRepo) ListGIRAs(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.GIRA, error) {
	return nil, nil
}
func (*runnerRepo) GetGIRA(context.Context, uuid.UUID, uuid.UUID) (longitudinal.GIRA, error) {
	return longitudinal.GIRA{}, domainerrors.ErrNotFound
}
func (*runnerRepo) GetStrategyHistory(context.Context, uuid.UUID, uuid.UUID, string, uuid.UUID) (longitudinal.StrategyHistory, error) {
	return longitudinal.StrategyHistory{}, nil
}
func (*runnerRepo) ListDiffs(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Diff, error) {
	return nil, nil
}
func (*runnerRepo) GetDiff(context.Context, uuid.UUID, uuid.UUID) (longitudinal.Diff, error) {
	return longitudinal.Diff{}, domainerrors.ErrNotFound
}
func (*runnerRepo) Decide(context.Context, longitudinal.DecisionInput) (longitudinal.Diff, error) {
	return longitudinal.Diff{}, nil
}
func (*runnerRepo) Merge(context.Context, longitudinal.MergeInput) (longitudinal.Diff, error) {
	return longitudinal.Diff{}, nil
}
func (*runnerRepo) GetProcessHistory(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (longitudinal.ProcessHistory, error) {
	return longitudinal.ProcessHistory{}, nil
}
func (*runnerRepo) GetHypothesisHistory(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (longitudinal.HypothesisHistory, error) {
	return longitudinal.HypothesisHistory{}, nil
}

type runnerAccess struct{}

func (runnerAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return true, nil
}

type runnerRunRepo struct{ run clinicalairun.Run }

func (r *runnerRunRepo) Start(_ context.Context, run clinicalairun.Run) (clinicalairun.Run, error) {
	r.run = run
	return run, nil
}
func (r *runnerRunRepo) Finish(_ context.Context, _, _ uuid.UUID, status string, output, code *string, at time.Time) (clinicalairun.Run, error) {
	r.run.Status, r.run.OutputHash, r.run.ErrorCode, r.run.CompletedAt = status, output, code, &at
	return r.run, nil
}
func (r *runnerRunRepo) ListSources(context.Context, uuid.UUID, uuid.UUID) ([]clinicalairun.Source, error) {
	return r.run.Sources, nil
}

func FixturesAThroughJ() []Fixture {
	tenant, client, processID, evidenceID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	registry := []longitudinal.ApproachDefinition{{ID: uuid.New(), Slug: "cbt", Version: 1, Name: "TCC", Description: "Ciclos de mantenimiento, evitación y experimentos conductuales", Status: "active"}, {ID: uuid.New(), Slug: "act", Version: 1, Name: "ACT", Description: "Flexibilidad, valores y acción comprometida", Status: "active"}}
	techniques := []longitudinal.TechniqueDefinition{{ID: uuid.New(), Slug: "behavioral_experiment", Version: 1, ApproachSlug: "cbt", ApproachVersion: 1, Name: "Experimento conductual", Mechanism: "Contrastar predicciones", Status: "active"}, {ID: uuid.New(), Slug: "values_committed_action", Version: 1, ApproachSlug: "act", ApproachVersion: 1, Name: "Acción comprometida", Mechanism: "Acción guiada por valores", Status: "active"}}
	empty := longitudinal.TherapeuticStrategy{Targets: []longitudinal.Target{}, Goals: []longitudinal.Goal{}, Rationales: []longitudinal.TherapeuticRationale{}, GIRAs: []longitudinal.GIRA{}}
	base := func(description, statement string) longitudinal.GIRABuilderInput {
		p := longitudinal.Process{ID: processID, TenantID: tenant, ClientID: client, Title: "Proceso terapéutico ficticio", Description: description, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1, Events: []longitudinal.Event{}, Hypotheses: []longitudinal.Hypothesis{}, TherapeuticStrategy: &empty}
		return longitudinal.GIRABuilderInput{SchemaVersion: longitudinal.GIRAPromptVersion, SelectedProcess: p, ApprovedEvidence: []longitudinal.Evidence{{ID: evidenceID, TenantID: tenant, ClientID: client, EpistemicType: "patient_report", Statement: statement, Status: "active", Version: 1}}, ApprovedEvents: []longitudinal.Event{}, ApprovedHypotheses: []longitudinal.Hypothesis{}, CurrentStrategy: empty, ApproachRegistry: registry, TechniqueRegistry: techniques, StateVersion: 1}
	}
	approaches := func(r longitudinal.InterpreterResult) map[string]uuid.UUID {
		out := map[string]uuid.UUID{}
		for _, op := range r.Operations {
			if op.OperationType == "create_therapeutic_rationale" {
				var proposal longitudinal.CreateTherapeuticRationaleProposal
				_ = json.Unmarshal(op.Proposal, &proposal)
				out[proposal.ApproachSlug] = proposal.TargetID
			}
		}
		return out
	}
	find := func(r longitudinal.InterpreterResult, kind string) []longitudinal.ProposedOperation {
		out := []longitudinal.ProposedOperation{}
		for _, op := range r.Operations {
			if op.OperationType == kind {
				out = append(out, op)
			}
		}
		return out
	}
	A := base("Ciclo claro de evitación conductual; una sola función TCC es suficiente. Proponer target, goal operacionalizado, indicador cualitativo, rationale TCC, GIRA y fase. No usar ACT.", "Evita conversaciones; el alivio inmediato mantiene la evitación.")
	B := base("Dos mecanismos distintos: evitación conductual y conflicto entre culpa y valores. Justificar TCC para el primero y ACT para el segundo, cada uno con target, goal, indicador y rationale propios.", "Evita conversaciones y abandona acciones elegidas cuando aparece culpa.")
	C := base("Caso conductual simple. No apilar enfoques; usar como máximo TCC.", "Evita una conversación específica por alivio inmediato.")
	D := base("No aceptar 'mejorar autoestima' como goal amorfo. Operacionalizarlo con conducta observable e indicador.", "Busca aprobación y revierte límites tras sentirse culpable.")
	E := base("Crear indicador cualitativo sin baseline ni target numérico arbitrario.", "Mantener el límite aun con culpa es el cambio observable relevante.")
	makeProgress := func(regression bool) longitudinal.GIRABuilderInput {
		input := base("Vincular la nueva evidencia al indicador; no lograr ni completar el goal.", "Mantuvo el límite pese a culpa.")
		if regression {
			input.ApprovedEvidence[0].Statement = "Volvió a revertir el límite y aumentó reassurance seeking."
		}
		targetID, goalID, indicatorID := uuid.New(), uuid.New(), uuid.New()
		input.CurrentStrategy = longitudinal.TherapeuticStrategy{Targets: []longitudinal.Target{{ID: targetID, ProcessID: processID, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1}}, Goals: []longitudinal.Goal{{ID: goalID, ProcessID: processID, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1, TargetIDs: []uuid.UUID{targetID}, Indicators: []longitudinal.GoalIndicator{{ID: indicatorID, GoalID: goalID, Description: "Mantiene límite", IndicatorType: "qualitative", Status: "active", Version: 1}}}}, Rationales: []longitudinal.TherapeuticRationale{}, GIRAs: []longitudinal.GIRA{}}
		input.SelectedProcess.TherapeuticStrategy = &input.CurrentStrategy
		return input
	}
	F, G := makeProgress(false), makeProgress(true)
	H := base("La formulación cambió materialmente. Proponer GIRA v2 que supersede v1 sin modificar ni destruir v1.", "Nueva evidencia cambia el foco terapéutico.")
	targetID, goalID, rationaleID, oldGIRA := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	H.CurrentStrategy = longitudinal.TherapeuticStrategy{Targets: []longitudinal.Target{{ID: targetID, ProcessID: processID, ApprovalStatus: "approved", Version: 1}}, Goals: []longitudinal.Goal{{ID: goalID, ProcessID: processID, ApprovalStatus: "approved", Version: 1}}, Rationales: []longitudinal.TherapeuticRationale{{ID: rationaleID, ProcessID: processID, TargetID: targetID, GoalID: goalID, ApproachSlug: "cbt", ApproachVersion: 1, GroundingStatus: "grounded", ApprovalStatus: "approved", Version: 1}}, GIRAs: []longitudinal.GIRA{{ID: oldGIRA, ProcessID: processID, GIRAVersion: 1, ApprovalStatus: "approved", ClinicalStatus: "active", EntityVersion: 1}}}
	H.SelectedProcess.TherapeuticStrategy = &H.CurrentStrategy
	I := H
	I.SelectedProcess.Description = "La GIRA histórica usa CBT v1 deprecado. No reescribir ni perder esa referencia histórica."
	I.ApproachRegistry = append([]longitudinal.ApproachDefinition(nil), registry...)
	I.ApproachRegistry[0].Status = "deprecated"
	J := H
	J.ApprovedEvidence = []longitudinal.Evidence{}
	J.SelectedProcess.Description = "No hay evidencia autorizada para el mecanismo de una técnica nueva. No crear rationale grounded; devolver insufficient_evidence."
	fixtures := []Fixture{
		{Name: "A_single_approach", Synthetic: true, Input: A, Assert: func(result longitudinal.InterpreterResult) error {
			a := approaches(result)
			if len(a) != 1 || a["cbt"] == uuid.Nil {
				return fmt.Errorf("expected one CBT approach: %v", a)
			}
			return nil
		}},
		{Name: "B_justified_integration", Synthetic: true, Input: B, Assert: func(result longitudinal.InterpreterResult) error {
			a := approaches(result)
			if a["cbt"] == uuid.Nil || a["act"] == uuid.Nil || a["cbt"] == a["act"] {
				return fmt.Errorf("distinct CBT/ACT target functions absent: %v", a)
			}
			return nil
		}},
		{Name: "C_no_eclectic_stack", Synthetic: true, Input: C, Assert: func(result longitudinal.InterpreterResult) error {
			if len(approaches(result)) > 1 {
				return fmt.Errorf("unjustified stack: %v", approaches(result))
			}
			return nil
		}},
		{Name: "D_operational_goal", Synthetic: true, Input: D, Assert: func(result longitudinal.InterpreterResult) error {
			for _, op := range find(result, "create_goal") {
				var proposal longitudinal.CreateGoalProposal
				_ = json.Unmarshal(op.Proposal, &proposal)
				if strings.EqualFold(strings.TrimSpace(proposal.Title), "mejorar autoestima") {
					return errors.New("amorphous goal accepted")
				}
			}
			return nil
		}},
		{Name: "E_qualitative_indicator", Synthetic: true, Input: E, Assert: func(result longitudinal.InterpreterResult) error {
			for _, op := range find(result, "create_goal_indicator") {
				var proposal longitudinal.CreateGoalIndicatorProposal
				_ = json.Unmarshal(op.Proposal, &proposal)
				if proposal.IndicatorType == "qualitative" && proposal.Baseline == nil && proposal.TargetValue == nil {
					return nil
				}
			}
			return errors.New("qualitative nonnumeric indicator absent")
		}},
		{Name: "F_progress_diff_not_achievement", Synthetic: true, Input: F, Assert: func(result longitudinal.InterpreterResult) error {
			if len(find(result, "achieve_goal")) > 0 {
				return errors.New("AI proposed achievement")
			}
			for _, op := range find(result, "link_indicator_evidence") {
				var proposal longitudinal.LinkIndicatorSourceProposal
				_ = json.Unmarshal(op.Proposal, &proposal)
				if proposal.RelationType == "supports_progress" {
					return nil
				}
			}
			return errors.New("progress relation absent")
		}},
		{Name: "G_regression_history_preserved", Synthetic: true, Input: G, Assert: func(result longitudinal.InterpreterResult) error {
			for _, op := range find(result, "link_indicator_evidence") {
				var proposal longitudinal.LinkIndicatorSourceProposal
				_ = json.Unmarshal(op.Proposal, &proposal)
				if proposal.RelationType == "supports_regression" {
					return nil
				}
			}
			return errors.New("regression relation absent")
		}},
		{Name: "H_new_gira_version", Synthetic: true, Input: H, Assert: func(result longitudinal.InterpreterResult) error {
			for _, op := range find(result, "create_gira") {
				var proposal longitudinal.CreateGIRAProposal
				_ = json.Unmarshal(op.Proposal, &proposal)
				if proposal.GIRAVersion == 2 && proposal.SupersedesGIRAID != nil && *proposal.SupersedesGIRAID == oldGIRA {
					return nil
				}
			}
			return errors.New("GIRA v2 supersession absent")
		}},
		{Name: "I_deprecated_version_resolves", Synthetic: true, Input: I, Assert: func(result longitudinal.InterpreterResult) error {
			for _, op := range find(result, "update_therapeutic_rationale") {
				var proposal longitudinal.CreateTherapeuticRationaleProposal
				_ = json.Unmarshal(op.Proposal, &proposal)
				if proposal.ApproachSlug == "cbt" && proposal.ApproachVersion != 1 {
					return errors.New("historical version was rewritten")
				}
			}
			return nil
		}},
		{Name: "J_unsupported_mechanism", Synthetic: true, Input: J, AllowBackendRejection: true, Assert: func(result longitudinal.InterpreterResult) error {
			for _, op := range result.Operations {
				if op.OperationType == "create_therapeutic_rationale" {
					return errors.New("unsupported rationale accepted")
				}
			}
			for _, uncertainty := range result.Uncertainties {
				if uncertainty.Type == "insufficient_evidence" {
					return nil
				}
			}
			return errors.New("missing insufficient_evidence")
		}},
	}
	for i := range fixtures {
		fixtures[i].ForbiddenOutboundLiterals = []string{tenant.String(), client.String(), processID.String(), evidenceID.String()}
	}
	return fixtures
}
