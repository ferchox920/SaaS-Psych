package ollama

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sessionflow/apps/api/internal/usecase/sessionreport"
	"strings"
	"testing"
)

func TestSessionReportSamplerCompatibilityPreservesAcceptedContract(t *testing.T) {
	schema := sessionreport.JSONSchemaV1()
	before, _ := json.Marshal(schema)
	adapted := sessionReportGenerationSchema(schema)
	after, _ := json.Marshal(schema)
	if !bytes.Equal(before, after) {
		t.Fatal("canonical schema mutated")
	}
	raw, _ := json.Marshal(adapted)
	if strings.Contains(string(raw), "maxLength") {
		t.Fatal("sampler repetition bound remains")
	}
	if !reflect.DeepEqual(schema["required"], adapted["required"]) || adapted["additionalProperties"] != false {
		t.Fatal("required fields/strict shape weakened")
	}
	report := sessionreport.ReportV1{}
	base := `{"schema_version":"session-report-v1.1","summary":"synthetic","facts":[],"relevant_changes":[],"interventions":[],"patient_responses":[],"affective_nodes":[],"inference_candidates":[],"hypothesis_candidates":[],"safety_signals":[],"open_questions":[],"longitudinal_candidates":[]}`
	if err := json.Unmarshal([]byte(base), &report); err != nil {
		t.Fatal(err)
	}
	if err := sessionreport.Validate(report); err != nil {
		t.Fatal(err)
	}
	report.Summary = strings.Repeat("á", 2001)
	if sessionreport.Validate(report) == nil {
		t.Fatal("canonical maxLength not enforced locally")
	}
}
