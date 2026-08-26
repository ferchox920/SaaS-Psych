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

var operationTypes = map[string]struct{}{"create_evidence": {}, "invalidate_evidence": {}, "create_event": {}, "approve_event": {}, "link_event_process": {}, "create_process": {}, "update_process": {}, "close_process": {}, "reopen_process": {}, "create_hypothesis": {}, "update_hypothesis": {}, "link_supporting_evidence": {}, "link_contradicting_evidence": {}, "strengthen_hypothesis": {}, "weaken_hypothesis": {}, "retire_hypothesis": {}}

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
		isCreate := op.OperationType == "create_evidence" || op.OperationType == "create_event" || op.OperationType == "create_process" || op.OperationType == "create_hypothesis"
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
