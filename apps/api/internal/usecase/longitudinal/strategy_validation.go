package longitudinal

import (
	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

func ValidateStrategyProposal(kind string, raw []byte) (bool, error) {
	invalid := func() (bool, error) { return true, domainerrors.NewValidation("invalid " + kind + " proposal") }
	validTargetType := func(v string) bool {
		return oneOf(v, "behavioral_pattern", "cognitive_pattern", "emotional_regulation", "experiential_avoidance", "interpersonal_pattern", "meaning_value_conflict", "skill_deficit", "environmental_context", "other")
	}
	validGoalType := func(v string) bool {
		return oneOf(v, "understanding", "behavior_change", "skill_acquisition", "emotional_regulation", "meaning_reconstruction", "relationship_change", "maintenance", "relapse_prevention", "other")
	}
	switch kind {
	case "create_target":
		var p CreateTargetProposal
		if decodeStrict(raw, &p) != nil || p.ProcessID == uuid.Nil || blank(p.Title) || blank(p.Description) || !validTargetType(p.TargetType) || len(p.EvidenceIDs)+len(p.HypothesisIDs)+len(p.EventIDs) == 0 {
			return invalid()
		}
	case "update_target":
		var p UpdateTargetProposal
		if decodeStrict(raw, &p) != nil || blank(p.Title) || blank(p.Description) || !validTargetType(p.TargetType) || len(p.EvidenceIDs)+len(p.HypothesisIDs)+len(p.EventIDs) == 0 {
			return invalid()
		}
	case "resolve_target", "retire_target":
		var p TransitionTargetProposal
		if decodeStrict(raw, &p) != nil || len(p.EvidenceIDs) == 0 {
			return invalid()
		}
	case "create_goal":
		var p CreateGoalProposal
		if decodeStrict(raw, &p) != nil || p.ProcessID == uuid.Nil || blank(p.Title) || blank(p.Description) || !validGoalType(p.GoalType) || !oneOf(p.Priority, "low", "medium", "high") || len(p.TargetIDs) == 0 {
			return invalid()
		}
	case "update_goal":
		var p UpdateGoalProposal
		if decodeStrict(raw, &p) != nil || blank(p.Title) || blank(p.Description) || !validGoalType(p.GoalType) || !oneOf(p.Priority, "low", "medium", "high") || len(p.TargetIDs) == 0 {
			return invalid()
		}
	case "activate_goal", "pause_goal", "achieve_goal", "abandon_goal":
		var p TransitionGoalProposal
		if decodeStrict(raw, &p) != nil || len(p.EvidenceIDs)+len(p.IndicatorIDs) == 0 {
			return invalid()
		}
	case "create_goal_indicator":
		var p CreateGoalIndicatorProposal
		if decodeStrict(raw, &p) != nil || p.GoalID == uuid.Nil || blank(p.Description) || !oneOf(p.IndicatorType, "qualitative", "behavioral", "self_report", "frequency", "scale", "other") {
			return invalid()
		}
	case "update_goal_indicator":
		var p UpdateGoalIndicatorProposal
		if decodeStrict(raw, &p) != nil || blank(p.Description) || !oneOf(p.IndicatorType, "qualitative", "behavioral", "self_report", "frequency", "scale", "other") || !oneOf(p.Status, "active", "inactive", "retired") {
			return invalid()
		}
	case "link_indicator_evidence", "link_indicator_event":
		var p LinkIndicatorSourceProposal
		if decodeStrict(raw, &p) != nil || p.IndicatorID == uuid.Nil || p.SourceID == uuid.Nil || !oneOf(p.RelationType, "supports_progress", "supports_regression", "neutral") || len(p.EvidenceIDs) == 0 {
			return invalid()
		}
	case "create_therapeutic_rationale", "update_therapeutic_rationale":
		var p CreateTherapeuticRationaleProposal
		if decodeStrict(raw, &p) != nil || p.ProcessID == uuid.Nil || p.TargetID == uuid.Nil || p.GoalID == uuid.Nil || blank(p.ApproachSlug) || p.ApproachVersion < 1 || blank(p.Rationale) || blank(p.ExpectedEffect) || !oneOf(p.GroundingStatus, "grounded", "insufficient_evidence", "exploration_needed") || (p.GroundingStatus == "grounded" && len(p.EvidenceIDs)+len(p.HypothesisIDs) == 0) || ((p.TechniqueSlug == nil) != (p.TechniqueVersion == nil)) {
			return invalid()
		}
	case "create_gira":
		var p CreateGIRAProposal
		if decodeStrict(raw, &p) != nil || p.ProcessID == uuid.Nil || p.GIRAVersion < 1 || blank(p.Title) || blank(p.Summary) || len(p.TargetIDs) == 0 || len(p.GoalIDs) == 0 || len(p.RationaleIDs) == 0 {
			return invalid()
		}
	case "supersede_gira", "activate_gira", "pause_gira", "complete_gira":
		var p TransitionGIRAProposal
		if decodeStrict(raw, &p) != nil || len(p.EvidenceIDs) == 0 {
			return invalid()
		}
	case "create_gira_phase":
		var p CreateGIRAPhaseProposal
		if decodeStrict(raw, &p) != nil || p.GIRAID == uuid.Nil || p.Position < 1 || blank(p.Title) || blank(p.Description) || len(p.GoalIDs) == 0 {
			return invalid()
		}
	case "update_gira_phase":
		var p UpdateGIRAPhaseProposal
		if decodeStrict(raw, &p) != nil || p.Position < 1 || blank(p.Title) || blank(p.Description) || len(p.GoalIDs) == 0 {
			return invalid()
		}
	case "activate_gira_phase", "complete_gira_phase", "pause_gira_phase":
		var p TransitionGIRAPhaseProposal
		if decodeStrict(raw, &p) != nil || len(p.EvidenceIDs) == 0 {
			return invalid()
		}
	default:
		return false, nil
	}
	return true, nil
}
