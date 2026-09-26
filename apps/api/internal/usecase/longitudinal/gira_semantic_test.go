package longitudinal

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func semanticCompilerFixture() (GIRAGenerationRequest, GIRASemanticProposal) {
	processID, evidenceID := uuid.New(), uuid.New()
	technique := "behavioral_experiment"
	one := 1
	snapshot := GIRABuilderInput{
		SchemaVersion:     GIRAPromptVersion,
		SelectedProcess:   Process{ID: processID, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 3},
		ApprovedEvidence:  []Evidence{{ID: evidenceID, Status: "active", Version: 2, Statement: "Evitación mantiene alivio inmediato"}},
		CurrentStrategy:   TherapeuticStrategy{Targets: []Target{}, Goals: []Goal{}, Rationales: []TherapeuticRationale{}, GIRAs: []GIRA{}},
		ApproachRegistry:  []ApproachDefinition{{Slug: "cbt", Version: 1, Status: "active"}},
		TechniqueRegistry: []TechniqueDefinition{{Slug: technique, Version: one, ApproachSlug: "cbt", ApproachVersion: one, Status: "active"}},
	}
	proposal := GIRASemanticProposal{
		Targets:        []GIRASemanticTarget{{Ref: "target_1", Title: "Evitación", Description: "Evita la conversación", TargetType: "behavioral_pattern", EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}, EventRefs: []string{}}},
		Goals:          []GIRASemanticGoal{{Ref: "goal_1", Title: "Iniciar conversación", Description: "Inicia la conversación aun con malestar", GoalType: "behavior_change", Priority: "high", TargetRefs: []string{"target_1"}}},
		Indicators:     []GIRASemanticIndicator{{Ref: "indicator_1", GoalRef: "goal_1", Description: "Inicia la conversación acordada", IndicatorType: "qualitative"}},
		Rationales:     []GIRASemanticRationale{{Ref: "rationale_1", TargetRef: "target_1", GoalRef: "goal_1", ApproachSlug: "cbt", ApproachVersion: one, TechniqueSlug: &technique, TechniqueVersion: &one, Rationale: "Contrasta la predicción que mantiene evitación", ExpectedEffect: "Mayor aproximación funcional", EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}}},
		GIRA:           &GIRASemanticGIRA{Ref: "gira_1", Title: "Ruta", Summary: "Ruta conductual", TargetRefs: []string{"target_1"}, GoalRefs: []string{"goal_1"}, RationaleRefs: []string{"rationale_1"}},
		Phases:         []GIRASemanticPhase{{Ref: "phase_2", GIRARef: "gira_1", Position: 2, Title: "Consolidación", Description: "Consolidar", GoalRefs: []string{"goal_1"}, RationaleRefs: []string{"rationale_1"}, IndicatorRefs: []string{"indicator_1"}}, {Ref: "phase_1", GIRARef: "gira_1", Position: 1, Title: "Inicio", Description: "Iniciar", GoalRefs: []string{"goal_1"}, RationaleRefs: []string{"rationale_1"}, IndicatorRefs: []string{"indicator_1"}}},
		IndicatorLinks: []GIRASemanticIndicatorLink{}, Uncertainties: []GIRASemanticUncertainty{},
	}
	return BuildGIRAGenerationRequest(snapshot), proposal
}

func BenchmarkGIRALocalPipelineComponents(b *testing.B) {
	req, proposal := semanticCompilerFixture()
	raw, _ := json.Marshal(proposal)
	compiled, err := CompileGIRASemanticProposal(req, proposal, uuid.New)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("context_selection_minimization_alias_mapping", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = BuildGIRAGenerationRequest(req.Snapshot)
		}
	})
	b.Run("serialization", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := json.Marshal(req.RemoteContext); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("strict_decoder", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := DecodeGIRASemanticProposal(raw); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("deterministic_compiler", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := CompileGIRASemanticProposal(req, proposal, uuid.New); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("typed_validation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := ValidateGIRABuilderResult(req.Snapshot, compiled); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestBoundGIRAContextPrioritizesDirectSourcesAndCapsUnrelated(t *testing.T) {
	req, _ := semanticCompilerFixture()
	directID := uuid.New()
	req.Snapshot.CurrentStrategy.Targets = []Target{{ID: uuid.New(), EvidenceIDs: []uuid.UUID{directID}}}
	req.Snapshot.ApprovedEvidence = []Evidence{{ID: directID, Statement: "direct", UpdatedAt: time.Unix(1, 0)}}
	for i := 0; i < giraEvidenceLimit+8; i++ {
		req.Snapshot.ApprovedEvidence = append(req.Snapshot.ApprovedEvidence, Evidence{ID: uuid.New(), Statement: "unrelated", UpdatedAt: time.Unix(int64(100+i), 0)})
	}
	bounded := BoundGIRAContext(req.Snapshot)
	if len(bounded.ApprovedEvidence) != giraEvidenceLimit {
		t.Fatalf("evidence=%d", len(bounded.ApprovedEvidence))
	}
	if bounded.ApprovedEvidence[0].ID != directID {
		t.Fatal("direct evidence was not prioritized")
	}
}

func TestGIRAContextMeasurementHasClosedBreakdown(t *testing.T) {
	req, _ := semanticCompilerFixture()
	sizes := MeasureGIRAContext(req.RemoteContext)
	for _, component := range []string{"system_prompt", "process", "evidence", "events", "hypotheses", "strategy", "approaches", "techniques", "schema", "total"} {
		if sizes[component] <= 0 {
			t.Fatalf("missing size for %s: %v", component, sizes)
		}
	}
	oldInput, _ := json.Marshal(req.Snapshot)
	oldSchema, _ := json.Marshal(GIRAJSONSchema())
	newInput, _ := json.Marshal(req.RemoteContext)
	t.Logf("context_bytes old_input=%d old_schema=%d new_input=%d new_total=%d breakdown=%v", len(oldInput), len(oldSchema), len(newInput), sizes["total"], sizes)
}

func TestGIRASemanticDecoderRejectsUnknownFields(t *testing.T) {
	raw := []byte(`{"targets":[],"goals":[],"indicators":[],"rationales":[],"gira":null,"phases":[],"indicator_links":[],"uncertainties":[],"unexpected":true}`)
	if _, err := DecodeGIRASemanticProposal(raw); err == nil {
		t.Fatal("unknown field accepted")
	}
	valid := []byte(`{"targets":[],"goals":[],"indicators":[],"rationales":[],"gira":null,"phases":[],"indicator_links":[],"uncertainties":[]}`)
	if _, err := DecodeGIRASemanticProposal(append(valid, []byte(` trailing`)...)); err == nil {
		t.Fatal("trailing non-JSON content accepted")
	}
}

func TestGIRASemanticCompilerBuildsTopologicalOperationsAndBackendIDs(t *testing.T) {
	req, proposal := semanticCompilerFixture()
	next := 0
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	result, err := CompileGIRASemanticProposal(req, proposal, func() uuid.UUID { id := ids[next]; next++; return id })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"create_target", "create_goal", "create_goal_indicator", "create_therapeutic_rationale", "create_gira", "create_gira_phase", "create_gira_phase"}
	if len(result.Operations) != len(want) {
		t.Fatalf("operations=%d", len(result.Operations))
	}
	for i, kind := range want {
		if result.Operations[i].OperationType != kind {
			t.Fatalf("operation[%d]=%s", i, result.Operations[i].OperationType)
		}
		if result.Operations[i].TargetEntityID == nil || *result.Operations[i].TargetEntityID != ids[i] {
			t.Fatalf("backend id[%d] not used", i)
		}
	}
	var firstPhase CreateGIRAPhaseProposal
	if err := json.Unmarshal(result.Operations[5].Proposal, &firstPhase); err != nil {
		t.Fatal(err)
	}
	if firstPhase.Position != 1 {
		t.Fatalf("phase ordering=%d", firstPhase.Position)
	}
}

func TestGIRASemanticCompilerRejectsDuplicateAndUnknownRefs(t *testing.T) {
	for name, mutate := range map[string]func(*GIRASemanticProposal){
		"duplicate":      func(p *GIRASemanticProposal) { p.Goals[0].Ref = p.Targets[0].Ref },
		"unknown_target": func(p *GIRASemanticProposal) { p.Goals[0].TargetRefs = []string{"target_missing"} },
		"unknown_goal":   func(p *GIRASemanticProposal) { p.Indicators[0].GoalRef = "goal_missing" },
		"self_cycle":     func(p *GIRASemanticProposal) { p.Goals[0].TargetRefs = []string{"goal_1"} },
	} {
		t.Run(name, func(t *testing.T) {
			req, proposal := semanticCompilerFixture()
			mutate(&proposal)
			if _, err := CompileGIRASemanticProposal(req, proposal, uuid.New); err == nil {
				t.Fatal("malformed graph accepted")
			}
		})
	}
}

func TestGIRASemanticCompilerRejectsTechniqueMismatch(t *testing.T) {
	req, proposal := semanticCompilerFixture()
	proposal.Rationales[0].TechniqueSlug = stringPtr("unknown")
	if _, err := CompileGIRASemanticProposal(req, proposal, uuid.New); err == nil {
		t.Fatal("unknown technique accepted")
	}
}

func TestGIRASemanticCompilerRejectsEclecticStackOnSameTarget(t *testing.T) {
	req, proposal := semanticCompilerFixture()
	req.Snapshot.ApproachRegistry = append(req.Snapshot.ApproachRegistry, ApproachDefinition{Slug: "act", Version: 1, Status: "active"})
	technique, one := "values_committed_action", 1
	req.Snapshot.TechniqueRegistry = append(req.Snapshot.TechniqueRegistry, TechniqueDefinition{Slug: technique, Version: one, ApproachSlug: "act", ApproachVersion: one, Status: "active"})
	req = BuildGIRAGenerationRequest(req.Snapshot)
	proposal.Rationales = append(proposal.Rationales, GIRASemanticRationale{Ref: "rationale_2", TargetRef: "target_1", GoalRef: "goal_1", ApproachSlug: "act", ApproachVersion: one, TechniqueSlug: &technique, TechniqueVersion: &one, Rationale: "Una segunda modalidad sobre la misma función", ExpectedEffect: "Duplicado ecléctico", EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}})
	proposal.GIRA.RationaleRefs = append(proposal.GIRA.RationaleRefs, "rationale_2")
	if _, err := CompileGIRASemanticProposal(req, proposal, uuid.New); err == nil || !strings.Contains(err.Error(), "distinct targets") {
		t.Fatalf("eclectic stack error=%v", err)
	}
}

func TestGIRASemanticCompilerExistingRefsAndVersions(t *testing.T) {
	req, _ := semanticCompilerFixture()
	targetID, goalID, indicatorID := uuid.New(), uuid.New(), uuid.New()
	req.Snapshot.CurrentStrategy = TherapeuticStrategy{Targets: []Target{{ID: targetID, ProcessID: req.Snapshot.SelectedProcess.ID, Version: 4}}, Goals: []Goal{{ID: goalID, ProcessID: req.Snapshot.SelectedProcess.ID, Version: 2, TargetIDs: []uuid.UUID{targetID}, Indicators: []GoalIndicator{{ID: indicatorID, GoalID: goalID, Version: 7, Status: "active"}}}}, Rationales: []TherapeuticRationale{}, GIRAs: []GIRA{}}
	req = BuildGIRAGenerationRequest(req.Snapshot)
	proposal := GIRASemanticProposal{Targets: []GIRASemanticTarget{}, Goals: []GIRASemanticGoal{}, Indicators: []GIRASemanticIndicator{}, Rationales: []GIRASemanticRationale{}, Phases: []GIRASemanticPhase{}, IndicatorLinks: []GIRASemanticIndicatorLink{{IndicatorRef: "existing_indicator_1_1", SourceType: "evidence", SourceRef: "evidence_1", RelationType: "supports_progress", EvidenceRefs: []string{"evidence_1"}}}, Uncertainties: []GIRASemanticUncertainty{}}
	result, err := CompileGIRASemanticProposal(req, proposal, uuid.New)
	if err != nil {
		t.Fatal(err)
	}
	if result.Operations[0].ExpectedEntityVersion == nil || *result.Operations[0].ExpectedEntityVersion != 7 {
		t.Fatalf("expected version=%v", result.Operations[0].ExpectedEntityVersion)
	}
	repeated, err := CompileGIRASemanticProposal(req, proposal, uuid.New)
	if err != nil || repeated.Operations[0].ExpectedEntityVersion == nil || *repeated.Operations[0].ExpectedEntityVersion != 7 {
		t.Fatalf("compiler mutated reusable request: result=%#v err=%v", repeated, err)
	}
	proposal.IndicatorLinks[0].IndicatorRef = "existing_indicator_9_9"
	if _, err = CompileGIRASemanticProposal(req, proposal, uuid.New); err == nil || !strings.Contains(err.Error(), "unknown indicator") {
		t.Fatalf("unknown existing ref error=%v", err)
	}
}

func TestGIRASemanticCompilerSupersessionDerivesVersion(t *testing.T) {
	req, _ := semanticCompilerFixture()
	targetID, goalID, rationaleID, oldID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	req.Snapshot.CurrentStrategy = TherapeuticStrategy{Targets: []Target{{ID: targetID, ProcessID: req.Snapshot.SelectedProcess.ID, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1}}, Goals: []Goal{{ID: goalID, ProcessID: req.Snapshot.SelectedProcess.ID, ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1, Indicators: []GoalIndicator{{ID: uuid.New(), GoalID: goalID, Status: "active", Version: 1}}}}, Rationales: []TherapeuticRationale{{ID: rationaleID, ProcessID: req.Snapshot.SelectedProcess.ID, TargetID: targetID, GoalID: goalID, ApproachSlug: "cbt", ApproachVersion: 1, ApprovalStatus: "approved", GroundingStatus: "grounded", Version: 1}}, GIRAs: []GIRA{{ID: oldID, ProcessID: req.Snapshot.SelectedProcess.ID, GIRAVersion: 4, ApprovalStatus: "approved", ClinicalStatus: "active", EntityVersion: 2, TargetIDs: []uuid.UUID{targetID}, GoalIDs: []uuid.UUID{goalID}, Rationales: []TherapeuticRationale{{ID: rationaleID}}}}}
	req = BuildGIRAGenerationRequest(req.Snapshot)
	oldRef := "existing_gira_1"
	proposal := GIRASemanticProposal{Targets: []GIRASemanticTarget{}, Goals: []GIRASemanticGoal{}, Indicators: []GIRASemanticIndicator{}, Rationales: []GIRASemanticRationale{}, GIRA: &GIRASemanticGIRA{Ref: "gira_1", Title: "v5", Summary: "supersession", SupersedesGIRARef: &oldRef, TargetRefs: []string{"existing_target_1"}, GoalRefs: []string{"existing_goal_1"}, RationaleRefs: []string{"existing_rationale_1"}}, Phases: []GIRASemanticPhase{}, IndicatorLinks: []GIRASemanticIndicatorLink{}, Uncertainties: []GIRASemanticUncertainty{}}
	result, err := CompileGIRASemanticProposal(req, proposal, uuid.New)
	if err != nil {
		t.Fatal(err)
	}
	var compiled CreateGIRAProposal
	if err := json.Unmarshal(result.Operations[0].Proposal, &compiled); err != nil {
		t.Fatal(err)
	}
	if compiled.GIRAVersion != 5 || compiled.SupersedesGIRAID == nil || *compiled.SupersedesGIRAID != oldID {
		t.Fatalf("compiled=%#v", compiled)
	}
}
