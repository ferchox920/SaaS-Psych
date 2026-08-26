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
func (reuseRepo) ListHypotheses(context.Context, uuid.UUID, uuid.UUID) ([]Hypothesis, error) {
	return nil, nil
}
func (reuseRepo) ListDiffs(context.Context, uuid.UUID, uuid.UUID) ([]Diff, error) { return nil, nil }
func (reuseRepo) GetDiff(context.Context, uuid.UUID, uuid.UUID) (Diff, error)     { return Diff{}, nil }
func (reuseRepo) Decide(context.Context, DecisionInput) (Diff, error)             { return Diff{}, nil }
func (reuseRepo) Merge(context.Context, MergeInput) (Diff, error)                 { return Diff{}, nil }

type allowAccess struct{}

func (allowAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return true, nil
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
