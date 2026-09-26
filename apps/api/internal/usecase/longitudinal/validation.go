package longitudinal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

var operationTypes = map[string]struct{}{
	"create_evidence": {}, "invalidate_evidence": {}, "create_event": {}, "approve_event": {}, "link_event_process": {}, "create_process": {}, "update_process": {}, "close_process": {}, "reopen_process": {}, "create_hypothesis": {}, "update_hypothesis": {}, "link_supporting_evidence": {}, "link_contradicting_evidence": {}, "strengthen_hypothesis": {}, "weaken_hypothesis": {}, "retire_hypothesis": {},
	"create_target": {}, "update_target": {}, "resolve_target": {}, "retire_target": {}, "create_goal": {}, "update_goal": {}, "activate_goal": {}, "pause_goal": {}, "achieve_goal": {}, "abandon_goal": {}, "create_goal_indicator": {}, "update_goal_indicator": {}, "link_indicator_evidence": {}, "link_indicator_event": {}, "create_therapeutic_rationale": {}, "update_therapeutic_rationale": {}, "create_gira": {}, "supersede_gira": {}, "activate_gira": {}, "pause_gira": {}, "complete_gira": {}, "create_gira_phase": {}, "update_gira_phase": {}, "activate_gira_phase": {}, "complete_gira_phase": {}, "pause_gira_phase": {},
}

func DecodeInterpreterResult(raw []byte) (InterpreterResult, error) {
	var out InterpreterResult
	if err := decodeStrict(raw, &out); err != nil {
		return out, err
	}
	if out.Operations == nil || out.Uncertainties == nil {
		return out, domainerrors.NewValidation("operations and uncertainties are required")
	}
	seen := map[string]bool{}
	for i := range out.Operations {
		op := &out.Operations[i]
		if strings.TrimSpace(op.ID) == "" || seen[op.ID] {
			return out, domainerrors.NewValidation("operation ids must be non-empty and unique")
		}
		seen[op.ID] = true
		if _, ok := operationTypes[op.OperationType]; !ok {
			return out, domainerrors.NewValidation("invalid longitudinal operation type")
		}
		if op.TargetEntityID == nil || *op.TargetEntityID == uuid.Nil {
			return out, domainerrors.NewValidation("target_entity_id must be a reserved uuid")
		}
		isCreate := strings.HasPrefix(op.OperationType, "create_")
		if (isCreate && op.ExpectedEntityVersion != nil) || (!isCreate && op.ExpectedEntityVersion == nil) {
			return out, domainerrors.NewValidation("expected_entity_version must be null only for create operations")
		}
		if err := ValidateProposal(op.OperationType, op.Proposal); err != nil {
			return out, err
		}
	}
	for _, u := range out.Uncertainties {
		if u.Type != "explore" && u.Type != "insufficient_evidence" && u.Type != "open_question" {
			return out, domainerrors.NewValidation("invalid uncertainty type")
		}
		if strings.TrimSpace(u.Question) == "" {
			return out, domainerrors.NewValidation("uncertainty question is required")
		}
	}
	return out, nil
}

// ValidateInterpreterSemantics enforces source-aware epistemic boundaries that
// cannot be expressed by the provider JSON schema alone. Patient responses are
// first-person data; corrections and discrepancies may support a provisional
// hypothesis, but cannot be relabeled as inference/fact or consolidated green.
func ValidateInterpreterSemantics(reportJSON []byte, result InterpreterResult) error {
	var report struct {
		Facts []struct {
			ID string `json:"id"`
		} `json:"facts"`
		RelevantChanges []struct {
			ID string `json:"id"`
		} `json:"relevant_changes"`
		PatientResponses []struct {
			ID           string `json:"id"`
			ResponseType string `json:"response_type"`
		} `json:"patient_responses"`
		AffectiveNodes []struct {
			ID string `json:"id"`
		} `json:"affective_nodes"`
	}
	if err := decodeStrict(reportJSON, &report); err != nil {
		// SessionReport has more fields than the semantic view. Decode it normally
		// while retaining strict validation for model-owned operation payloads.
		if json.Unmarshal(reportJSON, &report) != nil {
			return domainerrors.NewValidation("invalid session report for longitudinal semantics")
		}
	}
	patientResponses := map[string]bool{}
	corrections := map[string]bool{}
	eligibleSourceItems := map[string]bool{}
	for _, item := range report.Facts {
		eligibleSourceItems[item.ID] = true
	}
	for _, item := range report.RelevantChanges {
		eligibleSourceItems[item.ID] = true
	}
	for _, response := range report.PatientResponses {
		eligibleSourceItems[response.ID] = true
		patientResponses[response.ID] = true
		kind := strings.ToLower(strings.TrimSpace(response.ResponseType))
		corrections[response.ID] = kind == "correction" || kind == "discrepancy"
	}
	for _, item := range report.AffectiveNodes {
		eligibleSourceItems[item.ID] = true
	}
	correctionEvidence := map[uuid.UUID]bool{}
	for _, op := range result.Operations {
		if op.OperationType != "create_evidence" || op.TargetEntityID == nil {
			continue
		}
		var proposal CreateEvidenceProposal
		if err := decodeStrict(op.Proposal, &proposal); err != nil {
			return err
		}
		if !eligibleSourceItems[proposal.SourceItemID] {
			return domainerrors.NewValidation("create_evidence must reference an eligible session report item")
		}
		if patientResponses[proposal.SourceItemID] && proposal.EpistemicType != "patient_report" {
			return domainerrors.NewValidation("patient response evidence must remain patient_report data")
		}
		if corrections[proposal.SourceItemID] {
			correctionEvidence[*op.TargetEntityID] = true
		}
	}
	for _, op := range result.Operations {
		if op.OperationType != "create_hypothesis" {
			continue
		}
		var proposal CreateHypothesisProposal
		if err := decodeStrict(op.Proposal, &proposal); err != nil {
			return err
		}
		if proposal.ConfidenceLevel == "green" {
			for _, evidenceID := range proposal.SupportingEvidenceIDs {
				if correctionEvidence[evidenceID] {
					return domainerrors.NewValidation("patient correction cannot automatically consolidate a hypothesis")
				}
			}
		}
	}
	return nil
}

// ValidateInterpreterReferences simulates the ordered operation list against
// the exact longitudinal snapshot sent to the model. It rejects otherwise
// schema-valid proposals that could never merge (invented ids, stale versions,
// wrong target types, or forward references).
func ValidateInterpreterReferences(input InterpreterInput, result InterpreterResult) error {
	type entity struct {
		kind    string
		version int
		active  bool
	}
	entities := map[uuid.UUID]entity{}
	addEvidence := func(e Evidence) {
		entities[e.ID] = entity{kind: "evidence", version: e.Version, active: e.Status == "active"}
	}
	addEvent := func(e Event) {
		entities[e.ID] = entity{kind: "event", version: e.Version, active: e.ApprovalStatus == "approved"}
		for _, evidence := range e.Evidence {
			addEvidence(evidence)
		}
	}
	addHypothesis := func(h Hypothesis) {
		entities[h.ID] = entity{kind: "hypothesis", version: h.Version, active: h.ApprovalStatus == "approved"}
		for _, evidence := range h.SupportingEvidence {
			addEvidence(evidence)
		}
		for _, evidence := range h.ContradictingEvidence {
			addEvidence(evidence)
		}
	}
	for _, evidence := range input.CurrentState.ActiveEvidence {
		addEvidence(evidence)
	}
	for _, event := range input.CurrentState.RecentEvents {
		addEvent(event)
	}
	for _, process := range input.CurrentState.Processes {
		entities[process.ID] = entity{kind: "process", version: process.Version, active: process.ApprovalStatus == "approved"}
		for _, event := range process.Events {
			addEvent(event)
		}
		for _, hypothesis := range process.Hypotheses {
			addHypothesis(hypothesis)
		}
	}
	for _, hypothesis := range input.CurrentState.UnassignedHypotheses {
		addHypothesis(hypothesis)
	}

	require := func(id uuid.UUID, kind string) (entity, error) {
		e, ok := entities[id]
		if !ok || e.kind != kind || !e.active {
			return entity{}, domainerrors.NewValidation(kind + " reference is not present in current longitudinal state")
		}
		return e, nil
	}
	requireEvidence := func(ids []uuid.UUID) error {
		for _, id := range ids {
			if _, err := require(id, "evidence"); err != nil {
				return err
			}
		}
		return nil
	}
	mutate := func(op ProposedOperation, kind string) error {
		if op.TargetEntityID == nil || op.ExpectedEntityVersion == nil {
			return domainerrors.NewValidation("invalid longitudinal operation target")
		}
		e, err := require(*op.TargetEntityID, kind)
		if err != nil {
			return err
		}
		if e.version != *op.ExpectedEntityVersion {
			return domainerrors.NewValidation("expected_entity_version does not match current longitudinal state")
		}
		e.version++
		entities[*op.TargetEntityID] = e
		return nil
	}
	create := func(op ProposedOperation, kind string) error {
		if op.TargetEntityID == nil {
			return domainerrors.NewValidation("create operation requires a reserved target uuid")
		}
		if _, exists := entities[*op.TargetEntityID]; exists {
			return domainerrors.NewValidation("reserved target uuid collides with longitudinal state or an earlier operation")
		}
		entities[*op.TargetEntityID] = entity{kind: kind, version: 1, active: true}
		return nil
	}

	for _, op := range result.Operations {
		switch op.OperationType {
		case "create_evidence":
			if err := create(op, "evidence"); err != nil {
				return err
			}
		case "invalidate_evidence":
			var p InvalidateEvidenceProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if err := requireEvidence(p.EvidenceIDs); err != nil {
				return err
			}
			if err := mutate(op, "evidence"); err != nil {
				return err
			}
			e := entities[*op.TargetEntityID]
			e.active = false
			entities[*op.TargetEntityID] = e
		case "create_event":
			var p CreateEventProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if err := requireEvidence(p.EvidenceIDs); err != nil {
				return err
			}
			if err := create(op, "event"); err != nil {
				return err
			}
		case "approve_event":
			var p ApproveEventProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if err := requireEvidence(p.EvidenceIDs); err != nil {
				return err
			}
			if err := mutate(op, "event"); err != nil {
				return err
			}
		case "create_process":
			var p CreateProcessProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if err := requireEvidence(p.EvidenceIDs); err != nil {
				return err
			}
			if err := create(op, "process"); err != nil {
				return err
			}
		case "update_process", "close_process", "reopen_process":
			var evidenceIDs []uuid.UUID
			if op.OperationType == "update_process" {
				var p UpdateProcessProposal
				if err := decodeStrict(op.Proposal, &p); err != nil {
					return err
				}
				evidenceIDs = p.EvidenceIDs
			} else {
				var p TransitionProcessProposal
				if err := decodeStrict(op.Proposal, &p); err != nil {
					return err
				}
				evidenceIDs = p.EvidenceIDs
			}
			if err := requireEvidence(evidenceIDs); err != nil {
				return err
			}
			if err := mutate(op, "process"); err != nil {
				return err
			}
		case "link_event_process":
			var p LinkEventProcessProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if op.TargetEntityID == nil || p.ProcessID != *op.TargetEntityID {
				return domainerrors.NewValidation("target_entity_id must identify linked process")
			}
			if _, err := require(p.EventID, "event"); err != nil {
				return err
			}
			if err := requireEvidence(p.EvidenceIDs); err != nil {
				return err
			}
			if err := mutate(op, "process"); err != nil {
				return err
			}
		case "create_hypothesis":
			var p CreateHypothesisProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if p.ProcessID != nil {
				if _, err := require(*p.ProcessID, "process"); err != nil {
					return err
				}
			}
			allEvidence := append(append([]uuid.UUID{}, p.SupportingEvidenceIDs...), p.ContradictingEvidenceIDs...)
			if err := requireEvidence(allEvidence); err != nil {
				return err
			}
			if err := create(op, "hypothesis"); err != nil {
				return err
			}
		case "update_hypothesis":
			var p UpdateHypothesisProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if p.ProcessID != nil {
				if _, err := require(*p.ProcessID, "process"); err != nil {
					return err
				}
			}
			if err := requireEvidence(p.EvidenceIDs); err != nil {
				return err
			}
			if err := mutate(op, "hypothesis"); err != nil {
				return err
			}
		case "link_supporting_evidence", "link_contradicting_evidence":
			var p LinkHypothesisEvidenceProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if op.TargetEntityID == nil || p.HypothesisID != *op.TargetEntityID {
				return domainerrors.NewValidation("target_entity_id must identify linked hypothesis")
			}
			if err := requireEvidence(append(append([]uuid.UUID{}, p.EvidenceIDs...), p.EvidenceID)); err != nil {
				return err
			}
			if err := mutate(op, "hypothesis"); err != nil {
				return err
			}
		case "strengthen_hypothesis", "weaken_hypothesis", "retire_hypothesis":
			var p TransitionHypothesisProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if err := requireEvidence(p.EvidenceIDs); err != nil {
				return err
			}
			if err := mutate(op, "hypothesis"); err != nil {
				return err
			}
		default:
			return domainerrors.NewValidation("invalid longitudinal operation type")
		}
	}
	return nil
}

func ValidateProposal(kind string, raw []byte) error {
	if len(raw) == 0 {
		return domainerrors.NewValidation("operation proposal is required")
	}
	switch kind {
	case "create_evidence":
		var p CreateEvidenceProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.SourceItemID == "" || !oneOf(p.EpistemicType, "patient_report", "therapist_observation", "measurement", "documented_fact", "inference") {
			return domainerrors.NewValidation("invalid create_evidence proposal")
		}
	case "create_event":
		var p CreateEventProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if !oneOf(p.EventType, "reported_change", "behavior", "decision", "affective_shift", "relationship_event", "symptom_change", "coping_strategy", "therapeutic_response", "safety_event", "context_change", "other") || blank(p.Title) || blank(p.Description) || p.ObservedAt.IsZero() || len(p.EvidenceIDs) == 0 {
			return domainerrors.NewValidation("invalid create_event proposal")
		}
		if p.InterventionSourceItemID != nil && p.EventType != "therapeutic_response" {
			return domainerrors.NewValidation("intervention context is only valid for therapeutic_response")
		}
	case "create_process":
		var p CreateProcessProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if blank(p.Title) || blank(p.Description) || !oneOf(p.ClinicalStatus, "observing", "active", "stabilized") || len(p.EvidenceIDs) == 0 {
			return domainerrors.NewValidation("invalid create_process proposal")
		}
	case "update_process":
		var p UpdateProcessProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if blank(p.Title) || blank(p.Description) || len(p.EvidenceIDs) == 0 {
			return domainerrors.NewValidation("invalid update_process proposal")
		}
	case "close_process":
		var p TransitionProcessProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if len(p.EvidenceIDs) == 0 || p.ReopenStatus != "" {
			return domainerrors.NewValidation("invalid close_process proposal")
		}
	case "reopen_process":
		var p TransitionProcessProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if len(p.EvidenceIDs) == 0 || !oneOf(p.ReopenStatus, "observing", "active") {
			return domainerrors.NewValidation("invalid reopen_process proposal")
		}
	case "link_event_process":
		var p LinkEventProcessProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.EventID == uuid.Nil || p.ProcessID == uuid.Nil || len(p.EvidenceIDs) == 0 {
			return domainerrors.NewValidation("invalid link_event_process proposal")
		}
	case "create_hypothesis":
		var p CreateHypothesisProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if blank(p.Statement) || !oneOf(p.ConfidenceLevel, "red", "yellow", "green") || len(p.SupportingEvidenceIDs) == 0 {
			return domainerrors.NewValidation("invalid create_hypothesis proposal")
		}
	case "update_hypothesis":
		var p UpdateHypothesisProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if blank(p.Statement) || len(p.EvidenceIDs) == 0 {
			return domainerrors.NewValidation("invalid update_hypothesis proposal")
		}
	case "link_supporting_evidence", "link_contradicting_evidence":
		var p LinkHypothesisEvidenceProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.HypothesisID == uuid.Nil || p.EvidenceID == uuid.Nil || len(p.EvidenceIDs) == 0 {
			return domainerrors.NewValidation("invalid hypothesis evidence link proposal")
		}
	case "strengthen_hypothesis", "weaken_hypothesis", "retire_hypothesis":
		var p TransitionHypothesisProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if len(p.EvidenceIDs) == 0 {
			return domainerrors.NewValidation("hypothesis transition requires evidence")
		}
	case "invalidate_evidence":
		var p InvalidateEvidenceProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if len(p.EvidenceIDs) == 0 {
			return domainerrors.NewValidation("evidence invalidation requires evidence")
		}
	case "approve_event":
		var p ApproveEventProposal
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if len(p.EvidenceIDs) == 0 {
			return domainerrors.NewValidation("event approval requires evidence")
		}
	default:
		if recognized, err := ValidateStrategyProposal(kind, raw); recognized {
			return err
		}
		return domainerrors.NewValidation("invalid longitudinal operation type")
	}
	return nil
}

func decodeStrict(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return domainerrors.NewValidation("invalid operation payload: " + err.Error())
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domainerrors.NewValidation("operation payload contains trailing data")
	}
	return nil
}
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func blank(v string) bool { return strings.TrimSpace(v) == "" }
