// Command clinical-project-schema prints the frozen semantic strategy schema
// in OpenAPI 3.0 form for the manual project import contract. It performs no I/O
// other than stdout and never invokes a provider.
package main

import (
	"encoding/json"
	"os"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

func openAPI(v any) any {
	switch x := v.(type) {
	case map[string]any:
		if types, ok := x["type"].([]string); ok {
			x["type"] = types[0]
			x["nullable"] = true
		}
		if alternatives, ok := x["oneOf"].([]any); ok && len(alternatives) == 2 {
			if null, ok := alternatives[1].(map[string]any); ok && null["type"] == "null" {
				first := alternatives[0].(map[string]any)
				first["nullable"] = true
				return openAPI(first)
			}
		}
		for k, value := range x {
			x[k] = openAPI(value)
		}
		return x
	case []any:
		for i, value := range x {
			x[i] = openAPI(value)
		}
		return x
	default:
		return v
	}
}
func main() {
	if err := json.NewEncoder(os.Stdout).Encode(openAPI(longitudinal.GIRASemanticJSONSchema())); err != nil {
		os.Exit(1)
	}
}
