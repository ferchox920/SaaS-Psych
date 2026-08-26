package clinicalsession

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClinicalSessionLifecycle(t *testing.T) {
	now := time.Now().UTC()
	item, err := New(uuid.New(), uuid.New(), nil, uuid.New(), now.Add(-time.Hour), now)
	if err != nil || item.Status != StatusInProgress || item.EndedAt != nil {
		t.Fatalf("invalid initial state: %#v err=%v", item, err)
	}
	if err := item.Complete(now); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if item.Status != StatusCompleted || item.EndedAt == nil {
		t.Fatalf("expected completed: %#v", item)
	}
	if err := item.Void(now.Add(time.Minute)); err == nil {
		t.Fatal("completed session must not reopen or transition")
	}
}

func TestClinicalSessionCanBeVoidedButNotReopened(t *testing.T) {
	now := time.Now().UTC()
	item, err := New(uuid.New(), uuid.New(), nil, uuid.New(), now.Add(-time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := item.Void(now); err != nil {
		t.Fatal(err)
	}
	if item.Status != StatusVoided {
		t.Fatalf("status=%s", item.Status)
	}
	if err := item.Complete(now.Add(time.Minute)); err == nil {
		t.Fatal("voided session must remain terminal")
	}
}

func TestClinicalSessionRejectsInvalidTimesAndIDs(t *testing.T) {
	now := time.Now().UTC()
	if _, err := New(uuid.Nil, uuid.New(), nil, uuid.New(), now, now); err == nil {
		t.Fatal("expected tenant validation")
	}
	item, _ := New(uuid.New(), uuid.New(), nil, uuid.New(), now, now)
	if err := item.Complete(now.Add(-time.Minute)); err == nil {
		t.Fatal("expected ended_at validation")
	}
}
