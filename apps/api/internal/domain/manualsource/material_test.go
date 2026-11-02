package manualsource

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fixture() Import {
	return Import{Kind: "chatgpt_project", ProjectID: "g-p-11111111111111111111111111111111", SourceID: "conversation-stable-id", Content: Content{Title: "Synthetic source", Messages: []Message{{ID: "m1", Role: "user", Text: "Synthetic mixed speaker text"}, {ID: "m2", Role: "assistant", Text: "Synthetic unapproved supervision"}}, Metadata: json.RawMessage(`{"b":2,"a":1}`)}}
}
func TestCanonicalContentIdentity(t *testing.T) {
	v := fixture()
	raw, hash, err := v.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	v.Content.Metadata = json.RawMessage(`{ "a":1, "b":2 }`)
	now := time.Now()
	v.SourceUpdatedAt = &now
	raw2, hash2, err := v.Canonical()
	if err != nil || hash != hash2 || string(raw) != string(raw2) {
		t.Fatal("metadata key order or retrieval timestamp changed identity", err)
	}
	v.Content.Messages[1].Text = "Different supervision"
	_, newHash, err := v.Canonical()
	if err != nil || hash == newHash {
		t.Fatal("changed assistant text did not change revision", err)
	}
}
func TestRejectAmbiguousOrOversizedInput(t *testing.T) {
	cases := []func(*Import){func(v *Import) { v.ProjectID = "https://wrong" }, func(v *Import) { v.Kind = "unknown" }, func(v *Import) { v.Content.Messages[1].ID = "m1" }, func(v *Import) { v.Content.Messages[0].Role = "assumed_patient" }, func(v *Import) { s := "2026-10-29"; v.CandidateDate = &s }, func(v *Import) { v.Content.Text = strings.Repeat("x", 900001) }, func(v *Import) { v.Content.Metadata = json.RawMessage(`{`) }}
	for i, mutate := range cases {
		v := fixture()
		mutate(&v)
		if _, _, err := v.Canonical(); err == nil {
			t.Fatalf("accepted invalid case %d", i)
		}
	}
}
