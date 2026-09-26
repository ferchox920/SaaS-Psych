package remotegira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
)

type pipelineRepo struct {
	input   longitudinal.GIRABuilderInput
	created *longitudinal.CreateDiffInput
}

func (r *pipelineRepo) GetProcess(context.Context, uuid.UUID, uuid.UUID) (longitudinal.Process, error) {
	return r.input.SelectedProcess, nil
}
func (r *pipelineRepo) State(context.Context, uuid.UUID, uuid.UUID) (longitudinal.State, error) {
	return longitudinal.State{ClientID: r.input.SelectedProcess.ClientID, StateVersion: r.input.StateVersion, ActiveEvidence: r.input.ApprovedEvidence, RecentEvents: r.input.ApprovedEvents, Processes: []longitudinal.Process{r.input.SelectedProcess}}, nil
}
func (r *pipelineRepo) ListApproaches(context.Context) ([]longitudinal.ApproachDefinition, error) {
	return r.input.ApproachRegistry, nil
}
func (r *pipelineRepo) ListTechniques(context.Context) ([]longitudinal.TechniqueDefinition, error) {
	return r.input.TechniqueRegistry, nil
}
func (r *pipelineRepo) CreateDiff(_ context.Context, in longitudinal.CreateDiffInput) (longitudinal.Diff, error) {
	r.created = &in
	return longitudinal.Diff{ID: uuid.New(), TenantID: in.TenantID, ClientID: in.ClientID, Status: "pending_review", BaseStateVersion: in.BaseStateVersion, Revision: 1, Operations: in.Operations, Uncertainties: in.Uncertainties}, nil
}
func (*pipelineRepo) SessionAnalysis(context.Context, uuid.UUID, uuid.UUID) (longitudinal.SessionAnalysis, error) {
	return longitudinal.SessionAnalysis{}, domainerrors.ErrNotFound
}
func (*pipelineRepo) FindOpenDiff(context.Context, uuid.UUID, uuid.UUID, int64) (longitudinal.Diff, error) {
	return longitudinal.Diff{}, domainerrors.ErrNotFound
}
func (*pipelineRepo) ListEvidence(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Evidence, error) {
	return nil, nil
}
func (*pipelineRepo) ListEvents(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Event, error) {
	return nil, nil
}
func (*pipelineRepo) ListProcesses(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Process, error) {
	return nil, nil
}
func (*pipelineRepo) ListHypotheses(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Hypothesis, error) {
	return nil, nil
}
func (*pipelineRepo) ListTargets(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Target, error) {
	return nil, nil
}
func (*pipelineRepo) ListGoals(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Goal, error) {
	return nil, nil
}
func (*pipelineRepo) ListGIRAs(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.GIRA, error) {
	return nil, nil
}
func (*pipelineRepo) GetGIRA(context.Context, uuid.UUID, uuid.UUID) (longitudinal.GIRA, error) {
	return longitudinal.GIRA{}, domainerrors.ErrNotFound
}
func (*pipelineRepo) GetStrategyHistory(context.Context, uuid.UUID, uuid.UUID, string, uuid.UUID) (longitudinal.StrategyHistory, error) {
	return longitudinal.StrategyHistory{}, nil
}
func (*pipelineRepo) ListDiffs(context.Context, uuid.UUID, uuid.UUID) ([]longitudinal.Diff, error) {
	return nil, nil
}
func (*pipelineRepo) GetDiff(context.Context, uuid.UUID, uuid.UUID) (longitudinal.Diff, error) {
	return longitudinal.Diff{}, domainerrors.ErrNotFound
}
func (*pipelineRepo) Decide(context.Context, longitudinal.DecisionInput) (longitudinal.Diff, error) {
	return longitudinal.Diff{}, nil
}
func (*pipelineRepo) Merge(context.Context, longitudinal.MergeInput) (longitudinal.Diff, error) {
	return longitudinal.Diff{}, nil
}
func (*pipelineRepo) GetProcessHistory(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (longitudinal.ProcessHistory, error) {
	return longitudinal.ProcessHistory{}, nil
}
func (*pipelineRepo) GetHypothesisHistory(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (longitudinal.HypothesisHistory, error) {
	return longitudinal.HypothesisHistory{}, nil
}

type pipelineAccess struct{}

func (pipelineAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return true, nil
}

type pipelineRunRepo struct{ run clinicalairun.Run }

func (r *pipelineRunRepo) Start(_ context.Context, run clinicalairun.Run) (clinicalairun.Run, error) {
	r.run = run
	return run, nil
}
func (r *pipelineRunRepo) Finish(_ context.Context, _, _ uuid.UUID, status string, output, code *string, at time.Time) (clinicalairun.Run, error) {
	r.run.Status, r.run.OutputHash, r.run.ErrorCode, r.run.CompletedAt = status, output, code, &at
	return r.run, nil
}
func (r *pipelineRunRepo) ListSources(context.Context, uuid.UUID, uuid.UUID) ([]clinicalairun.Source, error) {
	return r.run.Sources, nil
}

type auditRecord struct {
	action   string
	metadata map[string]any
}
type pipelineAuditor struct{ records []auditRecord }

func (a *pipelineAuditor) RecordDomainEvent(_ context.Context, _, _ uuid.UUID, action, _ string, _ *uuid.UUID, metadata map[string]any) error {
	a.records = append(a.records, auditRecord{action: action, metadata: metadata})
	return nil
}

func pipelineInput() longitudinal.GIRABuilderInput {
	tenant, client, processID, evidenceID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	strategy := longitudinal.TherapeuticStrategy{Targets: []longitudinal.Target{}, Goals: []longitudinal.Goal{}, Rationales: []longitudinal.TherapeuticRationale{}, GIRAs: []longitudinal.GIRA{}}
	return longitudinal.GIRABuilderInput{SchemaVersion: longitudinal.GIRAPromptVersion, SelectedProcess: longitudinal.Process{ID: processID, TenantID: tenant, ClientID: client, Title: "Paciente María Prueba", Description: "María Prueba evita conversaciones; maria@example.test", ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1, Hypotheses: []longitudinal.Hypothesis{}, TherapeuticStrategy: &strategy}, ApprovedEvidence: []longitudinal.Evidence{{ID: evidenceID, TenantID: tenant, ClientID: client, EpistemicType: "patient_report", Statement: "Paciente María Prueba evita y obtiene alivio", Status: "active", Version: 1}}, ApprovedEvents: []longitudinal.Event{}, ApprovedHypotheses: []longitudinal.Hypothesis{}, CurrentStrategy: strategy, ApproachRegistry: []longitudinal.ApproachDefinition{{ID: uuid.New(), Slug: "cbt", Version: 1, Status: "active", TargetDomains: []string{"behavioral_pattern"}, CoreMechanisms: []string{"contrastar predicciones"}}}, TechniqueRegistry: []longitudinal.TechniqueDefinition{{ID: uuid.New(), Slug: "behavioral_experiment", Version: 1, ApproachSlug: "cbt", ApproachVersion: 1, Status: "active", TargetDomains: []string{"behavioral_pattern"}, Mechanism: "contrastar predicciones"}}, StateVersion: 4}
}

func newPipelineService(t *testing.T, provider longitudinal.GIRABuilderProvider) (*longitudinal.Service, *pipelineRepo, *pipelineRunRepo, *pipelineAuditor) {
	t.Helper()
	input := pipelineInput()
	repo := &pipelineRepo{input: input}
	runs := &pipelineRunRepo{}
	auditor := &pipelineAuditor{}
	service := longitudinal.NewService(repo, pipelineAccess{}, nil, clinicalairun.NewService(runs), nil, auditor, "ollama", "local-model", map[string]any{}).WithConfiguredGIRABuilder(provider, "openai", "gpt-test", map[string]any{"data_control_status": "unknown"})
	return service, repo, runs, auditor
}

func TestMockedOpenAIPipelineCreatesOnlyPendingDiffAndSafeAudit(t *testing.T) {
	_, valid := validProviderFixture()
	provider, server := newFakeProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("x-request-id", "req_pipeline_1")
		_, _ = w.Write(openAIResponseBody(string(valid)))
	}, time.Second)
	defer server.Close()
	service, repo, runs, auditor := newPipelineService(t, provider)
	before, _ := json.Marshal(repo.input.SelectedProcess.TherapeuticStrategy)
	out, err := service.BuildGIRA(context.Background(), repo.input.SelectedProcess.TenantID, repo.input.SelectedProcess.ID, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(repo.input.SelectedProcess.TherapeuticStrategy)
	if out.Diff.Status != "pending_review" || repo.created == nil || !bytes.Equal(before, after) {
		t.Fatalf("pipeline mutated strategy or omitted pending diff: status=%s created=%t", out.Diff.Status, repo.created != nil)
	}
	if runs.run.Provider != "openai" || runs.run.Model != "gpt-test" || len(runs.run.InputHash) != 64 || len(runs.run.ContextHash) != 64 {
		t.Fatalf("AIRun provenance mismatch: %+v", runs.run)
	}
	if repo.created.RunMetadata["remote_request_id"] != "req_pipeline_1" || repo.created.RunMetadata["minimization_policy_version"] != longitudinal.GIRAPrivacyPolicyVersion {
		t.Fatalf("safe remote metadata missing: %+v", repo.created.RunMetadata)
	}
	actions := []string{}
	for _, record := range auditor.records {
		actions = append(actions, record.action)
		raw, _ := json.Marshal(record.metadata)
		for _, forbidden := range []string{"María Prueba", "maria@example.test", "evita conversaciones", "contrastar predicciones"} {
			if strings.Contains(string(raw), forbidden) {
				t.Fatalf("audit metadata leaked clinical content %q: %s", forbidden, raw)
			}
		}
	}
	if !reflect.DeepEqual(actions, []string{"remote_gira.request_prepared", "remote_gira.request_sent", "remote_gira.response_received"}) {
		t.Fatalf("unexpected outbound audit sequence: %v", actions)
	}
}

type errorProvider struct{ err error }

func (p errorProvider) BuildGIRA(context.Context, string, longitudinal.GIRAProviderRequest, func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	return clinicalanalysis.ProviderOutput{}, p.err
}

func TestServiceRemoteFailureIsTerminalWithNoDiffAndNoFallback(t *testing.T) {
	for _, test := range []struct {
		name      string
		err       error
		cancelled bool
	}{
		{name: "network", err: &Error{Code: "provider_unavailable"}},
		{name: "rate_limit", err: &Error{Code: "rate_limited", StatusCode: 429}},
		{name: "server", err: &Error{Code: "provider_unavailable", StatusCode: 503}},
		{name: "repair", err: &Error{Code: "invalid_model_output"}},
		{name: "timeout", err: &Error{Code: "timeout", cause: context.DeadlineExceeded}, cancelled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, repo, runs, auditor := newPipelineService(t, errorProvider{err: test.err})
			_, err := service.BuildGIRA(context.Background(), repo.input.SelectedProcess.TenantID, repo.input.SelectedProcess.ID, uuid.New())
			if !errors.Is(err, test.err) || repo.created != nil {
				t.Fatalf("unsafe failure outcome created=%t err=%v", repo.created != nil, err)
			}
			expectedStatus := clinicalairun.StatusFailed
			if test.cancelled {
				expectedStatus = clinicalairun.StatusCancelled
			}
			if runs.run.Status != expectedStatus || runs.run.ErrorCode == nil || *runs.run.ErrorCode != ErrorCode(test.err) {
				t.Fatalf("AIRun not terminal: %+v", runs.run)
			}
			if len(auditor.records) != 3 || auditor.records[2].action != "remote_gira.request_failed" {
				t.Fatalf("failure audit missing: %+v", auditor.records)
			}
		})
	}
}
