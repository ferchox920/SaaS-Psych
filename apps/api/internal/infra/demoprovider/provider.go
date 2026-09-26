package demoprovider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"sessionflow/apps/api/internal/usecase/clinicalanalysis"
	"sessionflow/apps/api/internal/usecase/longitudinal"
	"sessionflow/apps/api/internal/usecase/sessionreport"
)

// Provider is a fixed, synthetic fixture for local portfolio demonstrations.
// It deliberately does not infer anything from the submitted clinical text.
type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) GenerateSessionReport(ctx context.Context, _ string, _ []byte, _ map[string]any) (sessionreport.ProviderOutput, error) {
	if err := ctx.Err(); err != nil {
		return sessionreport.ProviderOutput{}, err
	}
	report := sessionreport.ReportV1{
		SchemaVersion: sessionreport.SchemaVersion,
		Summary:       "SIMULACIÓN: informe fijo de una sesión ficticia. Requiere revisión profesional; no es una inferencia clínica.",
		Facts: []sessionreport.Fact{{
			ID: "fact-001", Statement: "En el caso ficticio, la persona describió que pospuso una conversación difícil.", Category: "patient_report",
		}},
		RelevantChanges: []sessionreport.RelevantChange{}, Interventions: []sessionreport.Intervention{},
		PatientResponses: []sessionreport.PatientResponse{}, AffectiveNodes: []sessionreport.AffectiveNode{},
		InferenceCandidates: []sessionreport.InferenceCandidate{},
		HypothesisCandidates: []sessionreport.HypothesisCandidate{{
			ID: "hypothesis-001", Statement: "Candidato simulado: podría haber evitación; explorar, no asumir.", TrafficLight: "yellow", EvidenceRefs: []string{"fact-001"},
		}},
		SafetySignals: []sessionreport.SafetySignal{}, OpenQuestions: []sessionreport.OpenQuestion{},
		LongitudinalCandidates: []sessionreport.LongitudinalCandidate{},
	}
	encoded, err := json.Marshal(report)
	return sessionreport.ProviderOutput{JSON: encoded}, err
}

func (p *Provider) InterpretLongitudinal(ctx context.Context, _ string, input longitudinal.InterpreterInput, _ func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	if err := ctx.Err(); err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	var source struct {
		Facts []struct {
			ID string `json:"id"`
		} `json:"facts"`
	}
	if err := json.Unmarshal(input.SessionReport, &source); err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	result := longitudinal.InterpreterResult{Operations: []longitudinal.ProposedOperation{}, Uncertainties: []longitudinal.Uncertainty{}}
	if len(source.Facts) > 0 {
		reservedID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("sessionflow-demo:%s:%d", input.SessionReport, input.CurrentState.StateVersion)))
		proposal, err := json.Marshal(longitudinal.CreateEvidenceProposal{SourceItemID: source.Facts[0].ID, EpistemicType: "patient_report"})
		if err != nil {
			return clinicalanalysis.ProviderOutput{}, err
		}
		result.Operations = append(result.Operations, longitudinal.ProposedOperation{
			ID: "synthetic-evidence-1", OperationType: "create_evidence", TargetEntityID: &reservedID, Proposal: proposal,
		})
	}
	encoded, err := json.Marshal(result)
	return clinicalanalysis.ProviderOutput{JSON: encoded}, err
}
