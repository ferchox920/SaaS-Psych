package longitudinal

func GIRASemanticJSONSchema() map[string]any {
	str := map[string]any{"type": "string", "minLength": 1}
	ref := func(pattern string) map[string]any { return map[string]any{"type": "string", "pattern": pattern} }
	refs := func(pattern string) map[string]any {
		return map[string]any{"type": "array", "items": ref(pattern), "uniqueItems": true}
	}
	newRef := func(kind string) map[string]any { return ref("^" + kind + "_[a-z0-9_]{1,48}$") }
	entityRef := func(kind string) map[string]any {
		return ref("^(" + kind + "_|existing_" + kind + "_)[a-z0-9_]{1,48}$")
	}
	nullStr := map[string]any{"type": []string{"string", "null"}}
	nullInt := map[string]any{"type": []string{"integer", "null"}, "minimum": 1}
	object := func(required []string, properties map[string]any) map[string]any {
		return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
	}
	array := func(item map[string]any) map[string]any { return map[string]any{"type": "array", "items": item} }
	target := object([]string{"ref", "title", "description", "target_type", "evidence_refs", "hypothesis_refs", "event_refs"}, map[string]any{
		"ref": newRef("target"), "title": str, "description": str, "target_type": enumSchema("behavioral_pattern", "cognitive_pattern", "emotional_regulation", "experiential_avoidance", "interpersonal_pattern", "meaning_value_conflict", "skill_deficit", "environmental_context", "other"), "evidence_refs": refs("^evidence_[0-9]+$"), "hypothesis_refs": refs("^hypothesis_[0-9]+$"), "event_refs": refs("^event_[0-9]+$"),
	})
	goal := object([]string{"ref", "title", "description", "goal_type", "priority", "target_refs"}, map[string]any{
		"ref": newRef("goal"), "title": str, "description": str, "goal_type": enumSchema("understanding", "behavior_change", "skill_acquisition", "emotional_regulation", "meaning_reconstruction", "relationship_change", "maintenance", "relapse_prevention", "other"), "priority": enumSchema("low", "medium", "high"), "target_refs": refs("^(target_|existing_target_)[a-z0-9_]{1,48}$"),
	})
	indicator := object([]string{"ref", "goal_ref", "description", "indicator_type", "measurement_method", "baseline", "target_value"}, map[string]any{
		"ref": newRef("indicator"), "goal_ref": entityRef("goal"), "description": str, "indicator_type": enumSchema("qualitative", "behavioral", "self_report", "frequency", "scale", "other"), "measurement_method": nullStr, "baseline": nullStr, "target_value": nullStr,
	})
	rationale := object([]string{"ref", "target_ref", "goal_ref", "approach_slug", "approach_version", "technique_slug", "technique_version", "rationale", "expected_effect", "evidence_refs", "hypothesis_refs"}, map[string]any{
		"ref": newRef("rationale"), "target_ref": entityRef("target"), "goal_ref": entityRef("goal"), "approach_slug": str, "approach_version": map[string]any{"type": "integer", "minimum": 1}, "technique_slug": nullStr, "technique_version": nullInt, "rationale": str, "expected_effect": str, "evidence_refs": refs("^evidence_[0-9]+$"), "hypothesis_refs": refs("^hypothesis_[0-9]+$"),
	})
	gira := object([]string{"ref", "title", "summary", "supersedes_gira_ref", "target_refs", "goal_refs", "rationale_refs"}, map[string]any{
		"ref": newRef("gira"), "title": str, "summary": str, "supersedes_gira_ref": map[string]any{"type": []string{"string", "null"}, "pattern": "^existing_gira_[a-z0-9_]{1,48}$"}, "target_refs": refs("^(target_|existing_target_)[a-z0-9_]{1,48}$"), "goal_refs": refs("^(goal_|existing_goal_)[a-z0-9_]{1,48}$"), "rationale_refs": refs("^(rationale_|existing_rationale_)[a-z0-9_]{1,48}$"),
	})
	phase := object([]string{"ref", "gira_ref", "position", "title", "description", "entry_criteria", "exit_criteria", "goal_refs", "rationale_refs", "indicator_refs"}, map[string]any{
		"ref": newRef("phase"), "gira_ref": entityRef("gira"), "position": map[string]any{"type": "integer", "minimum": 1}, "title": str, "description": str, "entry_criteria": nullStr, "exit_criteria": nullStr, "goal_refs": refs("^(goal_|existing_goal_)[a-z0-9_]{1,48}$"), "rationale_refs": refs("^(rationale_|existing_rationale_)[a-z0-9_]{1,48}$"), "indicator_refs": refs("^(indicator_|existing_indicator_)[a-z0-9_]{1,48}$"),
	})
	link := object([]string{"indicator_ref", "source_type", "source_ref", "relation_type", "evidence_refs"}, map[string]any{
		"indicator_ref": entityRef("indicator"), "source_type": enumSchema("evidence", "event"), "source_ref": ref("^(evidence_|event_)[0-9]+$"), "relation_type": enumSchema("supports_progress", "supports_regression", "neutral"), "evidence_refs": refs("^evidence_[0-9]+$"),
	})
	uncertainty := object([]string{"type", "question", "evidence_refs"}, map[string]any{
		"type": enumSchema("explore", "insufficient_evidence", "open_question"), "question": str, "evidence_refs": refs("^evidence_[0-9]+$"),
	})
	return object([]string{"targets", "goals", "indicators", "rationales", "gira", "phases", "indicator_links", "uncertainties"}, map[string]any{
		"targets": array(target), "goals": array(goal), "indicators": array(indicator), "rationales": array(rationale), "gira": map[string]any{"oneOf": []any{gira, map[string]any{"type": "null"}}}, "phases": array(phase), "indicator_links": array(link), "uncertainties": array(uncertainty),
	})
}
