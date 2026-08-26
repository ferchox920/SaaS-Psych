package longitudinal

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOperationValidationPreservesEpistemicBoundary(t *testing.T) {
	valid, _ := json.Marshal(CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "documented_fact"})
	if err := ValidateProposal("create_evidence", valid); err != nil {
		t.Fatal(err)
	}
	hypothesis, _ := json.Marshal(CreateEvidenceProposal{SourceItemID: "hypothesis-001", EpistemicType: "hypothesis"})
	if err := ValidateProposal("create_evidence", hypothesis); err == nil {
		t.Fatal("hypothesis must never be an evidence epistemic type")
	}
	event, _ := json.Marshal(CreateEventProposal{EventType: "reported_change", Title: "Cambio", Description: "Descripción", ObservedAt: time.Now(), EvidenceIDs: []uuid.UUID{uuid.New()}})
	if err := ValidateProposal("create_event", event); err != nil {
		t.Fatal(err)
	}
	withoutEvidence, _ := json.Marshal(CreateEventProposal{EventType: "reported_change", Title: "Cambio", Description: "Descripción", ObservedAt: time.Now(), EvidenceIDs: []uuid.UUID{}})
	if err := ValidateProposal("create_event", withoutEvidence); err == nil {
		t.Fatal("interpretative operations require evidence")
	}
}

func TestInterpreterRejectsUnknownFieldsAndUnsupportedUncertainty(t *testing.T) {
	raw := []byte(`{"operations":[],"uncertainties":[],"approved":true}`)
	if _, err := DecodeInterpreterResult(raw); err == nil {
		t.Fatal("unknown fields must be rejected")
	}
	raw = []byte(`{"operations":[],"uncertainties":[{"type":"conclusion","question":"x","evidence_ids":[]}]}`)
	if _, err := DecodeInterpreterResult(raw); err == nil {
		t.Fatal("unsupported uncertainty must be rejected")
	}
}
