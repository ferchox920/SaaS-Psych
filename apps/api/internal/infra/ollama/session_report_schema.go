package ollama

// sessionReportGenerationSchema avoids llama.cpp's bounded-repetition expansion
// limit (observed HTTP 400 before inference). Only maxLength moves out of the
// sampler grammar; the original schema and typed validator still enforce every
// length before a provider result, AIRun success or report can be accepted.
// Copy recursively: frozen contracts and live/review/longitudinal schemas are
// never mutated by this provider-specific adaptation.
func sessionReportGenerationSchema(schema map[string]any) map[string]any {
	var copyValue func(any) any
	copyValue = func(value any) any {
		switch v := value.(type) {
		case map[string]any:
			out := make(map[string]any, len(v))
			for key, item := range v {
				if key != "maxLength" {
					out[key] = copyValue(item)
				}
			}
			return out
		case []any:
			out := make([]any, len(v))
			for i, item := range v {
				out[i] = copyValue(item)
			}
			return out
		default:
			return value
		}
	}
	return copyValue(schema).(map[string]any)
}
