package handlers

import (
	"go.yaml.in/yaml/v2"
	"strings"
	"testing"
)

func TestProjectOpenAPIParsesAndReferencesResolve(t *testing.T) {
	var root map[any]any
	if err := yaml.UnmarshalStrict(embeddedOpenAPISpec, &root); err != nil {
		t.Fatal(err)
	}
	if root["openapi"] != "3.0.3" {
		t.Fatal("unexpected OpenAPI version")
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[any]any:
			if ref, ok := x["$ref"].(string); ok {
				if !strings.HasPrefix(ref, "#/") {
					t.Fatalf("unresolved external ref %s", ref)
				}
				var current any = root
				for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
					obj, ok := current.(map[any]any)
					if !ok {
						t.Fatalf("invalid ref %s", ref)
					}
					current, ok = obj[part]
					if !ok {
						t.Fatalf("unknown ref %s", ref)
					}
				}
			}
			for _, value := range x {
				walk(value)
			}
		case []any:
			for _, value := range x {
				walk(value)
			}
		}
	}
	walk(root)
	schemas := root["components"].(map[any]any)["schemas"].(map[any]any)
	page := schemas["ClinicalProjectExportPage"].(map[any]any)["properties"].(map[any]any)
	if page["items"].(map[any]any)["maxItems"] != 25 || page["requires_external_manual_consent"] == nil {
		t.Fatal("missing bounded history/policy contract")
	}
	for _, name := range []string{"ClinicalProjectImportV1", "ClinicalProjectExport", "ClinicalProjectStrategy", "ClinicalExternalProposal"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("missing contract %s", name)
		}
	}
}
