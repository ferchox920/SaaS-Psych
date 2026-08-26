package clinicalairun

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type runRepoStub struct{ run Run }

func (r *runRepoStub) Start(_ context.Context, run Run) (Run, error) { r.run = run; return run, nil }
func (r *runRepoStub) Finish(_ context.Context, _, _ uuid.UUID, status string, outputHash, errorCode *string, at time.Time) (Run, error) {
	r.run.Status = status
	r.run.OutputHash = outputHash
	r.run.ErrorCode = errorCode
	r.run.CompletedAt = &at
	return r.run, nil
}
func (r *runRepoStub) ListSources(context.Context, uuid.UUID, uuid.UUID) ([]Source, error) {
	return append([]Source(nil), r.run.Sources...), nil
}
func startInput() StartInput {
	return StartInput{TenantID: uuid.New(), ClientID: uuid.New(), CreatedByUserID: uuid.New(), Provider: "ollama", Model: "fixture", Operation: "review_session", PromptName: "clinical-review", PromptVersion: "clinical-review-v1", Parameters: map[string]any{"temperature": 0.1}, Input: map[string]any{"text": "PHI is hashed only"}, Context: map[string]any{"version": 1}}
}
func TestRunLifecycleStoresHashesAndSuccessfulOutput(t *testing.T) {
	repo := &runRepoStub{}
	service := NewService(repo)
	run, err := service.Start(context.Background(), startInput())
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != StatusRunning || len(run.InputHash) != 64 || len(run.ContextHash) != 64 {
		t.Fatalf("run=%#v", run)
	}
	finished, err := service.Succeed(context.Background(), run.TenantID, run.ID, map[string]any{"result": "structured"})
	if err != nil || finished.Status != StatusSucceeded || finished.OutputHash == nil || finished.ErrorCode != nil {
		t.Fatalf("run=%#v err=%v", finished, err)
	}
}
func TestRunFailureAndCancellationHaveNoOutputHash(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		repo := &runRepoStub{}
		service := NewService(repo)
		run, _ := service.Start(context.Background(), startInput())
		finished, err := service.Fail(context.Background(), run.TenantID, run.ID, "provider_unavailable", cancelled)
		if err != nil || finished.OutputHash != nil || finished.ErrorCode == nil {
			t.Fatalf("run=%#v err=%v", finished, err)
		}
		want := StatusFailed
		if cancelled {
			want = StatusCancelled
		}
		if finished.Status != want {
			t.Fatalf("status=%s want=%s", finished.Status, want)
		}
	}
}
func TestHashChangesWithInputWithoutRetainingInput(t *testing.T) {
	if Hash(map[string]string{"text": "a"}) == Hash(map[string]string{"text": "b"}) {
		t.Fatal("input hashes must differ")
	}
}
func TestSourcesAreCanonicalAndBuildProvenanceHasSafeFallback(t *testing.T) {
	v1, v2 := 1, 2
	a, b := uuid.New(), uuid.New()
	input := startInput()
	input.Sources = []Source{{SourceType: SourceSessionReport, SourceID: b, SourceVersion: &v1}, {SourceType: SourceFormulationSnapshot, SourceID: a, SourceVersion: &v2}}
	run, err := NewService(&runRepoStub{}).Start(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if run.AppVersion != "development" || run.BuildRevision != "unknown" {
		t.Fatalf("build provenance=%#v", run)
	}
	reversed := append([]Source(nil), input.Sources...)
	reversed[0], reversed[1] = reversed[1], reversed[0]
	canonical, err := CanonicalSources(input.TenantID, reversed)
	if err != nil {
		t.Fatal(err)
	}
	if HashSources(run.Sources) != HashSources(canonical) {
		t.Fatal("same logical source set must have deterministic hash")
	}
	canonical[0].SourceVersion = &v1
	if HashSources(run.Sources) == HashSources(canonical) {
		t.Fatal("source version change must alter context hash")
	}
}
func TestBuildProvenanceDoesNotChangeClinicalContentHashes(t *testing.T) {
	input := startInput()
	one, err := NewService(&runRepoStub{}).WithBuildInfo("one", "aaa").Start(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewService(&runRepoStub{}).WithBuildInfo("two", "bbb").Start(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if one.InputHash != two.InputHash || one.ContextHash != two.ContextHash {
		t.Fatal("build metadata must not affect clinical content hashes")
	}
}
