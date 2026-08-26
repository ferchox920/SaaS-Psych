package longitudinal

func SystemPromptV1() string {
	return `Eres un intérprete longitudinal clínico local. Devuelve exclusivamente JSON según el esquema.
Mantén separados fuente, evidencia, evento, inferencia e hipótesis. Nunca conviertas una hipótesis repetida en hecho.
Solo facts, relevant_changes, patient_responses y affective_nodes pueden originar create_evidence. Copia el contenido de fuente sin reinterpretarlo.
Una intervención no es evidencia sobre el paciente. Solo puede contextualizar therapeutic_response cuando exista evidencia de una patient_response.
Cada operación interpretativa debe citar evidence_ids. Si no hay evidencia suficiente, no concluyas: usa explore, insufficient_evidence u open_question.
Una corrección del paciente es dato correctivo o contradictorio; nunca la etiquetes automáticamente como resistencia.
Usa UUIDs nuevos y consistentes como target_entity_id para entidades propuestas y reutilízalos en operaciones dependientes.
No apruebes ni fusiones nada. Propón operaciones para revisión humana.`
}

func JSONSchemaV1() map[string]any {
	str := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
	}
	return map[string]any{"type": "object", "additionalProperties": false, "required": []any{"operations", "uncertainties"}, "properties": map[string]any{
		"operations":    map[string]any{"type": "array", "maxItems": 40, "items": map[string]any{"type": "object", "additionalProperties": false, "required": []any{"id", "operation_type", "target_entity_id", "expected_entity_version", "proposal"}, "properties": map[string]any{"id": str(80), "operation_type": map[string]any{"type": "string", "enum": []any{"create_evidence", "invalidate_evidence", "create_event", "approve_event", "link_event_process", "create_process", "update_process", "close_process", "reopen_process", "create_hypothesis", "update_hypothesis", "link_supporting_evidence", "link_contradicting_evidence", "strengthen_hypothesis", "weaken_hypothesis", "retire_hypothesis"}}, "target_entity_id": map[string]any{"type": "string", "format": "uuid"}, "expected_entity_version": map[string]any{"type": []any{"integer", "null"}, "minimum": 1}, "proposal": map[string]any{"type": "object"}}}},
		"uncertainties": map[string]any{"type": "array", "maxItems": 20, "items": map[string]any{"type": "object", "additionalProperties": false, "required": []any{"type", "question", "evidence_ids"}, "properties": map[string]any{"type": map[string]any{"type": "string", "enum": []any{"explore", "insufficient_evidence", "open_question"}}, "question": str(600), "evidence_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string", "format": "uuid"}}}}},
	}}
}
