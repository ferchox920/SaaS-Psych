package clinicalanalysis

import (
	"strings"
	"testing"
)

func TestGenericClinicalCopyDoesNotNameClinician(t *testing.T) {
	result := ApplyDeterministicGuards(LiveRequest{Fragment: "Caso ficticio: no quiero vivir."}, guardedResult())
	for label, value := range map[string]string{
		"live prompt":   LiveSystemPrompt(),
		"review prompt": ReviewSystemPrompt(),
		"risk caution":  result.Caution,
	} {
		if strings.Contains(strings.ToLower(value), "fernando") {
			t.Errorf("%s names an individual in generic clinical copy", label)
		}
	}
	if !strings.Contains(result.Caution, "protocolo clínico") || !strings.Contains(LiveSystemPrompt(), "juicio clínico") {
		t.Fatal("generic language must preserve the clinical protocol and human-judgment safeguards")
	}
}

func TestApplyDeterministicGuardsBlocksRedConfrontation(t *testing.T) {
	result := guardedResult()
	result.Hypothesis.TrafficLight = TrafficRed
	result.Now = NowConfront

	result = ApplyDeterministicGuards(LiveRequest{Fragment: "Fragmento ficticio."}, result)
	if result.Now != NowExplore {
		t.Fatalf("expected explore, got %s", result.Now)
	}
}

func TestApplyDeterministicGuardsDowngradesUnsupportedFact(t *testing.T) {
	result := guardedResult()
	result.Hypothesis.EpistemicLevel = EpistemicFact
	result.Evidence = []Evidence{{SourceID: "current_fragment", Kind: EpistemicInference, Summary: "Lectura provisional"}}

	result = ApplyDeterministicGuards(LiveRequest{Fragment: "Fragmento ficticio."}, result)
	if result.Hypothesis.EpistemicLevel != EpistemicHypothesis || result.Hypothesis.TrafficLight != TrafficRed {
		t.Fatalf("expected unsupported fact to be downgraded, got %+v", result.Hypothesis)
	}
}

func TestApplyDeterministicGuardsTreatsCorrectionAsHypothesisUpdate(t *testing.T) {
	result := guardedResult()
	result.Hypothesis.TrafficLight = TrafficGreen

	result = ApplyDeterministicGuards(LiveRequest{Fragment: "Fragmento ficticio.", PatientResponse: "No es así, me entendiste mal."}, result)
	if result.Hypothesis.TrafficLight != TrafficYellow || result.Hypothesis.EpistemicLevel != EpistemicHypothesis {
		t.Fatalf("expected correction to weaken hypothesis, got %+v", result.Hypothesis)
	}
}

func TestApplyDeterministicGuardsPrioritizesResponseAfterConfrontation(t *testing.T) {
	result := guardedResult()
	result.Now = NowConfront

	result = ApplyDeterministicGuards(LiveRequest{
		Fragment: "Fragmento ficticio.", PreviousAction: NowConfront, PatientResponse: "Se queda callado y mira al piso.",
	}, result)
	if result.Now != NowReflect {
		t.Fatalf("expected reflect after confrontation, got %s", result.Now)
	}
}

func TestApplyDeterministicGuardsRiskOverridesInterpretation(t *testing.T) {
	result := guardedResult()
	result.Now = NowValues
	result.SuggestedInterventions = []string{"Interpretación profunda", "Confrontación", "Tercera"}

	result = ApplyDeterministicGuards(LiveRequest{Fragment: "A veces pienso: me voy a matar."}, result)
	if !result.Risk.RequiresHumanAssessment || result.Now != NowRegulate || result.Hypothesis.TrafficLight != TrafficRed {
		t.Fatalf("expected safe risk override, got %+v", result)
	}
	if len(result.SuggestedInterventions) != 1 {
		t.Fatalf("expected one safety intervention, got %d", len(result.SuggestedInterventions))
	}
}

func TestApplyDeterministicGuardsAllowsNoIntervention(t *testing.T) {
	result := guardedResult()
	result.Now = NowNoIntervention
	result.SuggestedInterventions = nil
	result = ApplyDeterministicGuards(LiveRequest{Fragment: "Fragmento ficticio."}, result)
	if err := ValidateResult(result); err != nil {
		t.Fatalf("expected no-intervention output to be valid, got %v", err)
	}
}

func guardedResult() Result {
	return Result{
		Mode: "live", Node: "Cambio afectivo", Caution: "Mantener formulación provisional.",
		Hypothesis: Hypothesis{Text: "Hipótesis ficticia.", EpistemicLevel: EpistemicHypothesis, TrafficLight: TrafficYellow},
		Evidence:   []Evidence{{SourceID: "current_fragment", Kind: EpistemicFact, Summary: "El paciente ficticio expresó una duda."}},
		Now:        NowExplore, SuggestedInterventions: []string{"¿Qué cambia para ti al decirlo ahora?"},
		Risk: Risk{},
	}
}
