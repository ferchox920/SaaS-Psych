package longitudinal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"sessionflow/apps/api/internal/usecase/clinicalairun"
	"sessionflow/apps/api/internal/usecase/clinicalanalysis"
)

type giraRunRepo struct {
	started bool
	run     clinicalairun.Run
}

func (r *giraRunRepo) Start(_ context.Context, run clinicalairun.Run) (clinicalairun.Run, error) {
	r.started = true
	r.run = run
	return run, nil
}
func (r *giraRunRepo) Finish(_ context.Context, _, _ uuid.UUID, status string, output, code *string, at time.Time) (clinicalairun.Run, error) {
	r.run.Status = status
	r.run.OutputHash = output
	r.run.ErrorCode = code
	r.run.CompletedAt = &at
	return r.run, nil
}
func (r *giraRunRepo) ListSources(context.Context, uuid.UUID, uuid.UUID) ([]clinicalairun.Source, error) {
	return r.run.Sources, nil
}

type giraRepo struct {
	reuseRepo
	process    Process
	state      State
	approaches []ApproachDefinition
	techniques []TechniqueDefinition
	created    *CreateDiffInput
}

func (r *giraRepo) GetProcess(context.Context, uuid.UUID, uuid.UUID) (Process, error) {
	return r.process, nil
}
func (r *giraRepo) State(context.Context, uuid.UUID, uuid.UUID) (State, error) { return r.state, nil }
func (r *giraRepo) ListApproaches(context.Context) ([]ApproachDefinition, error) {
	return r.approaches, nil
}
func (r *giraRepo) ListTechniques(context.Context) ([]TechniqueDefinition, error) {
	return r.techniques, nil
}
func (r *giraRepo) CreateDiff(_ context.Context, in CreateDiffInput) (Diff, error) {
	r.created = &in
	return Diff{ID: uuid.New(), TenantID: in.TenantID, ClientID: in.ClientID, Status: "pending_review", BaseStateVersion: in.BaseStateVersion, Revision: 1, Operations: in.Operations, Uncertainties: in.Uncertainties}, nil
}

type fixtureGIRAProvider struct {
	fixture string
	runs    *giraRunRepo
}

type cancellingGIRAProvider struct{}

type recordingGIRAProvider struct{ called bool }

func (p *recordingGIRAProvider) BuildGIRA(context.Context, string, GIRAProviderRequest, func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	p.called = true
	return clinicalanalysis.ProviderOutput{}, nil
}

func (cancellingGIRAProvider) BuildGIRA(context.Context, string, GIRAProviderRequest, func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	return clinicalanalysis.ProviderOutput{}, context.Canceled
}

func (p fixtureGIRAProvider) BuildGIRA(_ context.Context, _ string, in GIRAProviderRequest, _ func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	if !p.runs.started {
		return clinicalanalysis.ProviderOutput{}, errors.New("provider invoked before AI run")
	}
	result := fixtureGIRASemanticResult(p.fixture, in.Context)
	raw, _ := json.Marshal(result)
	return clinicalanalysis.ProviderOutput{JSON: raw}, nil
}

func fixtureGIRASemanticResult(fixture string, in RemoteGIRAContext) GIRASemanticProposal {
	empty := GIRASemanticProposal{Targets: []GIRASemanticTarget{}, Goals: []GIRASemanticGoal{}, Indicators: []GIRASemanticIndicator{}, Rationales: []GIRASemanticRationale{}, Phases: []GIRASemanticPhase{}, IndicatorLinks: []GIRASemanticIndicatorLink{}, Uncertainties: []GIRASemanticUncertainty{}}
	if fixture == "F" || fixture == "G" {
		relation := "supports_progress"
		if fixture == "G" {
			relation = "supports_regression"
		}
		empty.IndicatorLinks = []GIRASemanticIndicatorLink{{IndicatorRef: "existing_indicator_1_1", SourceType: "evidence", SourceRef: "evidence_1", RelationType: relation, EvidenceRefs: []string{"evidence_1"}}}
		return empty
	}
	if fixture == "H" {
		empty.GIRA = &GIRASemanticGIRA{Ref: "gira_2", Title: "GIRA v2", Summary: "Revisión material sin sobrescribir v1", SupersedesGIRARef: stringPtr("existing_gira_1"), TargetRefs: []string{"existing_target_1"}, GoalRefs: []string{"existing_goal_1"}, RationaleRefs: []string{"existing_rationale_1"}}
		return empty
	}
	if fixture == "I" {
		empty.Uncertainties = []GIRASemanticUncertainty{{Type: "explore", Question: "Mantener resolución histórica de la definición deprecada", EvidenceRefs: []string{"evidence_1"}}}
		return empty
	}
	title := "Sostener límite ante culpa anticipatoria"
	if fixture == "D" {
		title = "Iniciar una acción concreta coherente con autovaloración en dos situaciones relevantes"
	}
	approach, technique, targetType := "cbt", "behavioral_experiment", "behavioral_pattern"
	if fixture == "J" {
		approach, technique, targetType = "act", "values_committed_action", "meaning_value_conflict"
	}
	empty.Targets = []GIRASemanticTarget{{Ref: "target_1", Title: "Evitación funcional", Description: "Evitación ante desaprobación anticipada", TargetType: targetType, EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}, EventRefs: []string{}}}
	empty.Goals = []GIRASemanticGoal{{Ref: "goal_1", Title: title, Description: "Conducta observable vinculada al target", GoalType: "behavior_change", Priority: "high", TargetRefs: []string{"target_1"}}}
	empty.Indicators = []GIRASemanticIndicator{{Ref: "indicator_1", GoalRef: "goal_1", Description: "La persona inicia y mantiene la conducta acordada aun con malestar", IndicatorType: "qualitative"}}
	grounding := []string{"evidence_1"}
	if fixture == "J" {
		grounding = []string{}
	}
	empty.Rationales = []GIRASemanticRationale{{Ref: "rationale_1", TargetRef: "target_1", GoalRef: "goal_1", ApproachSlug: approach, ApproachVersion: 1, TechniqueSlug: &technique, TechniqueVersion: intPtr(1), Rationale: "Función clínica específica sustentada", ExpectedEffect: "Aumentar conducta funcional observable", EvidenceRefs: grounding, HypothesisRefs: []string{}}}
	if fixture == "B" {
		actTechnique := "values_committed_action"
		empty.Targets = append(empty.Targets, GIRASemanticTarget{Ref: "target_2", Title: "Conflicto de valores", Description: "Culpa interfiere con acción elegida", TargetType: "meaning_value_conflict", EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}, EventRefs: []string{}})
		empty.Goals = append(empty.Goals, GIRASemanticGoal{Ref: "goal_2", Title: "Actuar según valor elegido pese a culpa", Description: "Acción observable", GoalType: "meaning_reconstruction", Priority: "medium", TargetRefs: []string{"target_2"}})
		empty.Indicators = append(empty.Indicators, GIRASemanticIndicator{Ref: "indicator_2", GoalRef: "goal_2", Description: "Realiza acción elegida sin exigir eliminar culpa", IndicatorType: "behavioral"})
		empty.Rationales = append(empty.Rationales, GIRASemanticRationale{Ref: "rationale_2", TargetRef: "target_2", GoalRef: "goal_2", ApproachSlug: "act", ApproachVersion: 1, TechniqueSlug: &actTechnique, TechniqueVersion: intPtr(1), Rationale: "Flexibilidad y acción valiosa cumplen función distinta", ExpectedEffect: "Acción guiada por valores", EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}})
	}
	if fixture != "J" {
		targetRefs, goalRefs, rationaleRefs := []string{"target_1"}, []string{"goal_1"}, []string{"rationale_1"}
		if fixture == "B" {
			targetRefs = append(targetRefs, "target_2")
			goalRefs = append(goalRefs, "goal_2")
			rationaleRefs = append(rationaleRefs, "rationale_2")
		}
		empty.GIRA = &GIRASemanticGIRA{Ref: "gira_1", Title: "Ruta terapéutica", Summary: "Estrategia estructurada", TargetRefs: targetRefs, GoalRefs: goalRefs, RationaleRefs: rationaleRefs}
		empty.Phases = []GIRASemanticPhase{{Ref: "phase_1", GIRARef: "gira_1", Position: 1, Title: "Intervención inicial", Description: "Fase caso-específica", GoalRefs: goalRefs, RationaleRefs: rationaleRefs, IndicatorRefs: []string{"indicator_1"}}}
	}
	return empty
}

func stringPtr(v string) *string { return &v }

func proposed(id uuid.UUID, kind string, version *int, payload any) ProposedOperation {
	raw, _ := json.Marshal(payload)
	return ProposedOperation{ID: uuid.NewString(), OperationType: kind, TargetEntityID: &id, ExpectedEntityVersion: version, Proposal: raw}
}

func fixtureGIRAResult(fixture string, in GIRABuilderInput) InterpreterResult {
	evidenceID := in.ApprovedEvidence[0].ID
	if fixture == "F" || fixture == "G" {
		goal := in.CurrentStrategy.Goals[0]
		indicator := goal.Indicators[0]
		relation := "supports_progress"
		if fixture == "G" {
			relation = "supports_regression"
		}
		v := indicator.Version
		return InterpreterResult{Operations: []ProposedOperation{proposed(indicator.ID, "link_indicator_evidence", &v, LinkIndicatorSourceProposal{IndicatorID: indicator.ID, SourceID: evidenceID, RelationType: relation, EvidenceIDs: []uuid.UUID{evidenceID}})}, Uncertainties: []Uncertainty{}}
	}
	if fixture == "H" {
		old := in.CurrentStrategy.GIRAs[0]
		id := uuid.New()
		return InterpreterResult{Operations: []ProposedOperation{proposed(id, "create_gira", nil, CreateGIRAProposal{ProcessID: in.SelectedProcess.ID, GIRAVersion: old.GIRAVersion + 1, Title: "GIRA v2", Summary: "Revisión material sin sobrescribir v1", SupersedesGIRAID: &old.ID, TargetIDs: []uuid.UUID{in.CurrentStrategy.Targets[0].ID}, GoalIDs: []uuid.UUID{in.CurrentStrategy.Goals[0].ID}, RationaleIDs: []uuid.UUID{in.CurrentStrategy.Rationales[0].ID}})}, Uncertainties: []Uncertainty{}}
	}
	if fixture == "I" {
		return InterpreterResult{Operations: []ProposedOperation{}, Uncertainties: []Uncertainty{{Type: "explore", Question: "Mantener resolución histórica de la definición deprecada", EvidenceIDs: []uuid.UUID{evidenceID}}}}
	}
	target, goal, indicator, rationale, gira, phase := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	title := "Sostener límite ante culpa anticipatoria"
	if fixture == "D" {
		title = "Iniciar una acción concreta coherente con autovaloración en dos situaciones relevantes"
	}
	approach, technique, targetType := "cbt", "behavioral_experiment", "behavioral_pattern"
	if fixture == "J" {
		approach = "act"
		technique = "values_committed_action"
		targetType = "meaning_value_conflict"
	}
	ops := []ProposedOperation{
		proposed(target, "create_target", nil, CreateTargetProposal{ProcessID: in.SelectedProcess.ID, Title: "Evitación funcional", Description: "Evitación ante desaprobación anticipada", TargetType: targetType, EvidenceIDs: []uuid.UUID{evidenceID}, HypothesisIDs: []uuid.UUID{}, EventIDs: []uuid.UUID{}}),
		proposed(goal, "create_goal", nil, CreateGoalProposal{ProcessID: in.SelectedProcess.ID, Title: title, Description: "Conducta observable vinculada al target", GoalType: "behavior_change", Priority: "high", TargetIDs: []uuid.UUID{target}}),
		proposed(indicator, "create_goal_indicator", nil, CreateGoalIndicatorProposal{GoalID: goal, Description: "La persona inicia y mantiene la conducta acordada aun con malestar", IndicatorType: "qualitative"}),
	}
	grounding := []uuid.UUID{evidenceID}
	if fixture == "J" {
		grounding = []uuid.UUID{}
	}
	ops = append(ops, proposed(rationale, "create_therapeutic_rationale", nil, CreateTherapeuticRationaleProposal{ProcessID: in.SelectedProcess.ID, TargetID: target, GoalID: goal, ApproachSlug: approach, ApproachVersion: 1, TechniqueSlug: &technique, TechniqueVersion: intPtr(1), Rationale: "Función clínica específica sustentada por la evidencia", ExpectedEffect: "Aumentar conducta funcional observable", GroundingStatus: "grounded", EvidenceIDs: grounding, HypothesisIDs: []uuid.UUID{}}))
	if fixture == "B" {
		t2, g2, i2, r2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		actTechnique := "values_committed_action"
		ops = append(ops,
			proposed(t2, "create_target", nil, CreateTargetProposal{ProcessID: in.SelectedProcess.ID, Title: "Conflicto de valores", Description: "Culpa interfiere con acción elegida", TargetType: "meaning_value_conflict", EvidenceIDs: []uuid.UUID{evidenceID}, HypothesisIDs: []uuid.UUID{}, EventIDs: []uuid.UUID{}}),
			proposed(g2, "create_goal", nil, CreateGoalProposal{ProcessID: in.SelectedProcess.ID, Title: "Actuar según valor elegido pese a culpa", Description: "Acción elegida y observable", GoalType: "meaning_reconstruction", Priority: "medium", TargetIDs: []uuid.UUID{t2}}),
			proposed(i2, "create_goal_indicator", nil, CreateGoalIndicatorProposal{GoalID: g2, Description: "Realiza acción elegida sin exigir eliminar culpa", IndicatorType: "behavioral"}),
			proposed(r2, "create_therapeutic_rationale", nil, CreateTherapeuticRationaleProposal{ProcessID: in.SelectedProcess.ID, TargetID: t2, GoalID: g2, ApproachSlug: "act", ApproachVersion: 1, TechniqueSlug: &actTechnique, TechniqueVersion: intPtr(1), Rationale: "Flexibilidad y acción valiosa cumplen una función distinta", ExpectedEffect: "Acción guiada por valores", GroundingStatus: "grounded", EvidenceIDs: []uuid.UUID{evidenceID}, HypothesisIDs: []uuid.UUID{}}),
		)
	}
	if fixture != "J" {
		targets, goals, rationales := []uuid.UUID{target}, []uuid.UUID{goal}, []uuid.UUID{rationale}
		if fixture == "B" {
			targets = append(targets, *ops[4].TargetEntityID)
			goals = append(goals, *ops[5].TargetEntityID)
			rationales = append(rationales, *ops[7].TargetEntityID)
		}
		ops = append(ops, proposed(gira, "create_gira", nil, CreateGIRAProposal{ProcessID: in.SelectedProcess.ID, GIRAVersion: 1, Title: "Ruta terapéutica", Summary: "Estrategia estructurada", TargetIDs: targets, GoalIDs: goals, RationaleIDs: rationales}), proposed(phase, "create_gira_phase", nil, CreateGIRAPhaseProposal{GIRAID: gira, Position: 1, Title: "Intervención inicial", Description: "Fase caso-específica", GoalIDs: goals, RationaleIDs: rationales, IndicatorIDs: []uuid.UUID{indicator}}))
	}
	return InterpreterResult{Operations: ops, Uncertainties: []Uncertainty{}}
}

func intPtr(v int) *int { return &v }

func TestGIRABuilderFixturesAThroughJRealServicePipeline(t *testing.T) {
	for _, fixture := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"} {
		t.Run(fixture, func(t *testing.T) {
			tenant, client, actor, processID, evidenceID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
			process := Process{ID: processID, TenantID: tenant, ClientID: client, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1, Hypotheses: []Hypothesis{}}
			strategy := TherapeuticStrategy{Targets: []Target{}, Goals: []Goal{}, Rationales: []TherapeuticRationale{}, GIRAs: []GIRA{}}
			if fixture == "F" || fixture == "G" || fixture == "H" || fixture == "I" {
				targetID, goalID, indicatorID, rationaleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
				strategy.Targets = []Target{{ID: targetID, ProcessID: processID, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1}}
				strategy.Goals = []Goal{{ID: goalID, ProcessID: processID, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1, Indicators: []GoalIndicator{{ID: indicatorID, GoalID: goalID, Version: 1, Status: "active"}}}}
				strategy.Rationales = []TherapeuticRationale{{ID: rationaleID, ProcessID: processID, TargetID: targetID, GoalID: goalID, ApproachSlug: "cbt", ApproachVersion: 1, ApprovalStatus: "approved", GroundingStatus: "grounded", Version: 1}}
				if fixture == "H" || fixture == "I" {
					strategy.GIRAs = []GIRA{{ID: uuid.New(), ProcessID: processID, GIRAVersion: 1, ApprovalStatus: "approved", ClinicalStatus: "active", EntityVersion: 1, TargetIDs: []uuid.UUID{targetID}, GoalIDs: []uuid.UUID{goalID}, Rationales: strategy.Rationales}}
				}
			}
			process.TherapeuticStrategy = &strategy
			repo := &giraRepo{process: process, state: State{ClientID: client, StateVersion: 4, ActiveEvidence: []Evidence{{ID: evidenceID, TenantID: tenant, ClientID: client, Status: "active", Version: 1}}, RecentEvents: []Event{}, Processes: []Process{process}}, approaches: []ApproachDefinition{{ID: uuid.New(), Slug: "cbt", Version: 1, Status: "active"}, {ID: uuid.New(), Slug: "act", Version: 1, Status: "active"}}, techniques: []TechniqueDefinition{{ID: uuid.New(), Slug: "behavioral_experiment", Version: 1, ApproachSlug: "cbt", ApproachVersion: 1}, {ID: uuid.New(), Slug: "values_committed_action", Version: 1, ApproachSlug: "act", ApproachVersion: 1}}}
			runs := &giraRunRepo{}
			runService := clinicalairun.NewService(runs)
			provider := fixtureGIRAProvider{fixture: fixture, runs: runs}
			service := NewService(repo, allowAccess{}, nil, runService, nil, nil, "fixture", "fixture", map[string]any{}).WithGIRABuilder(provider)
			out, err := service.BuildGIRA(context.Background(), tenant, processID, actor)
			if fixture == "J" {
				if err == nil {
					t.Fatal("unsupported mechanism was accepted")
				}
				if repo.created != nil {
					t.Fatal("invalid output persisted a diff")
				}
				if runs.run.Status != clinicalairun.StatusFailed {
					t.Fatalf("invalid output did not leave terminal failed run: %s", runs.run.Status)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if out.Diff.Status != "pending_review" || repo.created == nil {
				t.Fatalf("diff=%#v", out.Diff)
			}
			for _, op := range out.Diff.Operations {
				if op.ReviewStatus != "pending" {
					t.Fatalf("operation mutated state: %#v", op)
				}
			}
		})
	}
}

func TestGIRABuilderRejectsAmorphousGoalWithoutIndicator(t *testing.T) {
	processID, evidenceID, targetID, goalID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	in := GIRABuilderInput{SelectedProcess: Process{ID: processID}, ApprovedEvidence: []Evidence{{ID: evidenceID, Status: "active"}}, CurrentStrategy: TherapeuticStrategy{}, ApproachRegistry: []ApproachDefinition{}, TechniqueRegistry: []TechniqueDefinition{}}
	r := InterpreterResult{Operations: []ProposedOperation{proposed(targetID, "create_target", nil, CreateTargetProposal{ProcessID: processID, Title: "Target", Description: "Grounded", TargetType: "other", EvidenceIDs: []uuid.UUID{evidenceID}, HypothesisIDs: []uuid.UUID{}, EventIDs: []uuid.UUID{}}), proposed(goalID, "create_goal", nil, CreateGoalProposal{ProcessID: processID, Title: "Improve self-esteem", Description: "Feel better", GoalType: "other", Priority: "medium", TargetIDs: []uuid.UUID{targetID}})}, Uncertainties: []Uncertainty{}}
	if err := ValidateGIRABuilderResult(in, r); err == nil {
		t.Fatal("amorphous unmeasured goal accepted")
	}
}

func TestGIRABuilderRequiresTreatingAccess(t *testing.T) {
	tenant, client, actor, processID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &giraRepo{process: Process{ID: processID, TenantID: tenant, ClientID: client, ApprovalStatus: "approved", Version: 1}}
	runs := &giraRunRepo{}
	service := NewService(repo, denyAccess{}, nil, clinicalairun.NewService(runs), nil, nil, "fixture", "fixture", nil).WithGIRABuilder(cancellingGIRAProvider{})
	if _, err := service.BuildGIRA(context.Background(), tenant, processID, actor); err == nil {
		t.Fatal("GIRA builder accepted actor without CA-W")
	}
	if runs.started {
		t.Fatal("AI run started before authorization")
	}
}

func TestGIRABuilderRejectsCrossClientContextBeforeRunOrProvider(t *testing.T) {
	tenant, client, otherClient, processID, actor := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &giraRepo{
		process: Process{ID: processID, TenantID: tenant, ClientID: client, ApprovalStatus: "approved", Version: 1},
		state:   State{ClientID: client, StateVersion: 1, ActiveEvidence: []Evidence{{ID: uuid.New(), TenantID: tenant, ClientID: otherClient, Status: "active", Version: 1}}},
	}
	runs := &giraRunRepo{}
	provider := &recordingGIRAProvider{}
	service := NewService(repo, allowAccess{}, nil, clinicalairun.NewService(runs), nil, nil, "fixture", "fixture", nil).WithGIRABuilder(provider)
	_, err := service.BuildGIRA(context.Background(), tenant, processID, actor)
	if err == nil || provider.called || runs.started || repo.created != nil {
		t.Fatalf("cross-client context escaped boundary: provider=%t run=%t diff=%t err=%v", provider.called, runs.started, repo.created != nil, err)
	}
}

func TestGIRABuilderCancellationTerminatesRunWithoutDiff(t *testing.T) {
	tenant, client, actor, processID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := &giraRepo{
		process:    Process{ID: processID, TenantID: tenant, ClientID: client, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1},
		state:      State{ClientID: client, StateVersion: 1, ActiveEvidence: []Evidence{}, RecentEvents: []Event{}},
		approaches: []ApproachDefinition{}, techniques: []TechniqueDefinition{},
	}
	runs := &giraRunRepo{}
	service := NewService(repo, allowAccess{}, nil, clinicalairun.NewService(runs), nil, nil, "fixture", "fixture", nil).WithGIRABuilder(cancellingGIRAProvider{})
	if _, err := service.BuildGIRA(context.Background(), tenant, processID, actor); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if runs.run.Status != clinicalairun.StatusCancelled || repo.created != nil {
		t.Fatalf("run_status=%s diff=%#v", runs.run.Status, repo.created)
	}
}

func TestGIRABuilderCannotCompleteClinicalState(t *testing.T) {
	for _, operation := range []string{"achieve_goal", "complete_gira", "complete_gira_phase"} {
		t.Run(operation, func(t *testing.T) {
			id, version := uuid.New(), 1
			result := InterpreterResult{Operations: []ProposedOperation{proposed(id, operation, &version, map[string]any{"evidence_ids": []uuid.UUID{}})}, Uncertainties: []Uncertainty{}}
			if err := ValidateGIRABuilderResult(GIRABuilderInput{}, result); err == nil {
				t.Fatal("AI completion operation accepted")
			}
		})
	}
}

func TestGIRABuilderRejectsRationaleOutsideGoalTargetChain(t *testing.T) {
	processID, evidenceID, targetA, targetB, goalID, rationaleID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	technique, one := "behavioral_experiment", 1
	in := GIRABuilderInput{
		SelectedProcess:  Process{ID: processID},
		ApprovedEvidence: []Evidence{{ID: evidenceID, Status: "active"}},
		CurrentStrategy: TherapeuticStrategy{
			Targets: []Target{{ID: targetA, ProcessID: processID, ApprovalStatus: "approved", Version: 1}, {ID: targetB, ProcessID: processID, ApprovalStatus: "approved", Version: 1}},
			Goals:   []Goal{{ID: goalID, ProcessID: processID, ApprovalStatus: "approved", Version: 1, TargetIDs: []uuid.UUID{targetB}}},
		},
		ApproachRegistry:  []ApproachDefinition{{Slug: "cbt", Version: 1}},
		TechniqueRegistry: []TechniqueDefinition{{Slug: technique, Version: 1, ApproachSlug: "cbt", ApproachVersion: 1}},
	}
	result := InterpreterResult{Operations: []ProposedOperation{proposed(rationaleID, "create_therapeutic_rationale", nil, CreateTherapeuticRationaleProposal{ProcessID: processID, TargetID: targetA, GoalID: goalID, ApproachSlug: "cbt", ApproachVersion: 1, TechniqueSlug: &technique, TechniqueVersion: &one, Rationale: "Cruce inválido", ExpectedEffect: "Ninguno", GroundingStatus: "grounded", EvidenceIDs: []uuid.UUID{evidenceID}, HypothesisIDs: []uuid.UUID{}})}, Uncertainties: []Uncertainty{}}
	if err := ValidateGIRABuilderResult(in, result); err == nil {
		t.Fatal("rationale accepted for a target not linked to its goal")
	}
}
