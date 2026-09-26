package demoprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"sessionflow/apps/api/internal/usecase/longitudinal"
	"sessionflow/apps/api/internal/usecase/sessionreport"
)

func TestDemoProviderIsDeterministicAndStructurallyValid(t *testing.T) {
	provider := New()
	first, err := provider.GenerateSessionReport(context.Background(), "ignored", []byte(`{"session_text":"synthetic A"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.GenerateSessionReport(context.Background(), "ignored", []byte(`{"session_text":"synthetic B"}`), nil)
	if err != nil || !bytes.Equal(first.JSON, second.JSON) {
		t.Fatalf("demo report changed with input: %v", err)
	}
	report, err := sessionreport.DecodeAndValidate(first.JSON)
	if err != nil || len(report.Facts) != 1 {
		t.Fatalf("invalid demo report: %#v err=%v", report, err)
	}
	input := longitudinal.InterpreterInput{SchemaVersion: longitudinal.PromptVersion, SessionReport: json.RawMessage(first.JSON)}
	output, err := provider.InterpretLongitudinal(context.Background(), "ignored", input, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := longitudinal.DecodeInterpreterResult(output.JSON)
	if err != nil || len(result.Operations) != 1 {
		t.Fatalf("invalid demo operations: %#v err=%v", result, err)
	}
	if err := longitudinal.ValidateInterpreterSemantics(first.JSON, result); err != nil {
		t.Fatal(err)
	}
	if err := longitudinal.ValidateInterpreterReferences(input, result); err != nil {
		t.Fatal(err)
	}
	if result.Operations[0].TargetEntityID == nil || *result.Operations[0].TargetEntityID == uuid.Nil {
		t.Fatal("missing reserved evidence identity")
	}
	other, err := provider.InterpretLongitudinal(context.Background(), "ignored", input, nil)
	if err != nil || !bytes.Equal(other.JSON, output.JSON) {
		t.Fatalf("demo longitudinal output is not deterministic: %v", err)
	}
}
