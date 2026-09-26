package longitudinal

import (
	"encoding/json"
	"strings"
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

func TestPatientCorrectionRemainsPatientDataAndCannotConsolidateHypothesis(t *testing.T) {
	evidenceID, hypothesisID := uuid.New(), uuid.New()
	report := []byte(`{"patient_responses":[{"id":"response-001","response_type":"correction","description":"Fictitious correction."}]}`)
	unsafeEvidence, _ := json.Marshal(CreateEvidenceProposal{SourceItemID: "response-001", EpistemicType: "inference"})
	result := InterpreterResult{Operations: []ProposedOperation{{ID: "e", OperationType: "create_evidence", TargetEntityID: &evidenceID, Proposal: unsafeEvidence}}, Uncertainties: []Uncertainty{}}
	if err := ValidateInterpreterSemantics(report, result); err == nil {
		t.Fatal("patient correction was relabeled as an inference")
	}
	safeEvidence, _ := json.Marshal(CreateEvidenceProposal{SourceItemID: "response-001", EpistemicType: "patient_report"})
	greenHypothesis, _ := json.Marshal(CreateHypothesisProposal{Statement: "Provisional interpretation.", ConfidenceLevel: "green", SupportingEvidenceIDs: []uuid.UUID{evidenceID}, ContradictingEvidenceIDs: []uuid.UUID{}})
	result.Operations = []ProposedOperation{{ID: "e", OperationType: "create_evidence", TargetEntityID: &evidenceID, Proposal: safeEvidence}, {ID: "h", OperationType: "create_hypothesis", TargetEntityID: &hypothesisID, Proposal: greenHypothesis}}
	if err := ValidateInterpreterSemantics(report, result); err == nil {
		t.Fatal("patient correction automatically consolidated a green hypothesis")
	}
	redHypothesis, _ := json.Marshal(CreateHypothesisProposal{Statement: "Provisional interpretation.", ConfidenceLevel: "red", SupportingEvidenceIDs: []uuid.UUID{evidenceID}, ContradictingEvidenceIDs: []uuid.UUID{}})
	result.Operations[1].Proposal = redHypothesis
	if err := ValidateInterpreterSemantics(report, result); err != nil {
		t.Fatalf("provisional correction hypothesis should remain valid: %v", err)
	}
}

func TestInterpreterEvidenceSourceMustBeEligibleReportItem(t *testing.T) {
	evidenceID := uuid.New()
	payload, _ := json.Marshal(CreateEvidenceProposal{SourceItemID: "hypothesis-001", EpistemicType: "inference"})
	result := InterpreterResult{Operations: []ProposedOperation{{ID: "e", OperationType: "create_evidence", TargetEntityID: &evidenceID, Proposal: payload}}}
	report := []byte(`{"facts":[],"relevant_changes":[],"patient_responses":[],"affective_nodes":[],"hypothesis_candidates":[{"id":"hypothesis-001"}]}`)
	if err := ValidateInterpreterSemantics(report, result); err == nil {
		t.Fatal("hypothesis candidate was accepted as factual evidence source")
	}
	payload, _ = json.Marshal(CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})
	result.Operations[0].Proposal = payload
	report = []byte(`{"facts":[{"id":"fact-001"}]}`)
	if err := ValidateInterpreterSemantics(report, result); err != nil {
		t.Fatalf("eligible fact was rejected: %v", err)
	}
}

func TestInterpreterReferencesRejectInventedTargetsAndStaleVersions(t *testing.T) {
	evidenceID, eventID, processID := uuid.New(), uuid.New(), uuid.New()
	state := State{
		ActiveEvidence: []Evidence{{ID: evidenceID, Status: "active", Version: 1}},
		RecentEvents:   []Event{{ID: eventID, ApprovalStatus: "approved", Version: 1}},
		Processes:      []Process{{ID: processID, ApprovalStatus: "approved", Version: 2}},
	}
	payload, _ := json.Marshal(LinkEventProcessProposal{EventID: eventID, ProcessID: processID, EvidenceIDs: []uuid.UUID{evidenceID}})
	version := 2
	valid := InterpreterResult{Operations: []ProposedOperation{{ID: "link", OperationType: "link_event_process", TargetEntityID: &processID, ExpectedEntityVersion: &version, Proposal: payload}}}
	if err := ValidateInterpreterReferences(InterpreterInput{CurrentState: state}, valid); err != nil {
		t.Fatalf("valid state-bound operation rejected: %v", err)
	}
	invented := uuid.New()
	valid.Operations[0].TargetEntityID = &invented
	if err := ValidateInterpreterReferences(InterpreterInput{CurrentState: state}, valid); err == nil {
		t.Fatal("invented process target was accepted")
	}
	valid.Operations[0].TargetEntityID = &processID
	stale := 3
	valid.Operations[0].ExpectedEntityVersion = &stale
	if err := ValidateInterpreterReferences(InterpreterInput{CurrentState: state}, valid); err == nil {
		t.Fatal("stale process version was accepted")
	}
}

func TestInterpreterReferencesAllowOrderedCreateDependencies(t *testing.T) {
	evidenceID, processID, hypothesisID := uuid.New(), uuid.New(), uuid.New()
	evidencePayload, _ := json.Marshal(CreateEvidenceProposal{SourceItemID: "fact-001", EpistemicType: "patient_report"})
	processPayload, _ := json.Marshal(CreateProcessProposal{Title: "Thread", Description: "New thread", ClinicalStatus: "observing", EvidenceIDs: []uuid.UUID{evidenceID}})
	hypothesisPayload, _ := json.Marshal(CreateHypothesisProposal{ProcessID: &processID, Statement: "Competing explanation", ConfidenceLevel: "red", SupportingEvidenceIDs: []uuid.UUID{evidenceID}, ContradictingEvidenceIDs: []uuid.UUID{}})
	result := InterpreterResult{Operations: []ProposedOperation{
		{ID: "e", OperationType: "create_evidence", TargetEntityID: &evidenceID, Proposal: evidencePayload},
		{ID: "p", OperationType: "create_process", TargetEntityID: &processID, Proposal: processPayload},
		{ID: "h", OperationType: "create_hypothesis", TargetEntityID: &hypothesisID, Proposal: hypothesisPayload},
	}}
	if err := ValidateInterpreterReferences(InterpreterInput{CurrentState: State{}}, result); err != nil {
		t.Fatalf("ordered create dependencies rejected: %v", err)
	}
	result.Operations[0], result.Operations[1] = result.Operations[1], result.Operations[0]
	if err := ValidateInterpreterReferences(InterpreterInput{CurrentState: State{}}, result); err == nil {
		t.Fatal("forward evidence reference was accepted")
	}
}

func TestLongitudinalSchemaNarrowsRecognizedReportCandidate(t *testing.T) {
	schema := JSONSchemaForReport([]byte(`{"longitudinal_candidates":[{"operation":"possible_new_process"}]}`))
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"create_process"`) || strings.Contains(text, `"link_event_process"`) || strings.Contains(text, `"create_hypothesis"`) {
		t.Fatalf("unexpected candidate-specific schema: %s", text)
	}
	empty := JSONSchemaForReport([]byte(`{"longitudinal_candidates":[{"operation":"insufficient_evidence"}]}`))
	properties := empty["properties"].(map[string]any)
	operations := properties["operations"].(map[string]any)
	if len(operations["enum"].([]any)) != 1 {
		t.Fatalf("insufficient-evidence candidate must allow zero operations only: %#v", operations)
	}
}
