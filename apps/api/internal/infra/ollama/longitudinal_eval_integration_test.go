package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	approvedcontext "sessionflow/apps/api/internal/usecase/approvedcontext"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
)

type longitudinalOllamaFixture struct {
	name   string
	input  longitudinal.InterpreterInput
	assert func(*testing.T, longitudinal.InterpreterResult)
}

func TestOllamaLongitudinalEvaluationAThroughG(t *testing.T) {
	if os.Getenv("RUN_OLLAMA_LONGITUDINAL_EVAL") != "1" {
		t.Skip("set RUN_OLLAMA_LONGITUDINAL_EVAL=1 to run A-G against the installed local model")
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "qwen3.5:9b"
	}
	provider, err := NewProvider(Config{BaseURL: "http://127.0.0.1:11434", Model: model, ContextTokens: 8192, Temperature: 0.1, Timeout: 150 * time.Second, KeepAlive: "15m", MaxOutputTokens: 1536, ReviewContextTokens: 8192, ReviewTemperature: 0.1, ReviewTimeout: 150 * time.Second, ReviewMaxOutputTokens: 1536})
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range ollamaLongitudinalFixtures() {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			output, e := provider.InterpretLongitudinal(context.Background(), longitudinal.SystemPromptV1(), fixture.input, nil)
			if e != nil {
				t.Fatal(e)
			}
			result, e := longitudinal.DecodeInterpreterResult(output.JSON)
			if e != nil {
				t.Fatalf("invalid longitudinal output: %v; output=%s", e, output.JSON)
			}
			if e = longitudinal.ValidateInterpreterSemantics(fixture.input.SessionReport, result); e != nil {
				t.Fatalf("unsafe longitudinal semantics: %v; output=%s", e, output.JSON)
			}
			fixture.assert(t, result)
			t.Logf("fixture=%s model=%s operations=%d uncertainties=%d output=%s", fixture.name, model, len(result.Operations), len(result.Uncertainties), output.JSON)
		})
	}
}

func ollamaLongitudinalFixtures() []longitudinalOllamaFixture {
	tenant, client := uuid.New(), uuid.New()
	e1, e2, eventID, processID, hypothesisID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	reservedProcessID, competingH1ID, competingH2ID := uuid.New(), uuid.New(), uuid.New()
	baseState := func() longitudinal.State {
		return longitudinal.State{ClientID: client, Processes: []longitudinal.Process{}, UnassignedHypotheses: []longitudinal.Hypothesis{}, RecentEvents: []longitudinal.Event{}, ActiveEvidence: []longitudinal.Evidence{}, OpenProposals: []longitudinal.Diff{}}
	}
	input := func(report string, state longitudinal.State) longitudinal.InterpreterInput {
		return longitudinal.InterpreterInput{SchemaVersion: longitudinal.PromptVersion, SessionReport: json.RawMessage(report), ApprovedContext: approvedcontext.ApprovedClinicalContext{ClientID: client, SessionReports: nil, Sources: nil}, CurrentState: state}
	}
	report := func(facts, responses, hypotheses, candidates string) string {
		return `{"schema_version":"session-report-v1.1","summary":"Fixture clínico ficticio.","facts":` + facts + `,"relevant_changes":[],"interventions":[],"patient_responses":` + responses + `,"affective_nodes":[],"inference_candidates":[],"hypothesis_candidates":` + hypotheses + `,"safety_signals":[],"open_questions":[],"longitudinal_candidates":` + candidates + `}`
	}
	find := func(result longitudinal.InterpreterResult, kinds ...string) bool {
		for _, op := range result.Operations {
			for _, kind := range kinds {
				if op.OperationType == kind {
					return true
				}
			}
		}
		return false
	}
	stateA := baseState()
	stateA.ActiveEvidence = []longitudinal.Evidence{{ID: e1, TenantID: tenant, ClientID: client, Status: "active", Version: 1, SourceType: "session_report", SourceID: uuid.New(), SourceVersion: 1, SourceItemID: "fact-001", EpistemicType: "patient_report", Statement: "Cambio informado."}}
	stateA.RecentEvents = []longitudinal.Event{{ID: eventID, TenantID: tenant, ClientID: client, EventType: "reported_change", Title: "Cambio informado", Description: "Cambio informado.", ApprovalStatus: "approved", Version: 1, Evidence: stateA.ActiveEvidence}}
	stateA.Processes = []longitudinal.Process{{ID: processID, TenantID: tenant, ClientID: client, Title: "Autonomía", Description: "Proceso existente.", ApprovalStatus: "approved", ClinicalStatus: "active", Version: 2, Events: []longitudinal.Event{}, Hypotheses: []longitudinal.Hypothesis{}}}
	stateC := baseState()
	stateC.ActiveEvidence = []longitudinal.Evidence{{ID: e2, TenantID: tenant, ClientID: client, Status: "active", Version: 1, SourceType: "session_report", SourceID: uuid.New(), SourceVersion: 1, SourceItemID: "response-001", EpistemicType: "patient_report", Statement: "La persona corrigió la interpretación."}}
	stateC.Processes = []longitudinal.Process{{ID: processID, TenantID: tenant, ClientID: client, Title: "Decisiones", Description: "Proceso existente.", ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1, Events: []longitudinal.Event{}, Hypotheses: []longitudinal.Hypothesis{{ID: hypothesisID, TenantID: tenant, ClientID: client, ProcessID: &processID, Statement: "La persona evita decidir.", ApprovalStatus: "approved", ClinicalStatus: "active", ConfidenceLevel: "yellow", Version: 2, SupportingEvidence: []longitudinal.Evidence{{ID: e1}}, ContradictingEvidence: []longitudinal.Evidence{}}}}}
	stateB := baseState()
	stateB.ActiveEvidence = []longitudinal.Evidence{
		{ID: e1, TenantID: tenant, ClientID: client, Status: "active", Version: 1, SourceType: "session_report", SourceID: uuid.New(), SourceVersion: 1, SourceItemID: "fact-prior-001", EpistemicType: "patient_report", Statement: "Patrón repetido de aislamiento informado previamente."},
		{ID: e2, TenantID: tenant, ClientID: client, Status: "active", Version: 1, SourceType: "session_report", SourceID: uuid.New(), SourceVersion: 1, SourceItemID: "fact-prior-002", EpistemicType: "patient_report", Statement: "El aislamiento volvió a presentarse en otro contexto."},
	}
	stateB.RecentEvents = []longitudinal.Event{{ID: eventID, TenantID: tenant, ClientID: client, EventType: "behavior", Title: "Aislamiento repetido", Description: "Evento aprobado previo.", ApprovalStatus: "approved", Version: 1, Evidence: stateB.ActiveEvidence}}
	stateG := baseState()
	stateG.ActiveEvidence = []longitudinal.Evidence{
		{ID: e1, TenantID: tenant, ClientID: client, Status: "active", Version: 1, SourceType: "session_report", SourceID: uuid.New(), SourceVersion: 1, SourceItemID: "fact-001", EpistemicType: "patient_report", Statement: "La persona informó temor al rechazo."},
		{ID: e2, TenantID: tenant, ClientID: client, Status: "active", Version: 1, SourceType: "session_report", SourceID: uuid.New(), SourceVersion: 1, SourceItemID: "fact-002", EpistemicType: "patient_report", Statement: "La persona informó una regla moral contra confrontar."},
	}
	stateG.Processes = []longitudinal.Process{{ID: processID, TenantID: tenant, ClientID: client, Title: "Evitación de conflicto", Description: "Proceso bajo observación.", ApprovalStatus: "approved", ClinicalStatus: "observing", Version: 1, Events: []longitudinal.Event{}, Hypotheses: []longitudinal.Hypothesis{}}}
	stateG.RecentEvents = []longitudinal.Event{{ID: eventID, TenantID: tenant, ClientID: client, EventType: "behavior", Title: "Evitación de conflicto", Description: "Evento aprobado que reúne ambos relatos.", ApprovalStatus: "approved", Version: 1, Evidence: stateG.ActiveEvidence}}
	return []longitudinalOllamaFixture{
		{name: "A_existing_process", input: input(report(`[{"id":"fact-001","statement":"La persona informó un cambio vinculado a autonomía.","category":"reported_change"}]`, `[]`, `[]`, `[{"id":"longitudinal-001","operation":"possible_existing_process_update","rationale":"Ejecutar únicamente link_event_process para vincular el evento reciente ya aprobado con el proceso existente de autonomía; no crear ni actualizar otro proceso."}]`), stateA), assert: func(t *testing.T, r longitudinal.InterpreterResult) {
			if !find(r, "link_event_process") || find(r, "create_process") {
				t.Fatalf("expected link without duplicate process: %#v", r)
			}
		}},
		{name: "B_new_thread", input: input(report(`[]`, `[]`, `[]`, fmt.Sprintf(`[{"id":"longitudinal-001","operation":"possible_new_process","rationale":"Proponer una única create_process observing para el patrón repetido de aislamiento, reservando target_entity_id exacto %s. evidence_ids deben ser exactamente %s y %s. Si vincula el evento aprobado %s después, process_id/target debe reutilizar exactamente %s con expected version 1. No crear hipótesis o evidencia; no existe proceso equivalente."}]`, reservedProcessID, e1, e2, eventID, reservedProcessID)), stateB), assert: func(t *testing.T, r longitudinal.InterpreterResult) {
			if !find(r, "create_process") {
				t.Fatalf("expected proposed process: %#v", r)
			}
		}},
		{name: "C_contradicted_hypothesis", input: input(report(`[]`, `[{"id":"response-001","response_type":"discrepancy","description":"La persona corrigió la interpretación y aportó un dato contrario."}]`, `[]`, `[{"id":"longitudinal-001","operation":"possible_hypothesis_update","rationale":"Usar link_contradicting_evidence con los UUID exactos de la hipótesis activa y evidencia activa visibles en el proceso existente; target_entity_id debe ser hypothesis_id y expected version 2. No crear ni actualizar procesos o eventos."}]`), stateC), assert: func(t *testing.T, r longitudinal.InterpreterResult) {
			if !find(r, "link_contradicting_evidence", "weaken_hypothesis") {
				t.Fatalf("expected contradiction handling: %#v", r)
			}
		}},
		{name: "D_insufficient_evidence", input: input(report(`[]`, `[]`, `[]`, `[]`), baseState()), assert: func(t *testing.T, r longitudinal.InterpreterResult) {
			if len(r.Operations) != 0 || len(r.Uncertainties) == 0 {
				t.Fatalf("expected uncertainty without strong hypothesis: %#v", r)
			}
		}},
		{name: "E_report_hypothesis_candidate", input: input(report(`[]`, `[]`, `[{"id":"hypothesis-001","statement":"Explicación provisional sin evidencia independiente.","traffic_light":"yellow","evidence_refs":[]}]`, `[{"id":"longitudinal-001","operation":"insufficient_evidence","rationale":"No hay evidencia elegible ni entidades existentes: devolver cero operaciones y una uncertainty; el hypothesis_candidate debe seguir siendo candidato."}]`), baseState()), assert: func(t *testing.T, r longitudinal.InterpreterResult) {
			if len(r.Operations) != 0 || len(r.Uncertainties) == 0 {
				t.Fatalf("candidate was promoted: %#v", r)
			}
		}},
		{name: "F_patient_correction", input: input(report(`[]`, `[{"id":"response-001","response_type":"correction","description":"La persona corrigió explícitamente la lectura previa."}]`, `[]`, fmt.Sprintf(`[{"id":"longitudinal-001","operation":"possible_hypothesis_update","rationale":"Tratar la corrección como información nueva/contradictoria. Usar evidence_id exacto %s y hypothesis_id/target exacto %s, expected version 2. Puede link_contradicting_evidence, weaken_hypothesis o update_hypothesis manteniendo epistemicidad; nunca resistencia. No crear procesos o eventos."}]`, e2, hypothesisID)), stateC), assert: func(t *testing.T, r longitudinal.InterpreterResult) {
			if !find(r, "create_evidence", "link_contradicting_evidence", "weaken_hypothesis", "update_hypothesis") {
				t.Fatalf("correction was not retained as data: %#v", r)
			}
		}},
		{name: "G_competing_hypotheses", input: input(report(`[]`, `[]`, `[]`, fmt.Sprintf(`[{"id":"longitudinal-001","operation":"possible_hypothesis_update","rationale":"Proponer exactamente dos create_hypothesis competidoras red con process_id exacto %s. Primera target_entity_id exacto %s y supporting_evidence_ids exactamente [%s] para temor al rechazo; segunda target_entity_id exacto %s y supporting_evidence_ids exactamente [%s] para regla moral. El evento aprobado %s ya existe y no requiere ninguna operación. No crear, invalidar o vincular evidencia, eventos o procesos."}]`, processID, competingH1ID, e1, competingH2ID, e2, eventID)), stateG), assert: func(t *testing.T, r longitudinal.InterpreterResult) {
			count := 0
			for _, op := range r.Operations {
				if op.OperationType == "create_hypothesis" {
					count++
				}
			}
			if count < 2 {
				t.Fatalf("expected competing hypotheses: %#v", r)
			}
		}},
	}
}
