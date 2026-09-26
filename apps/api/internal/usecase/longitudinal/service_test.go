package longitudinal

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type reuseRepo struct {
	session SessionAnalysis
	state   State
	diff    Diff
}

func TestMergeRejectsNilSystemActor(t *testing.T) {
	service := NewService(reuseRepo{}, allowAccess{}, nil, nil, nil, nil, "ollama", "fixture", nil)
	_, err := service.Merge(context.Background(), MergeInput{TenantID: uuid.New(), DiffID: uuid.New(), ActorID: uuid.Nil, ExpectedDiffRevision: 1})
	if !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
}

func (r reuseRepo) SessionAnalysis(context.Context, uuid.UUID, uuid.UUID) (SessionAnalysis, error) {
	return r.session, nil
}
func (r reuseRepo) FindOpenDiff(context.Context, uuid.UUID, uuid.UUID, int64) (Diff, error) {
	return r.diff, nil
}
func (r reuseRepo) State(context.Context, uuid.UUID, uuid.UUID) (State, error) { return r.state, nil }
func (reuseRepo) CreateDiff(context.Context, CreateDiffInput) (Diff, error)    { panic("must not create") }
func (reuseRepo) ListEvidence(context.Context, uuid.UUID, uuid.UUID) ([]Evidence, error) {
	return nil, nil
}
func (reuseRepo) ListEvents(context.Context, uuid.UUID, uuid.UUID) ([]Event, error) { return nil, nil }
func (reuseRepo) ListProcesses(context.Context, uuid.UUID, uuid.UUID) ([]Process, error) {
	return nil, nil
}
func (reuseRepo) GetProcess(context.Context, uuid.UUID, uuid.UUID) (Process, error) {
	return Process{}, nil
}
func (reuseRepo) ListHypotheses(context.Context, uuid.UUID, uuid.UUID) ([]Hypothesis, error) {
	return nil, nil
}
func (reuseRepo) ListTargets(context.Context, uuid.UUID, uuid.UUID) ([]Target, error) {
	return nil, nil
}
func (reuseRepo) ListGoals(context.Context, uuid.UUID, uuid.UUID) ([]Goal, error) {
	return nil, nil
}
func (reuseRepo) ListGIRAs(context.Context, uuid.UUID, uuid.UUID) ([]GIRA, error) {
	return nil, nil
}
func (reuseRepo) GetGIRA(context.Context, uuid.UUID, uuid.UUID) (GIRA, error) {
	return GIRA{}, nil
}
func (reuseRepo) ListApproaches(context.Context) ([]ApproachDefinition, error) {
	return nil, nil
}
func (reuseRepo) ListTechniques(context.Context) ([]TechniqueDefinition, error) {
	return nil, nil
}
func (reuseRepo) GetStrategyHistory(context.Context, uuid.UUID, uuid.UUID, string, uuid.UUID) (StrategyHistory, error) {
	return StrategyHistory{}, nil
}
func (reuseRepo) ListDiffs(context.Context, uuid.UUID, uuid.UUID) ([]Diff, error) { return nil, nil }
func (r reuseRepo) GetDiff(context.Context, uuid.UUID, uuid.UUID) (Diff, error)   { return r.diff, nil }
func (reuseRepo) Decide(context.Context, DecisionInput) (Diff, error)             { return Diff{}, nil }
func (reuseRepo) Merge(context.Context, MergeInput) (Diff, error)                 { return Diff{}, nil }
func (reuseRepo) GetProcessHistory(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (ProcessHistory, error) {
	return ProcessHistory{}, nil
}
func (reuseRepo) GetHypothesisHistory(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (HypothesisHistory, error) {
	return HypothesisHistory{}, nil
}

type allowAccess struct{}

func (allowAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return true, nil
}

type denyAccess struct{}

func (denyAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return false, nil
}

func TestLongitudinalCARAndCAWDenials(t *testing.T) {
	tenant, actor, client, session, report, diffID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := reuseRepo{session: SessionAnalysis{SessionID: session, ClientID: client, Status: "completed", ReportID: report, ReportVersion: 1}, state: State{ClientID: client}, diff: Diff{ID: diffID, ClientID: client, Revision: 1, Operations: []Operation{{ID: uuid.New(), OperationType: "create_evidence"}}}}
	service := NewService(repo, denyAccess{}, nil, nil, nil, nil, "ollama", "fixture", nil)
	if _, err := service.State(context.Background(), tenant, client, actor); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("CA-R State err=%v", err)
	}
	if _, err := service.Analyze(context.Background(), tenant, session, actor); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("CA-W Analyze err=%v", err)
	}
	if _, err := service.Decide(context.Background(), DecisionInput{TenantID: tenant, DiffID: diffID, OperationID: repo.diff.Operations[0].ID, ActorID: actor, ExpectedDiffRevision: 1, Decision: "approved"}); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("CA-W Decide err=%v", err)
	}
	if _, err := service.Merge(context.Background(), MergeInput{TenantID: tenant, DiffID: diffID, ActorID: actor, ExpectedDiffRevision: 1}); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("CA-W Merge err=%v", err)
	}
}

func TestAnalyzeReusesOpenDiffBeforeInvokingAI(t *testing.T) {
	tenant, actor, client, session, report, diffID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := reuseRepo{session: SessionAnalysis{SessionID: session, ClientID: client, Status: "completed", ReportID: report, ReportVersion: 1}, state: State{ClientID: client, StateVersion: 7}, diff: Diff{ID: diffID, ClientID: client, BaseStateVersion: 7, Status: "pending_review"}}
	service := NewService(repo, allowAccess{}, nil, nil, nil, nil, "ollama", "fixture", nil)
	out, err := service.Analyze(context.Background(), tenant, session, actor)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Reused || out.Diff.ID != diffID {
		t.Fatalf("out=%#v", out)
	}
}

func TestAnalyzeIncompleteSessionReturnsValidation(t *testing.T) {
	tenant, actor, client, session := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repo := reuseRepo{session: SessionAnalysis{SessionID: session, ClientID: client, Status: "in_progress"}}
	service := NewService(repo, allowAccess{}, nil, nil, nil, nil, "ollama", "fixture", nil)
	if _, err := service.Analyze(context.Background(), tenant, session, actor); !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("incomplete session err=%v", err)
	}
}
