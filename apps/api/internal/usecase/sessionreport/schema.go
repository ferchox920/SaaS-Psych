package sessionreport

func JSONSchemaV1() map[string]any {
	str := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
	}
	obj := func(required []any, properties map[string]any) map[string]any {
		return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
	}
	arr := func(max int, item map[string]any) map[string]any {
		return map[string]any{"type": "array", "maxItems": max, "items": item}
	}
	refArray := arr(8, map[string]any{"type": "string", "pattern": "^(fact|change|response|affect)-[0-9]{3,6}$"})
	refArray["uniqueItems"] = true
	id := func(prefix string) map[string]any {
		return map[string]any{"type": "string", "pattern": "^" + prefix + "-[0-9]{3,6}$"}
	}
	fact := obj([]any{"id", "statement", "category"}, map[string]any{"id": id("fact"), "statement": str(500), "category": str(80)})
	change := obj([]any{"id", "description", "category"}, map[string]any{"id": id("change"), "description": str(500), "category": str(80)})
	intervention := obj([]any{"id", "type", "description"}, map[string]any{"id": id("intervention"), "type": str(80), "description": str(500)})
	response := obj([]any{"id", "response_type", "description"}, map[string]any{"id": id("response"), "response_type": str(80), "description": str(500)})
	affect := obj([]any{"id", "description"}, map[string]any{"id": id("affect"), "description": str(500)})
	inference := obj([]any{"id", "statement", "evidence_refs"}, map[string]any{"id": id("inference"), "statement": str(600), "evidence_refs": refArray})
	hypothesis := obj([]any{"id", "statement", "traffic_light", "evidence_refs"}, map[string]any{"id": id("hypothesis"), "statement": str(600), "traffic_light": map[string]any{"type": "string", "enum": []any{"green", "yellow", "red"}}, "evidence_refs": refArray})
	safety := obj([]any{"id", "description", "category", "requires_human_assessment"}, map[string]any{"id": id("safety"), "description": str(500), "category": str(80), "requires_human_assessment": map[string]any{"type": "boolean"}})
	question := obj([]any{"id", "question"}, map[string]any{"id": id("question"), "question": str(500)})
	longitudinal := obj([]any{"id", "operation", "rationale"}, map[string]any{"id": id("longitudinal"), "operation": map[string]any{"type": "string", "enum": []any{"possible_existing_process_update", "possible_new_process", "possible_hypothesis_update", "possible_goal_update", "possible_formulation_update"}}, "rationale": str(500)})
	return obj([]any{"schema_version", "summary", "facts", "relevant_changes", "interventions", "patient_responses", "affective_nodes", "inference_candidates", "hypothesis_candidates", "safety_signals", "open_questions", "longitudinal_candidates"}, map[string]any{"schema_version": map[string]any{"type": "string", "enum": []any{SchemaVersion}}, "summary": str(2000), "facts": arr(20, fact), "relevant_changes": arr(12, change), "interventions": arr(12, intervention), "patient_responses": arr(12, response), "affective_nodes": arr(12, affect), "inference_candidates": arr(12, inference), "hypothesis_candidates": arr(12, hypothesis), "safety_signals": arr(8, safety), "open_questions": arr(12, question), "longitudinal_candidates": arr(12, longitudinal)})
}
