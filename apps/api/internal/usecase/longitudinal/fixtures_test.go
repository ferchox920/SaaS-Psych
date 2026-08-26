package longitudinal

import (
	"encoding/json"
	"os"
	"testing"
)

func TestLongitudinalClinicalFixturesAThroughG(t *testing.T) {
	raw, err := os.ReadFile("testdata/longitudinal_eval_fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name          string              `json:"name"`
		Operations    []ProposedOperation `json:"operations"`
		Uncertainties []Uncertainty       `json:"uncertainties"`
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 7 {
		t.Fatalf("fixtures=%d", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			encoded, _ := json.Marshal(InterpreterResult{Operations: fixture.Operations, Uncertainties: fixture.Uncertainties})
			result, err := DecodeInterpreterResult(encoded)
			if err != nil {
				t.Fatal(err)
			}
			switch fixture.Name {
			case "D_insufficient_evidence":
				if len(result.Operations) != 0 || result.Uncertainties[0].Type != "insufficient_evidence" {
					t.Fatal("must not invent a conclusion")
				}
			case "E_report_hypothesis_remains_candidate":
				for _, op := range result.Operations {
					if op.OperationType == "create_hypothesis" {
						t.Fatal("report hypothesis candidate was promoted")
					}
				}
			case "G_competing_hypotheses":
				if len(result.Operations) != 2 {
					t.Fatal("competing hypotheses must coexist")
				}
			}
		})
	}
}
