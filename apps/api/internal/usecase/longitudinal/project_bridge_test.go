package longitudinal

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

func projectFixture(t *testing.T) (ProjectExport, State, ClinicalProjectImportV1) {
	t.Helper()
	req, strategy := semanticCompilerFixture()
	tenant, client, actor := uuid.New(), uuid.New(), uuid.New()
	p := req.Snapshot.SelectedProcess
	p.TenantID = tenant
	p.ClientID = client
	p.Title = "Selected process"
	e := req.Snapshot.ApprovedEvidence[0]
	e.TenantID = tenant
	e.ClientID = client
	e.EpistemicType = "patient_report"
	h := Hypothesis{ID: uuid.New(), TenantID: tenant, ClientID: client, ProcessID: &p.ID, Statement: "Exploratory avoidance hypothesis", ApprovalStatus: "approved", ClinicalStatus: "active", ConfidenceLevel: "yellow", Version: 1, SupportingEvidence: []Evidence{e}, ContradictingEvidence: []Evidence{}}
	p.Hypotheses = []Hypothesis{h}
	state := State{ClientID: client, StateVersion: 5, Processes: []Process{p}, ActiveEvidence: []Evidence{e}}
	export, err := BuildProjectExport(tenant, client, actor, state, p.ID, req.Snapshot.ApproachRegistry, req.Snapshot.TechniqueRegistry)
	if err != nil {
		t.Fatal(err)
	}
	proposal := ClinicalProjectImportV1{SchemaVersion: ProjectImportVersion, SourceExportID: export.ID, SourceExportHash: export.ContentHash, Provenance: ManualProvenance{SourceType: "manual_external_ai", Assertion: "user_supplied", Surface: "ChatGPT Project"}, Operations: []PortableProjectOperation{}, Strategy: &strategy, OpenQuestions: []GIRASemanticUncertainty{}, SupervisionObservations: []string{"Synthetic supervision observation"}}
	return export, state, proposal
}
func TestProjectExportDeterministicApprovedScopedEpistemic(t *testing.T) {
	export, state, _ := projectFixture(t)
	original := projectHash(state)
	unrelated := state.Processes[0]
	unrelated.ID = uuid.New()
	unrelated.Title = "UNRELATED PRIVATE PROCESS"
	state.Processes = append(state.Processes, unrelated)
	rejected := state.Processes[0].Hypotheses[0]
	rejected.ID = uuid.New()
	rejected.ApprovalStatus = "rejected"
	rejected.Statement = "REJECTED SECRET"
	state.Processes[0].Hypotheses = append(state.Processes[0].Hypotheses, rejected)
	state.OpenProposals = []Diff{{Status: "pending_review"}}
	again, err := BuildProjectExport(export.TenantID, export.ClientID, export.GeneratedBy, state, export.Snapshot.SelectedProcess.ID, export.Snapshot.ApproachRegistry, export.Snapshot.TechniqueRegistry)
	if err != nil {
		t.Fatal(err)
	}
	if export.ContentHash != again.ContentHash || export.Markdown != again.Markdown || !reflect.DeepEqual(export.Sources, again.Sources) {
		t.Fatal("export is not stable or includes unrelated/rejected data")
	}
	if !strings.Contains(again.Markdown, "PATIENT_REPORT") || !strings.Contains(again.Markdown, "CLINICAL_HYPOTHESIS") || again.StateVersion != 5 {
		t.Fatal("epistemic labels or state version lost")
	}
	if strings.Contains(again.Markdown, export.ClientID.String()) || strings.Contains(again.Markdown, export.Snapshot.SelectedProcess.ID.String()) {
		t.Fatal("internal identity exported")
	}
	if original == "" {
		t.Fatal("missing fingerprint")
	}
	before := projectHash(state)
	_, _ = BuildProjectExport(export.TenantID, export.ClientID, export.GeneratedBy, state, export.Snapshot.SelectedProcess.ID, nil, nil)
	if before != projectHash(state) {
		t.Fatal("export mutated caller state")
	}
}
func TestProjectImportStrategyAndUnsafeInputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ClinicalProjectImportV1)
	}{
		{"valid", func(*ClinicalProjectImportV1) {}},
		{"unknown_ref", func(p *ClinicalProjectImportV1) { p.Strategy.Targets[0].EvidenceRefs = []string{"evidence_unknown"} }},
		{"foreign_uuid", func(p *ClinicalProjectImportV1) { p.Strategy.Targets[0].EvidenceRefs = []string{uuid.NewString()} }},
		{"unsupported_mechanism", func(p *ClinicalProjectImportV1) { p.Strategy.Rationales[0].ApproachSlug = "invented_neural_mechanism" }},
		{"achieve_goal", func(p *ClinicalProjectImportV1) {
			p.Operations = []PortableProjectOperation{{OperationType: "achieve_goal"}}
		}},
		{"complete_phase", func(p *ClinicalProjectImportV1) {
			p.Operations = []PortableProjectOperation{{OperationType: "complete_gira_phase"}}
		}},
		{"overwrite_gira", func(p *ClinicalProjectImportV1) {
			p.Operations = []PortableProjectOperation{{OperationType: "update_gira"}}
		}},
		{"rewrite_evidence", func(p *ClinicalProjectImportV1) {
			p.Operations = []PortableProjectOperation{{OperationType: "update_evidence"}}
		}},
		{"bad_hash", func(p *ClinicalProjectImportV1) { p.SourceExportHash = strings.Repeat("0", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, s, p := projectFixture(t)
			before := projectHash(s)
			tc.mutate(&p)
			out, err := ValidateProjectImport(e, s, p)
			if tc.name == "valid" {
				if err != nil || len(out.Operations) < 6 {
					t.Fatalf("valid strategy: %v", err)
				}
			} else if err == nil {
				t.Fatal("unsafe proposal accepted")
			}
			if before != projectHash(s) {
				t.Fatal("import mutated state")
			}
		})
	}
}
func TestProjectImportStrictAndStale(t *testing.T) {
	e, s, p := projectFixture(t)
	for _, raw := range []string{`{"x":1,"x":2}`, `{"proposal":{"evidence_refs":[],"evidence_refs":["other"]}}`} {
		if _, err := DecodeProjectImport([]byte(raw)); err == nil {
			t.Fatal("duplicate JSON properties accepted")
		}
	}
	raw, _ := json.Marshal(p)
	for _, raw := range [][]byte{[]byte(`{`), []byte(`null`), append(raw, []byte(` {}`)...), []byte(strings.Replace(string(raw), `"schema_version":`, `"unknown":true,"schema_version":`, 1)), []byte(strings.Replace(string(raw), `"title":"Ruta"`, `"clinical_status":"completed","title":"Ruta"`, 1))} {
		if _, err := DecodeProjectImport(raw); err == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
	if _, err := DecodeProjectImport(raw); err != nil {
		t.Fatal(err)
	}
	s.StateVersion++
	if _, err := ValidateProjectImport(e, s, p); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("stale accepted: %v", err)
	}
	s.StateVersion--
	s.ClientID = uuid.New()
	if _, err := ValidateProjectImport(e, s, p); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatal("cross-client accepted")
	}
}
func TestProjectImportPortableProcessHypothesis(t *testing.T) {
	e, s, p := projectFixture(t)
	p.Strategy = nil
	p.Operations = []PortableProjectOperation{
		{ID: "op_1", OperationType: "create_process", TargetRef: "new_process", SourceRefs: []string{"evidence_1"}, Rationale: "Synthetic rationale", Proposal: json.RawMessage(`{"title":"Possible process","description":"Exploratory process","clinical_status":"observing","evidence_refs":["evidence_1"]}`)},
		{ID: "op_2", OperationType: "create_hypothesis", TargetRef: "new_hypothesis", SourceRefs: []string{"evidence_1"}, Rationale: "Possible interpretation", Proposal: json.RawMessage(`{"process_ref":"new_process","statement":"Competing explanation","hypothesis_type":null,"confidence_level":"yellow","supporting_evidence_refs":["evidence_1"],"contradicting_evidence_refs":[]}`)},
	}
	out, err := ValidateProjectImport(e, s, p)
	if err != nil || len(out.Operations) != 2 {
		t.Fatalf("portable import: %v", err)
	}
	p.Operations[0].Proposal = json.RawMessage(`{"title":"x","description":"x","clinical_status":"observing","evidence_ids":["` + s.ActiveEvidence[0].ID.String() + `"]}`)
	if _, err = ValidateProjectImport(e, s, p); err == nil {
		t.Fatal("UUID bypass accepted")
	}
}
func TestProjectExportSyntheticPII(t *testing.T) {
	e, s, _ := projectFixture(t)
	s.ActiveEvidence[0].Statement = "Paciente Ana Perez correo ana@example.test teléfono +54 11 5555 1212 DNI TEST-DNI-XYZ Calle Falsa 123; pareja Carlos Gomez"
	s.Processes[0].Hypotheses[0].SupportingEvidence[0] = s.ActiveEvidence[0]
	out, err := BuildProjectExport(e.TenantID, e.ClientID, e.GeneratedBy, s, e.Snapshot.SelectedProcess.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"Ana Perez", "ana@example.test", "5555", "TEST-DNI-XYZ", "Falsa 123", "Carlos Gomez"} {
		if strings.Contains(out.Markdown, secret) {
			t.Fatalf("PII retained: %s", secret)
		}
	}
}

func TestProjectExportRejectsNestedForeignSource(t *testing.T) {
	for _, field := range []string{"tenant", "client"} {
		t.Run(field, func(t *testing.T) {
			e, s, _ := projectFixture(t)
			if field == "tenant" {
				s.Processes[0].Hypotheses[0].SupportingEvidence[0].TenantID = uuid.New()
			} else {
				s.Processes[0].Hypotheses[0].SupportingEvidence[0].ClientID = uuid.New()
			}
			if _, err := BuildProjectExport(e.TenantID, e.ClientID, e.GeneratedBy, s, e.Snapshot.SelectedProcess.ID, nil, nil); err == nil {
				t.Fatal("foreign nested source accepted")
			}
		})
	}
}
