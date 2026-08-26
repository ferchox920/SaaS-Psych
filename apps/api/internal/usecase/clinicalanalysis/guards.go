package clinicalanalysis

import (
	"fmt"
	"strings"
	"unicode"
)

var validNow = map[NowAction]bool{
	NowListen: true, NowClarify: true, NowReflect: true, NowExplore: true,
	NowConfront: true, NowRestructure: true, NowResignify: true, NowValues: true,
	NowRegulate: true, NowNoIntervention: true,
}

func ValidateResult(result Result) error {
	if result.Mode != "live" {
		return fmt.Errorf("mode must be live")
	}
	if strings.TrimSpace(result.Node) == "" || strings.TrimSpace(result.Hypothesis.Text) == "" {
		return fmt.Errorf("node and hypothesis text are required")
	}
	if result.Hypothesis.EpistemicLevel != EpistemicFact && result.Hypothesis.EpistemicLevel != EpistemicInference && result.Hypothesis.EpistemicLevel != EpistemicHypothesis {
		return fmt.Errorf("invalid epistemic level")
	}
	if result.Hypothesis.TrafficLight != TrafficGreen && result.Hypothesis.TrafficLight != TrafficYellow && result.Hypothesis.TrafficLight != TrafficRed {
		return fmt.Errorf("invalid traffic light")
	}
	if !validNow[result.Now] {
		return fmt.Errorf("invalid now action")
	}
	if len(result.Evidence) > 4 || len(result.SuggestedInterventions) > 2 {
		return fmt.Errorf("live output exceeds evidence or intervention limit")
	}
	for _, evidence := range result.Evidence {
		if strings.TrimSpace(evidence.SourceID) == "" || strings.TrimSpace(evidence.Summary) == "" {
			return fmt.Errorf("evidence source_id and summary are required")
		}
		if evidence.Kind != EpistemicFact && evidence.Kind != EpistemicInference {
			return fmt.Errorf("evidence kind must be fact or inference")
		}
	}
	if result.Hypothesis.TrafficLight == TrafficRed && result.Now == NowConfront {
		return fmt.Errorf("red traffic light cannot confront")
	}
	if result.Risk.Detected && !result.Risk.RequiresHumanAssessment {
		return fmt.Errorf("detected risk requires human assessment")
	}
	return nil
}

func ApplyDeterministicGuards(request LiveRequest, result Result) Result {
	allowedSources := map[string]bool{"current_fragment": true}
	for _, item := range request.RelevantContext {
		allowedSources[item.SourceID] = true
	}
	filteredEvidence := make([]Evidence, 0, min(len(result.Evidence), 4))
	for _, evidence := range result.Evidence {
		if allowedSources[evidence.SourceID] && strings.TrimSpace(evidence.Summary) != "" {
			filteredEvidence = append(filteredEvidence, evidence)
			if len(filteredEvidence) == 4 {
				break
			}
		}
	}
	result.Evidence = filteredEvidence
	if len(result.SuggestedInterventions) > 2 {
		result.SuggestedInterventions = result.SuggestedInterventions[:2]
	}
	if result.Hypothesis.EpistemicLevel == EpistemicFact && !hasFactEvidence(result.Evidence) {
		result.Hypothesis.EpistemicLevel = EpistemicHypothesis
		result.Hypothesis.TrafficLight = TrafficRed
		result.Caution = appendCaution(result.Caution, "No hay evidencia directa suficiente para presentarlo como hecho; explorar antes de afirmar.")
	}
	if result.Hypothesis.TrafficLight == TrafficRed && result.Now == NowConfront {
		result.Now = NowExplore
		result.Caution = appendCaution(result.Caution, "Semáforo rojo: explorar de forma provisional, sin confrontar como verdad.")
	}
	if isPatientCorrection(request.PatientResponse) && result.Hypothesis.TrafficLight == TrafficGreen {
		result.Hypothesis.TrafficLight = TrafficYellow
		result.Hypothesis.EpistemicLevel = EpistemicHypothesis
		result.Caution = appendCaution(result.Caution, "La corrección del paciente modifica la hipótesis; aclarar su significado sin asumir resistencia.")
	}
	if request.PreviousAction == NowConfront && strings.TrimSpace(request.PatientResponse) != "" && result.Now == NowConfront {
		result.Now = NowReflect
		result.Caution = appendCaution(result.Caution, "Después de la confrontación, analizar primero la respuesta del paciente antes de confrontar nuevamente.")
	}

	category, detected := DetectRisk(request.Fragment)
	if detected || result.Risk.Detected {
		if category == "" && result.Risk.Category != nil {
			category = *result.Risk.Category
		}
		if category == "" {
			category = "possible_clinical_risk"
		}
		result.Risk.Detected = true
		result.Risk.Category = &category
		result.Risk.RequiresHumanAssessment = true
		result.Now = NowRegulate
		result.Hypothesis.EpistemicLevel = EpistemicHypothesis
		result.Hypothesis.TrafficLight = TrafficRed
		result.Hypothesis.Text = "Posible indicador de riesgo que requiere evaluación clínica humana; este fragmento no permite confirmarlo ni descartarlo."
		result.Caution = "Suspender interpretaciones confrontativas o existenciales y aplicar el protocolo clínico configurado por Fernando."
		result.SuggestedInterventions = []string{"Evaluar directamente seguridad, inmediatez, medios, intención, protección y capacidad de autocuidado según el protocolo clínico."}
		result.TherapistMeta = nil
	}
	return result
}

func hasFactEvidence(items []Evidence) bool {
	for _, item := range items {
		if item.Kind == EpistemicFact {
			return true
		}
	}
	return false
}

func appendCaution(current, addition string) string {
	if strings.TrimSpace(current) == "" {
		return addition
	}
	return strings.TrimSpace(current) + " " + addition
}

func DetectRisk(fragment string) (string, bool) {
	text := normalizeText(fragment)
	categories := []struct {
		name    string
		phrases []string
	}{
		{"suicide_or_self_harm", []string{"me quiero matar", "me voy a matar", "quiero suicidarme", "no quiero vivir", "hacerme dano", "cortarme", "quitarme la vida"}},
		{"violence_or_abuse", []string{"voy a matarlo", "voy a matarla", "quiero hacerle dano", "me esta golpeando", "abuso sexual", "violencia en casa"}},
		{"psychosis", []string{"voces me ordenan", "me estan persiguiendo", "controlan mis pensamientos"}},
		{"intoxication", []string{"sobredosis", "intoxicado", "no puedo parar de consumir"}},
		{"medical_or_self_care", []string{"no puedo respirar", "dolor en el pecho", "no como hace dias", "no puedo cuidarme"}},
	}
	for _, category := range categories {
		for _, phrase := range category.phrases {
			if strings.Contains(text, phrase) {
				return category.name, true
			}
		}
	}
	return "", false
}

func isPatientCorrection(response string) bool {
	text := normalizeText(response)
	phrases := []string{"eso no", "no es asi", "no quise decir", "me entendiste mal", "no exactamente", "te corrijo"}
	for _, phrase := range phrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func normalizeText(value string) string {
	lower := strings.ToLower(value)
	return strings.Map(func(r rune) rune {
		switch r {
		case 'á', 'à', 'ä', 'â':
			return 'a'
		case 'é', 'è', 'ë', 'ê':
			return 'e'
		case 'í', 'ì', 'ï', 'î':
			return 'i'
		case 'ó', 'ò', 'ö', 'ô':
			return 'o'
		case 'ú', 'ù', 'ü', 'û':
			return 'u'
		case 'ñ':
			return 'n'
		default:
			if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
				return r
			}
			return ' '
		}
	}, lower)
}
