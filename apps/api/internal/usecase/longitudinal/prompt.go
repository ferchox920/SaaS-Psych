package longitudinal

import (
	"encoding/json"
	"strings"
)

func SystemPromptV1() string {
	return `Eres un intérprete longitudinal clínico local. Devuelve exclusivamente JSON según el esquema.
Mantén separados fuente, evidencia, evento, inferencia e hipótesis. Nunca conviertas una hipótesis repetida en hecho.
Solo facts, relevant_changes, patient_responses y affective_nodes pueden originar create_evidence. Copia el contenido de fuente sin reinterpretarlo.
Una intervención no es evidencia sobre el paciente. Solo puede contextualizar therapeutic_response cuando exista evidencia de una patient_response.
Cada operación interpretativa debe citar evidence_ids. Si no hay evidencia suficiente, no concluyas: usa explore, insufficient_evidence u open_question.
Una corrección del paciente es dato correctivo o contradictorio; nunca la etiquetes automáticamente como resistencia.
Usa UUIDs nuevos y consistentes como target_entity_id para entidades propuestas y reutilízalos en operaciones dependientes.
Para operaciones sobre entidades existentes, target_entity_id debe ser exactamente el UUID de la entidad propietaria en current_longitudinal_state y expected_entity_version debe ser exactamente su version actual, ajustada por operaciones anteriores del mismo resultado.
En link_event_process, target_entity_id y proposal.process_id deben ser el mismo UUID del proceso; proposal.event_id es el UUID del evento. En links de hipótesis, target_entity_id y proposal.hypothesis_id deben coincidir.
No inventes UUIDs para evidencia o entidades existentes: usa únicamente los presentes en current_longitudinal_state o reservados por una operación create anterior.
Usa longitudinal_candidates como guía de qué operación evaluar, no como evidencia. No agregues operaciones auxiliares no solicitadas: si el candidato pide create_process o create_hypothesis, propón esas operaciones directamente con evidencia activa; no agregues link_event_process salvo que el candidato pida explícitamente vincular un evento.
No apruebes ni fusiones nada. Propón operaciones para revisión humana.`
}

func JSONSchemaV1() map[string]any {
	return jsonSchemaV1(nil, false)
}

// JSONSchemaForReport narrows the operation union only when the approved
// SessionReport contains a recognized longitudinal routing candidate. Unknown
// or absent candidates retain the complete V1 contract.
func JSONSchemaForReport(reportJSON []byte) map[string]any {
	allowed, constrained := operationTypesForReport(reportJSON)
	return jsonSchemaV1(allowed, constrained)
}

func operationTypesForReport(reportJSON []byte) (map[string]bool, bool) {
	var report struct {
		Candidates []struct {
			Operation string `json:"operation"`
			Rationale string `json:"rationale"`
		} `json:"longitudinal_candidates"`
	}
	if json.Unmarshal(reportJSON, &report) != nil || len(report.Candidates) == 0 {
		return nil, false
	}
	allowed := map[string]bool{}
	recognized := false
	for _, candidate := range report.Candidates {
		family := []string{}
		switch candidate.Operation {
		case "possible_new_process":
			recognized = true
			family = []string{"create_evidence", "create_process"}
		case "possible_existing_process_update":
			recognized = true
			family = []string{"create_evidence", "create_event", "link_event_process", "update_process", "close_process", "reopen_process"}
		case "possible_hypothesis_update":
			recognized = true
			family = []string{"create_evidence", "create_hypothesis", "update_hypothesis", "link_supporting_evidence", "link_contradicting_evidence", "strengthen_hypothesis", "weaken_hypothesis", "retire_hypothesis"}
		case "insufficient_evidence":
			recognized = true
		}
		explicit := []string{}
		for _, kind := range family {
			if strings.Contains(candidate.Rationale, kind) {
				explicit = append(explicit, kind)
			}
		}
		if len(explicit) > 0 {
			family = explicit
		}
		for _, kind := range family {
			allowed[kind] = true
		}
	}
	return allowed, recognized
}

func jsonSchemaV1(allowed map[string]bool, constrained bool) map[string]any {
	str := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
	}
	obj := func(required []any, properties map[string]any) map[string]any {
		return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
	}
	uuidSchema := map[string]any{"type": "string", "format": "uuid"}
	uuidArray := func(min int) map[string]any {
		return map[string]any{"type": "array", "minItems": min, "maxItems": 20, "items": uuidSchema}
	}
	nullableUUID := map[string]any{"type": []any{"string", "null"}, "format": "uuid"}
	nullableString := map[string]any{"type": []any{"string", "null"}, "maxLength": 120}
	nullableTime := map[string]any{"type": []any{"string", "null"}, "format": "date-time"}
	operation := func(kind string, create bool, proposal map[string]any) map[string]any {
		expected := map[string]any{"type": "integer", "minimum": 1}
		if create {
			expected = map[string]any{"type": "null"}
		}
		targetDescription := "UUID exacto de la entidad existente propietaria de la operación; debe estar en current_longitudinal_state"
		if create {
			targetDescription = "UUID nuevo reservado para la entidad que esta misma operación crea"
		}
		result := obj([]any{"id", "operation_type", "target_entity_id", "expected_entity_version", "proposal"}, map[string]any{
			"id":                      str(80),
			"operation_type":          map[string]any{"type": "string", "enum": []any{kind}},
			"target_entity_id":        map[string]any{"type": "string", "format": "uuid", "description": targetDescription},
			"expected_entity_version": expected,
			"proposal":                proposal,
		})
		result["description"] = "Operación " + kind + "; no elegir este tipo salvo que sea clínicamente necesario y esté respaldado por el input"
		return result
	}
	evidenceIDs := func() map[string]any {
		return obj([]any{"evidence_ids"}, map[string]any{"evidence_ids": uuidArray(1)})
	}
	operations := []any{
		operation("create_evidence", true, obj([]any{"source_item_id", "epistemic_type"}, map[string]any{"source_item_id": str(80), "epistemic_type": map[string]any{"type": "string", "enum": []any{"patient_report", "therapist_observation", "measurement", "documented_fact", "inference"}}})),
		operation("invalidate_evidence", false, evidenceIDs()),
		operation("create_event", true, obj([]any{"event_type", "title", "description", "occurred_at", "observed_at", "evidence_ids"}, map[string]any{"event_type": map[string]any{"type": "string", "enum": []any{"reported_change", "behavior", "decision", "affective_shift", "relationship_event", "symptom_change", "coping_strategy", "therapeutic_response", "safety_event", "context_change", "other"}}, "title": str(200), "description": str(1000), "occurred_at": nullableTime, "observed_at": map[string]any{"type": "string", "format": "date-time"}, "evidence_ids": uuidArray(1), "intervention_source_item_id": nullableString})),
		operation("approve_event", false, evidenceIDs()),
		operation("link_event_process", false, obj([]any{"event_id", "process_id", "evidence_ids"}, map[string]any{"event_id": uuidSchema, "process_id": uuidSchema, "evidence_ids": uuidArray(1)})),
		operation("create_process", true, obj([]any{"title", "description", "clinical_status", "evidence_ids"}, map[string]any{"title": str(200), "description": str(1000), "clinical_status": map[string]any{"type": "string", "enum": []any{"observing", "active", "stabilized"}}, "evidence_ids": uuidArray(1)})),
		operation("update_process", false, obj([]any{"title", "description", "evidence_ids"}, map[string]any{"title": str(200), "description": str(1000), "evidence_ids": uuidArray(1)})),
		operation("close_process", false, obj([]any{"evidence_ids"}, map[string]any{"evidence_ids": uuidArray(1)})),
		operation("reopen_process", false, obj([]any{"evidence_ids", "reopen_status"}, map[string]any{"evidence_ids": uuidArray(1), "reopen_status": map[string]any{"type": "string", "enum": []any{"observing", "active"}}})),
		operation("create_hypothesis", true, obj([]any{"process_id", "statement", "hypothesis_type", "confidence_level", "supporting_evidence_ids", "contradicting_evidence_ids"}, map[string]any{"process_id": nullableUUID, "statement": str(1000), "hypothesis_type": nullableString, "confidence_level": map[string]any{"type": "string", "enum": []any{"red", "yellow", "green"}}, "supporting_evidence_ids": uuidArray(1), "contradicting_evidence_ids": uuidArray(0)})),
		operation("update_hypothesis", false, obj([]any{"process_id", "statement", "hypothesis_type", "evidence_ids"}, map[string]any{"process_id": nullableUUID, "statement": str(1000), "hypothesis_type": nullableString, "evidence_ids": uuidArray(1)})),
		operation("link_supporting_evidence", false, obj([]any{"hypothesis_id", "evidence_id", "evidence_ids"}, map[string]any{"hypothesis_id": uuidSchema, "evidence_id": uuidSchema, "evidence_ids": uuidArray(1)})),
		operation("link_contradicting_evidence", false, obj([]any{"hypothesis_id", "evidence_id", "evidence_ids"}, map[string]any{"hypothesis_id": uuidSchema, "evidence_id": uuidSchema, "evidence_ids": uuidArray(1)})),
		operation("strengthen_hypothesis", false, evidenceIDs()),
		operation("weaken_hypothesis", false, evidenceIDs()),
		operation("retire_hypothesis", false, evidenceIDs()),
	}
	if constrained {
		filtered := make([]any, 0, len(operations))
		for _, rawOperation := range operations {
			op := rawOperation.(map[string]any)
			properties := op["properties"].(map[string]any)
			operationType := properties["operation_type"].(map[string]any)["enum"].([]any)[0].(string)
			if allowed[operationType] {
				filtered = append(filtered, op)
			}
		}
		operations = filtered
	}
	operationArray := map[string]any{"type": "array", "maxItems": 40}
	if len(operations) == 0 {
		operationArray = map[string]any{"type": "array", "enum": []any{[]any{}}}
	} else {
		operationArray["items"] = map[string]any{"oneOf": operations}
	}
	return map[string]any{"type": "object", "additionalProperties": false, "required": []any{"operations", "uncertainties"}, "properties": map[string]any{
		"operations":    operationArray,
		"uncertainties": map[string]any{"type": "array", "maxItems": 20, "items": map[string]any{"type": "object", "additionalProperties": false, "required": []any{"type", "question", "evidence_ids"}, "properties": map[string]any{"type": map[string]any{"type": "string", "enum": []any{"explore", "insufficient_evidence", "open_question"}}, "question": str(600), "evidence_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string", "format": "uuid"}}}}},
	}}
}
