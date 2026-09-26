package sessionreport

import (
	"encoding/json"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"unicode/utf8"
)

// String bounds are part of the canonical report contract, independent of the
// provider's sampling grammar. This also protects human edits and all imports.
func validateReportStringBounds(report ReportV1) error {
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	var check func(any, map[string]any) error
	check = func(v any, schema map[string]any) error {
		switch item := v.(type) {
		case string:
			if max, ok := schema["maxLength"].(int); ok && utf8.RuneCountInString(item) > max {
				return domainerrors.NewValidation("session report string exceeds its schema length limit")
			}
		case map[string]any:
			properties, _ := schema["properties"].(map[string]any)
			for key, child := range item {
				if spec, ok := properties[key].(map[string]any); ok {
					if err := check(child, spec); err != nil {
						return err
					}
				}
			}
		case []any:
			if spec, ok := schema["items"].(map[string]any); ok {
				for _, child := range item {
					if err := check(child, spec); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	return check(value, JSONSchemaV1())
}
