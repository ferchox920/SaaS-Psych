package clinicalmemory

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type memoryRepoStub struct {
	clientID           uuid.UUID
	suggestionClientID uuid.UUID
	disposition        string
	listCalled         bool
}

func (r *memoryRepoStub) CreateSuggestion(context.Context, CreateSuggestionInput) (Suggestion, error) {
	return Suggestion{}, nil
}
func (r *memoryRepoStub) ClientIDForAppointment(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return r.clientID, nil
}
func (r *memoryRepoStub) SuggestionProvenance(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, string, error) {
	return r.suggestionClientID, r.disposition, nil
}
func (r *memoryRepoStub) ListSuggestions(context.Context, uuid.UUID, uuid.UUID) ([]Suggestion, error) {
	r.listCalled = true
	return nil, nil
}
func (r *memoryRepoStub) DecideSuggestion(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, string) (Suggestion, error) {
	return Suggestion{}, nil
}
func (r *memoryRepoStub) CreateSnapshot(_ context.Context, tenantID, clientID, actorID uuid.UUID, summary string, anchors []Anchor) (Snapshot, error) {
	return Snapshot{TenantID: tenantID, ClientID: clientID, CreatedByUserID: actorID, ApprovedSummary: summary, Anchors: anchors}, nil
}
func (r *memoryRepoStub) ListSnapshots(context.Context, uuid.UUID, uuid.UUID) ([]Snapshot, error) {
	return nil, nil
}
func (r *memoryRepoStub) GetApprovedContext(context.Context, uuid.UUID, uuid.UUID) (ApprovedContext, error) {
	return ApprovedContext{}, nil
}
func (r *memoryRepoStub) ApproveSnapshot(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (Snapshot, error) {
	return Snapshot{}, nil
}

type accessStub struct{ allowed bool }

func (a accessStub) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return a.allowed, nil
}

func TestListSuggestionsAuthorizesBeforeReadingContent(t *testing.T) {
	repo := &memoryRepoStub{clientID: uuid.New()}
	service := NewService(repo, accessStub{allowed: false})
	_, err := service.ListSuggestions(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected access denial")
	}
	if repo.listCalled {
		t.Fatal("suggestion content must not be read before authorization")
	}
}

func TestCreateSnapshotRejectsDiscardedSuggestionAsAnchor(t *testing.T) {
	clientID := uuid.New()
	suggestionID := uuid.New()
	repo := &memoryRepoStub{suggestionClientID: clientID, disposition: "discarded"}
	service := NewService(repo, accessStub{allowed: true})
	green := "green"
	_, err := service.CreateSnapshot(context.Background(), uuid.New(), clientID, uuid.New(), "Fictitious summary", []Anchor{
		{SourceID: "fact_1", Kind: "fact", Summary: "Fictitious fact"},
		{SourceID: "hypothesis_1", Kind: "hypothesis", Summary: "Fictitious hypothesis", TrafficLight: &green, SourceSuggestionID: &suggestionID},
	})
	if err == nil {
		t.Fatal("discarded suggestion must never become consolidated context")
	}
}
