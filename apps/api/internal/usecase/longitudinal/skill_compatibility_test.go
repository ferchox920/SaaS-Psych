package longitudinal

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// These are deterministic data/contract checks, not executions or certification
// of a clinical model. The instruction audit and expected human responses live
// in the Stage3C1 report; no provider is invoked here.
func TestStage3C1InstructionBoundaryFixtures(t *testing.T) {
	t.Run("A_epistemic_labels", func(t *testing.T) {
		e, s, _ := projectFixture(t)
		inference := s.ActiveEvidence[0]
		inference.ID = uuid.New()
		inference.EpistemicType = "inference"
		inference.Statement = "Possible interpretation, not a fact"
		s.ActiveEvidence = append(s.ActiveEvidence, inference)
		s.Processes[0].Hypotheses[0].SupportingEvidence = append(s.Processes[0].Hypotheses[0].SupportingEvidence, inference)
		out, err := BuildProjectExport(e.TenantID, e.ClientID, e.GeneratedBy, s, s.Processes[0].ID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		labels := map[string]bool{}
		for _, v := range out.Artifact.EpistemicLabels {
			labels[v] = true
		}
		for _, v := range []string{"PATIENT_REPORT", "AI_INFERENCE", "CLINICAL_HYPOTHESIS"} {
			if !labels[v] {
				t.Fatal("lost label", v)
			}
		}
	})
	t.Run("B_contradiction", func(t *testing.T) {
		e, s, _ := projectFixture(t)
		v := s.ActiveEvidence[0]
		v.ID = uuid.New()
		v.Statement = "Synthetic contradictory observation"
		s.ActiveEvidence = append(s.ActiveEvidence, v)
		s.Processes[0].Hypotheses[0].ContradictingEvidence = []Evidence{v}
		out, err := BuildProjectExport(e.TenantID, e.ClientID, e.GeneratedBy, s, s.Processes[0].ID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		h := out.Artifact.Sources.Hypotheses[0]
		if len(h.SupportingEvidenceRefs) != 1 || len(h.ContradictingEvidenceRefs) != 1 || h.SupportingEvidenceRefs[0] == h.ContradictingEvidenceRefs[0] || h.ConfidenceLevel != "yellow" {
			t.Fatal("contradiction/confidence changed")
		}
	})
	t.Run("C_competing_hypotheses", func(t *testing.T) {
		e, s, _ := projectFixture(t)
		h := s.Processes[0].Hypotheses[0]
		h.ID = uuid.New()
		h.Statement = "Alternative plausible explanation"
		s.Processes[0].Hypotheses = append(s.Processes[0].Hypotheses, h)
		out, err := BuildProjectExport(e.TenantID, e.ClientID, e.GeneratedBy, s, s.Processes[0].ID, nil, nil)
		if err != nil || len(out.Artifact.Sources.Hypotheses) != 2 {
			t.Fatalf("competing hypotheses: %v", err)
		}
	})
	t.Run("D_patient_correction", func(t *testing.T) {
		e, s, _ := projectFixture(t)
		s.ActiveEvidence[0].Statement = "Correction: the proposed explanation does not describe my experience"
		s.Processes[0].Hypotheses[0].SupportingEvidence[0] = s.ActiveEvidence[0]
		out, err := BuildProjectExport(e.TenantID, e.ClientID, e.GeneratedBy, s, s.Processes[0].ID, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if out.Artifact.Sources.Evidence[0].EpistemicType != "patient_report" || !strings.Contains(out.Markdown, "Correction:") || strings.Contains(out.Markdown, "resistance") {
			t.Fatal("correction reframed")
		}
	})
	t.Run("E_missing_context", func(t *testing.T) {
		e, _, _ := projectFixture(t)
		found := false
		for _, x := range e.Artifact.RuntimeProfile.UnavailableSections {
			if x == "unselected_formulation" {
				found = true
			}
		}
		if !found {
			t.Fatal("missing formulation not explicit")
		}
	})
	t.Run("F_exact_refs", func(t *testing.T) {
		e, s, p := projectFixture(t)
		p.Strategy.Targets[0].EvidenceRefs = []string{"evidence_3"}
		if _, err := ValidateProjectImport(e, s, p); err == nil {
			t.Fatal("invented ref accepted")
		}
	})
	t.Run("G_cross_export_receipt", func(t *testing.T) {
		e, s, p := projectFixture(t)
		p.SourceExportID = uuid.New()
		if _, err := ValidateProjectImport(e, s, p); err == nil {
			t.Fatal("cross-export receipt accepted")
		}
	})
	t.Run("H_missing_receipt", func(t *testing.T) {
		_, _, p := projectFixture(t)
		p.SourceExportID = uuid.Nil
		p.SourceExportHash = ""
		raw, _ := json.Marshal(p)
		if _, err := DecodeProjectImport(raw); err == nil {
			t.Fatal("missing receipt accepted")
		}
	})
	t.Run("I_valid_import", func(t *testing.T) {
		e, s, p := projectFixture(t)
		raw, _ := json.Marshal(p)
		decoded, err := DecodeProjectImport(raw)
		if err != nil {
			t.Fatal(err)
		}
		out, err := ValidateProjectImport(e, s, decoded)
		if err != nil || len(out.Operations) == 0 {
			t.Fatalf("typed semantic import: %v", err)
		}
	})
	t.Run("J_unsupported_strategy_and_note_only", func(t *testing.T) {
		e, s, p := projectFixture(t)
		p.Strategy.Rationales[0].ApproachSlug = "unsupported_mechanism"
		if _, err := ValidateProjectImport(e, s, p); err == nil {
			t.Fatal("unsupported strategy accepted")
		}
		p.Strategy = nil
		p.Operations = nil
		p.OpenQuestions = []GIRASemanticUncertainty{{Type: "insufficient_evidence", Question: "Explore before making a strategy", EvidenceRefs: []string{"evidence_1"}}}
		if _, err := ValidateProjectImport(e, s, p); err == nil {
			t.Fatal("note-only empty diff accepted")
		}
	})
}
