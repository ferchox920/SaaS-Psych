package longitudinal

import "strings"

func GIRASystemPromptV1() string {
	return strings.TrimSpace(`1. ROLE
Propón una estrategia terapéutica clínica para revisión humana. No apruebas ni modificas estado.

2. INPUT CONTRACT
Usa solo los refs, approach slugs/versiones y technique slugs/versiones presentes en la entrada. Los refs existing_* identifican estado aprobado. No inventes UUIDs.

3. ALLOWED PROPOSALS
Puedes proponer targets, goals, indicators, rationales, una GIRA con fases, enlaces de evidencia/evento a indicadores y uncertainties. Para progreso o regresión sobre estrategia existente, prioriza indicator_links. No propongas transiciones de estado.

4. CLINICAL RULES
- Cada target nuevo cita al menos un evidence_ref, event_ref o hypothesis_ref autorizado.
- Cada goal es observable/operacional y refiere target_refs válidos; cada goal nuevo tiene al menos un indicator.
- Un indicador puede ser cualitativo; no inventes números.
- Cada approach elegido tiene rationale propio que conecta target_ref, goal_ref, efecto esperado y grounding autorizado.
- Una technique debe pertenecer al approach/version exacto de los candidatos.
- Múltiples approaches solo cuando cumplen funciones clínicas distintas; uno o ninguno es válido.
- Propón la cantidad mínima de objetos necesaria: no dupliques targets/goals equivalentes ni agregues fases redundantes.
- Para progreso/regresión usa indicator_links; nunca declares goal logrado.
- Para una nueva versión GIRA usa supersedes_gira_ref existente; nunca reescribas la anterior.

5. OUTPUT CONTRACT
Devuelve exclusivamente JSON según el schema. Para objetos nuevos crea refs breves únicos como target_1, goal_1, indicator_1, rationale_1, gira_1 y phase_1. El backend asignará UUIDs, versiones y orden de operaciones. Incluye todos los arrays aunque estén vacíos y usa null en campos nullable sin valor.

6. INVALID CONDITIONS
Si falta grounding o el mecanismo no está sustentado, omite la entidad/rationale inseguro y agrega insufficient_evidence o explore. No completes Goals, GIRAs ni fases. Conserva formulation y therapeutic_strategy separados.`)
}

var giraBuilderOperations = []string{
	"create_target", "update_target", "resolve_target", "retire_target",
	"create_goal", "update_goal", "activate_goal", "pause_goal", "abandon_goal",
	"create_goal_indicator", "update_goal_indicator", "link_indicator_evidence", "link_indicator_event",
	"create_therapeutic_rationale", "update_therapeutic_rationale",
	"create_gira", "supersede_gira", "activate_gira", "pause_gira",
	"create_gira_phase", "update_gira_phase", "activate_gira_phase", "pause_gira_phase",
}

func GIRAJSONSchema() map[string]any {
	variants := make([]any, 0, len(giraBuilderOperations))
	for _, op := range giraBuilderOperations {
		create := strings.HasPrefix(op, "create_")
		expected := map[string]any{"type": "integer", "minimum": 1}
		if create {
			expected = map[string]any{"type": "null"}
		}
		variants = append(variants, map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"id", "operation_type", "target_entity_id", "expected_entity_version", "proposal"},
			"properties": map[string]any{
				"id":                      map[string]any{"type": "string", "minLength": 1},
				"operation_type":          map[string]any{"const": op},
				"target_entity_id":        uuidSchema(),
				"expected_entity_version": expected,
				"proposal":                giraProposalSchema(op),
			},
		})
	}
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"operations", "uncertainties"},
		"properties": map[string]any{
			"operations": map[string]any{"type": "array", "items": map[string]any{"oneOf": variants}},
			"uncertainties": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "additionalProperties": false, "required": []string{"type", "question", "evidence_ids"},
				"properties": map[string]any{"type": map[string]any{"type": "string", "enum": []string{"explore", "insufficient_evidence", "open_question"}}, "question": map[string]any{"type": "string", "minLength": 1}, "evidence_ids": uuidArraySchema()},
			}},
		},
	}
}

func giraProposalSchema(op string) map[string]any {
	p := map[string]any{}
	required := []string{}
	add := func(name string, schema any) { p[name] = schema; required = append(required, name) }
	str := func() map[string]any { return map[string]any{"type": "string", "minLength": 1} }
	nullStr := map[string]any{"type": []string{"string", "null"}}
	nullUUID := map[string]any{"type": []string{"string", "null"}, "format": "uuid"}
	nullInt := map[string]any{"type": []string{"integer", "null"}, "minimum": 1}
	switch op {
	case "create_target":
		add("process_id", uuidSchema())
		add("title", str())
		add("description", str())
		add("target_type", enumSchema("behavioral_pattern", "cognitive_pattern", "emotional_regulation", "experiential_avoidance", "interpersonal_pattern", "meaning_value_conflict", "skill_deficit", "environmental_context", "other"))
		add("evidence_ids", uuidArraySchema())
		add("hypothesis_ids", uuidArraySchema())
		add("event_ids", uuidArraySchema())
	case "update_target":
		add("title", str())
		add("description", str())
		add("target_type", enumSchema("behavioral_pattern", "cognitive_pattern", "emotional_regulation", "experiential_avoidance", "interpersonal_pattern", "meaning_value_conflict", "skill_deficit", "environmental_context", "other"))
		add("evidence_ids", uuidArraySchema())
		add("hypothesis_ids", uuidArraySchema())
		add("event_ids", uuidArraySchema())
	case "resolve_target", "retire_target", "activate_goal", "pause_goal", "abandon_goal", "supersede_gira", "activate_gira", "pause_gira", "activate_gira_phase", "pause_gira_phase":
		add("evidence_ids", uuidArraySchema())
	case "create_goal":
		add("process_id", uuidSchema())
		add("title", str())
		add("description", str())
		add("goal_type", enumSchema("understanding", "behavior_change", "skill_acquisition", "emotional_regulation", "meaning_reconstruction", "relationship_change", "maintenance", "relapse_prevention", "other"))
		add("priority", enumSchema("low", "medium", "high"))
		add("target_ids", uuidArraySchema())
	case "update_goal":
		add("title", str())
		add("description", str())
		add("goal_type", enumSchema("understanding", "behavior_change", "skill_acquisition", "emotional_regulation", "meaning_reconstruction", "relationship_change", "maintenance", "relapse_prevention", "other"))
		add("priority", enumSchema("low", "medium", "high"))
		add("target_ids", uuidArraySchema())
	case "create_goal_indicator":
		add("goal_id", uuidSchema())
		add("description", str())
		add("indicator_type", enumSchema("qualitative", "behavioral", "self_report", "frequency", "scale", "other"))
		add("measurement_method", nullStr)
		add("baseline", nullStr)
		add("target_value", nullStr)
	case "update_goal_indicator":
		add("description", str())
		add("indicator_type", enumSchema("qualitative", "behavioral", "self_report", "frequency", "scale", "other"))
		add("measurement_method", nullStr)
		add("baseline", nullStr)
		add("target_value", nullStr)
		add("status", enumSchema("active", "inactive", "retired"))
	case "link_indicator_evidence", "link_indicator_event":
		add("indicator_id", uuidSchema())
		add("source_id", uuidSchema())
		add("relation_type", enumSchema("supports_progress", "supports_regression", "neutral"))
		add("evidence_ids", uuidArraySchema())
	case "create_therapeutic_rationale", "update_therapeutic_rationale":
		add("process_id", uuidSchema())
		add("target_id", uuidSchema())
		add("goal_id", uuidSchema())
		add("approach_slug", str())
		add("approach_version", map[string]any{"type": "integer", "minimum": 1})
		add("technique_slug", nullStr)
		add("technique_version", nullInt)
		add("rationale", str())
		add("expected_effect", str())
		add("grounding_status", enumSchema("grounded", "insufficient_evidence", "exploration_needed"))
		add("evidence_ids", uuidArraySchema())
		add("hypothesis_ids", uuidArraySchema())
	case "create_gira":
		add("process_id", uuidSchema())
		add("gira_version", map[string]any{"type": "integer", "minimum": 1})
		add("title", str())
		add("summary", str())
		add("supersedes_gira_id", nullUUID)
		add("target_ids", uuidArraySchema())
		add("goal_ids", uuidArraySchema())
		add("rationale_ids", uuidArraySchema())
	case "create_gira_phase":
		add("gira_id", uuidSchema())
		add("position", map[string]any{"type": "integer", "minimum": 1})
		add("title", str())
		add("description", str())
		add("entry_criteria", nullStr)
		add("exit_criteria", nullStr)
		add("goal_ids", uuidArraySchema())
		add("rationale_ids", uuidArraySchema())
		add("indicator_ids", uuidArraySchema())
	case "update_gira_phase":
		add("position", map[string]any{"type": "integer", "minimum": 1})
		add("title", str())
		add("description", str())
		add("entry_criteria", nullStr)
		add("exit_criteria", nullStr)
		add("goal_ids", uuidArraySchema())
		add("rationale_ids", uuidArraySchema())
		add("indicator_ids", uuidArraySchema())
	}
	return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": p}
}

func uuidSchema() map[string]any { return map[string]any{"type": "string", "format": "uuid"} }
func uuidArraySchema() map[string]any {
	return map[string]any{"type": "array", "items": uuidSchema(), "uniqueItems": true}
}
func enumSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}
