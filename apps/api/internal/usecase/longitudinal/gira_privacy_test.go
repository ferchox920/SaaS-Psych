package longitudinal

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func syntheticPrivacySnapshot() GIRABuilderInput {
	tenant := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	client := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	processID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	supportID := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	contraID := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	support := Evidence{ID: supportID, TenantID: tenant, ClientID: client, EpistemicType: "patient_report", Statement: "Paciente María Prueba escribió maria@example.test y +54 11 4000-0000; vive en Calle Falsa 123; DNI TEST-DNI-1234; nació 1988-04-03.", Status: "active", Version: 1, UpdatedAt: time.Unix(2, 0)}
	contra := Evidence{ID: contraID, TenantID: tenant, ClientID: client, EpistemicType: "therapist_observation", Statement: "Su supervisor Juan Pérez de Empresa XYZ rechazó el límite; la doctora Ana Ensayo trabaja en Hospital Central, barrio Palermo.", Status: "active", Version: 1, UpdatedAt: time.Unix(1, 0)}
	hypothesis := Hypothesis{ID: uuid.MustParse("66666666-6666-4666-8666-666666666666"), TenantID: tenant, ClientID: client, ProcessID: &processID, Statement: "La neurocirujana pediátrica teme perder el vínculo con Juan Pérez.", ApprovalStatus: "approved", ClinicalStatus: "active", ConfidenceLevel: "yellow", Version: 1, SupportingEvidence: []Evidence{support}, ContradictingEvidence: []Evidence{contra}}
	event := Event{ID: uuid.MustParse("77777777-7777-4777-8777-777777777777"), TenantID: tenant, ClientID: client, EventType: "relational_pattern", Title: "Cita 2026-08-31T14:30:00Z en Escuela Norte", Description: "Pareja Pedro Tercero anticipó rechazo en ciudad de Rosario.", ApprovalStatus: "approved", Version: 1, Evidence: []Evidence{support}}
	strategy := TherapeuticStrategy{Targets: []Target{}, Goals: []Goal{}, Rationales: []TherapeuticRationale{}, GIRAs: []GIRA{}}
	process := Process{ID: processID, TenantID: tenant, ClientID: client, Title: "Proceso de María Prueba", Description: "María Prueba evita límites ante su padre Carlos Prueba; turno exacto 31/08/2026.", ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1, Hypotheses: []Hypothesis{hypothesis}, TherapeuticStrategy: &strategy}
	return GIRABuilderInput{SchemaVersion: GIRAPromptVersion, SelectedProcess: process, ApprovedEvidence: []Evidence{contra, support}, ApprovedEvents: []Event{event}, ApprovedHypotheses: []Hypothesis{hypothesis}, CurrentStrategy: strategy, ApproachRegistry: []ApproachDefinition{}, TechniqueRegistry: []TechniqueDefinition{}, StateVersion: 1}
}

func TestRemoteGIRAContextSerializedPayloadExcludesSyntheticPIIAndInternalIDs(t *testing.T) {
	req := BuildGIRAGenerationRequest(syntheticPrivacySnapshot())
	raw, err := json.Marshal(req.RemoteContext)
	if err != nil {
		t.Fatal(err)
	}
	payload := strings.ToLower(string(raw))
	for _, secret := range []string{
		"maría prueba", "maria@example.test", "+54 11 4000-0000", "calle falsa 123", "test-dni-1234",
		"1988-04-03", "2026-08-31t14:30:00z", "31/08/2026", "juan pérez", "ana ensayo", "pedro tercero",
		"empresa xyz", "hospital central", "escuela norte", "palermo", "rosario", "neurocirujana pediátrica",
		"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333",
	} {
		if strings.Contains(payload, strings.ToLower(secret)) {
			t.Errorf("serialized outbound payload leaked synthetic secret %q: %s", secret, raw)
		}
	}
	for _, replacement := range []string{"[person]", "[email]", "[phone]", "[location]", "[document]", "[date]", "[organization]", "[profession]"} {
		if !strings.Contains(payload, replacement) {
			t.Errorf("expected minimized marker %q in payload: %s", replacement, raw)
		}
	}
	if req.Privacy.PolicyVersion != GIRAPrivacyPolicyVersion || req.Privacy.PayloadBytes != len(raw) {
		t.Fatalf("invalid privacy summary: %+v bytes=%d", req.Privacy, len(raw))
	}
	if len(req.Privacy.ResidualRiskFlags) == 0 {
		t.Fatal("expected closed-set residual risk flags")
	}
}

func TestRemoteGIRAContextDTOHasNoForbiddenFields(t *testing.T) {
	forbidden := []string{"tenant", "client", "user", "appointment", "session", "airun", "name", "email", "phone", "address", "document", "calendar", "transcript", "audio", "report", "history", "audit", "uuid", "source_id"}
	seen := map[reflect.Type]bool{}
	var inspect func(reflect.Type)
	inspect = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
			lower := strings.ToLower(field.Name + " " + jsonName)
			for _, token := range forbidden {
				if strings.Contains(lower, token) {
					t.Errorf("forbidden outbound DTO field %s (%s) contains %q", field.Name, jsonName, token)
				}
			}
			inspect(field.Type)
		}
	}
	inspect(reflect.TypeOf(RemoteGIRAContext{}))
}

func TestRemoteGIRAContextPreservesEpistemicRolesAndRelationships(t *testing.T) {
	req := BuildGIRAGenerationRequest(syntheticPrivacySnapshot())
	if len(req.RemoteContext.Evidence) != 2 || len(req.RemoteContext.Hypotheses) != 1 || len(req.RemoteContext.Events) != 1 {
		t.Fatalf("unexpected remote context: %+v", req.RemoteContext)
	}
	types := []string{req.RemoteContext.Evidence[0].EpistemicType, req.RemoteContext.Evidence[1].EpistemicType}
	sort.Strings(types)
	if !reflect.DeepEqual(types, []string{"patient_report", "therapist_observation"}) {
		t.Fatalf("epistemic types collapsed: %v", types)
	}
	hypothesis := req.RemoteContext.Hypotheses[0]
	if !reflect.DeepEqual(hypothesis.SupportingEvidenceRefs, []string{"evidence_1"}) || !reflect.DeepEqual(hypothesis.ContradictingEvidenceRefs, []string{"evidence_2"}) {
		t.Fatalf("supporting/contradicting roles lost: %+v", hypothesis)
	}
	if !reflect.DeepEqual(req.RemoteContext.Events[0].EvidenceRefs, []string{"evidence_1"}) {
		t.Fatalf("event evidence relationship lost: %+v", req.RemoteContext.Events[0])
	}
}

func TestGIRAPrivacyMinimizationPreservesClinicalFunction(t *testing.T) {
	collector := newPrivacyCollector()
	before := "Juan Pérez rechazó nuevamente el límite y la paciente revirtió su decisión por miedo a que Juan dejara de hablarle."
	after := collector.text("evidence_statement", before)
	if !strings.HasPrefix(after, "[person] rechazó nuevamente el límite") || !strings.Contains(after, "revirtió su decisión por miedo") || !strings.Contains(after, "dejara de hablarle") {
		t.Fatalf("clinical function was not preserved: %q", after)
	}
	if strings.Contains(after, "Juan Pérez") {
		t.Fatalf("person identifier remained: %q", after)
	}
}

func TestRemoteGIRAAliasesAreDeterministicAndMappingStaysLocal(t *testing.T) {
	first := syntheticPrivacySnapshot()
	second := syntheticPrivacySnapshot()
	second.ApprovedEvidence[0], second.ApprovedEvidence[1] = second.ApprovedEvidence[1], second.ApprovedEvidence[0]
	a := BuildGIRAGenerationRequest(first)
	b := BuildGIRAGenerationRequest(second)
	aJSON, _ := json.Marshal(a.RemoteContext)
	bJSON, _ := json.Marshal(b.RemoteContext)
	if !bytes.Equal(aJSON, bJSON) {
		t.Fatalf("logical snapshot produced non-deterministic aliases\nA=%s\nB=%s", aJSON, bJSON)
	}
	providerRequest := NewGIRAProviderRequest(a)
	providerJSON, _ := json.Marshal(providerRequest)
	for _, internalID := range []string{first.SelectedProcess.ID.String(), first.ApprovedEvidence[0].ID.String(), first.ApprovedHypotheses[0].ID.String()} {
		if strings.Contains(string(providerJSON), internalID) {
			t.Fatalf("local alias mapping leaked UUID %s: %s", internalID, providerJSON)
		}
	}
	if !strings.Contains(string(providerJSON), "evidence_1") || !strings.Contains(string(providerJSON), "process_1") {
		t.Fatalf("opaque aliases absent: %s", providerJSON)
	}
}

func TestRemoteGIRAContextContainsOnlySelectedProcess(t *testing.T) {
	selected := syntheticPrivacySnapshot()
	unrelatedB := Process{ID: uuid.New(), Title: "UNRELATED_PROCESS_B_SECRET"}
	unrelatedC := Process{ID: uuid.New(), Title: "UNRELATED_PROCESS_C_SECRET"}
	state := State{ClientID: selected.SelectedProcess.ClientID, Processes: []Process{selected.SelectedProcess, unrelatedB, unrelatedC}}
	_ = state // Full longitudinal state is intentionally not accepted by the outbound builder.
	raw, _ := json.Marshal(BuildGIRAGenerationRequest(selected).RemoteContext)
	if strings.Contains(string(raw), unrelatedB.Title) || strings.Contains(string(raw), unrelatedC.Title) {
		t.Fatalf("unrelated process leaked: %s", raw)
	}
}

func BenchmarkBuildRemoteGIRAContext(b *testing.B) {
	snapshot := syntheticPrivacySnapshot()
	for i := 0; i < b.N; i++ {
		_ = BuildGIRAGenerationRequest(snapshot)
	}
}
