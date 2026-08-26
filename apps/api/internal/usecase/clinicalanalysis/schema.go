package clinicalanalysis

func LiveJSONSchema() map[string]any {
	stringEnum := func(values ...string) map[string]any {
		items := make([]any, len(values))
		for i, value := range values {
			items[i] = value
		}
		return map[string]any{"type": "string", "enum": items}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"mode", "node", "hypothesis", "evidence", "now", "caution", "suggested_interventions", "therapist_meta", "risk"},
		"properties": map[string]any{
			"mode": stringEnum("live"),
			"node": map[string]any{"type": "string", "maxLength": 120},
			"hypothesis": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []any{"text", "epistemic_level", "traffic_light"},
				"properties": map[string]any{
					"text":            map[string]any{"type": "string", "maxLength": 300},
					"epistemic_level": stringEnum("fact", "inference", "hypothesis"),
					"traffic_light":   stringEnum("green", "yellow", "red"),
				},
			},
			"evidence": map[string]any{
				"type":     "array",
				"maxItems": 4,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []any{"source_id", "kind", "summary"},
					"properties": map[string]any{
						"source_id": map[string]any{"type": "string"},
						"kind":      stringEnum("fact", "inference"),
						"summary":   map[string]any{"type": "string", "maxLength": 180},
					},
				},
			},
			"now":     stringEnum("listen", "clarify", "reflect", "explore", "confront", "restructure", "resignify", "values", "regulate", "no_intervention"),
			"caution": map[string]any{"type": "string", "maxLength": 240},
			"suggested_interventions": map[string]any{
				"type":     "array",
				"maxItems": 2,
				"items":    map[string]any{"type": "string", "maxLength": 220},
			},
			"therapist_meta": map[string]any{"type": []any{"string", "null"}, "maxLength": 220},
			"risk": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []any{"detected", "category", "requires_human_assessment"},
				"properties": map[string]any{
					"detected":                  map[string]any{"type": "boolean"},
					"category":                  map[string]any{"type": []any{"string", "null"}},
					"requires_human_assessment": map[string]any{"type": "boolean"},
				},
			},
		},
	}
}

func ReviewJSONSchema() map[string]any {
	stringArray := func(max int) map[string]any {
		return map[string]any{"type": "array", "maxItems": max, "items": map[string]any{"type": "string", "maxLength": 500}}
	}
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []any{"mode", "emerging_formulation", "effective_interventions", "weak_or_risky_interventions", "hypothesis_calibration", "patient_responses", "alliance", "therapist_patterns", "next_focus", "evidence", "caution", "risk"},
		"properties": map[string]any{
			"mode":                        map[string]any{"type": "string", "enum": []any{"review"}},
			"emerging_formulation":        map[string]any{"type": "string", "maxLength": 1600},
			"effective_interventions":     stringArray(6),
			"weak_or_risky_interventions": stringArray(6),
			"hypothesis_calibration":      map[string]any{"type": "string", "maxLength": 1000},
			"patient_responses":           map[string]any{"type": "string", "maxLength": 1000},
			"alliance":                    map[string]any{"type": "string", "maxLength": 800},
			"therapist_patterns":          stringArray(6),
			"next_focus":                  stringArray(6),
			"evidence":                    map[string]any{"type": "array", "maxItems": 8, "items": map[string]any{"type": "object", "additionalProperties": false, "required": []any{"source_id", "kind", "summary"}, "properties": map[string]any{"source_id": map[string]any{"type": "string"}, "kind": map[string]any{"type": "string", "enum": []any{"fact", "inference"}}, "summary": map[string]any{"type": "string", "maxLength": 300}}}},
			"caution":                     map[string]any{"type": "string", "maxLength": 600},
			"risk":                        map[string]any{"type": "object", "additionalProperties": false, "required": []any{"detected", "category", "requires_human_assessment"}, "properties": map[string]any{"detected": map[string]any{"type": "boolean"}, "category": map[string]any{"type": []any{"string", "null"}}, "requires_human_assessment": map[string]any{"type": "boolean"}}},
		},
	}
}
