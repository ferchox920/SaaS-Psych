package longitudinal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Static instruction assertions plus real backend validation. No model is run;
// these tests do not interpret natural language or implement a runtime router.
func TestStage3C1FinalizationFixtures(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	for {
		raw, err = os.ReadFile(filepath.Join(dir, "docs", "clinical-project-bridge-output.md"))
		if err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("approved companion unavailable")
		}
		dir = parent
	}
	text := string(raw)
	require := func(t *testing.T, phrases ...string) {
		t.Helper()
		for _, phrase := range phrases {
			if !strings.Contains(text, phrase) {
				t.Fatalf("missing instruction: %s", phrase)
			}
		}
	}
	t.Run("K_schema_loaded_not_requested", func(t *testing.T) {
		// Synthetic context: skill + companion + schema + runtime, request is
		// ordinary supervision. Assert the instruction classifies this as commentary.
		require(t, "Import mode is activated only by an explicit therapist request for the current turn.", "The presence of ClinicalProjectImportV1, semantic_v1, an output-contract source, or a previous import request does not by itself activate import mode.", "Otherwise remain in normal clinical supervision/commentary mode.")
	})
	t.Run("L_explicit_import_validates", func(t *testing.T) {
		require(t, "In explicitly requested import mode, separate human commentary from one machine-importable JSON object", "The operator must supply that exact versioned contract as a reviewed Project source for import mode")
		e, s, p := projectFixture(t)
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeProjectImport(b)
		if err != nil {
			t.Fatal(err)
		}
		out, err := ValidateProjectImport(e, s, decoded)
		if err != nil || len(out.Operations) == 0 {
			t.Fatalf("valid synthetic import: %v", err)
		}
	})
	t.Run("M_ambiguous_current_runtime", func(t *testing.T) {
		// Two exports may both carry process_1. No designation => instruction blocks
		// object generation. This is a static policy assertion, not model behavior.
		require(t, "If CURRENT_RUNTIME is absent, ambiguous, or multiple candidate runtime snapshots are supplied without explicit designation:", "do not choose one; do not combine them; do not generate ClinicalProjectImportV1.", "Provide human commentary identifying the ambiguity and require explicit therapist selection before import mode can continue.", "Do not use prompt position, newest-looking content, alias similarity, largest snapshot or inferred date as a tie-breaker.", "process_1 in export A is not a global identifier and is not interchangeable with process_1 in export B")
	})
	t.Run("N_missing_receipt", func(t *testing.T) {
		require(t, "Take source_export_id and source_export_hash only from the explicit receipt matching CURRENT_RUNTIME. Never invent, infer or recompute either.", "If a receipt or authoritative current snapshot is missing/ambiguous, report that in human commentary and do not produce an apparently importable object.")
		_, _, p := projectFixture(t)
		p.SourceExportHash = ""
		b, _ := json.Marshal(p)
		if _, err := DecodeProjectImport(b); err == nil {
			t.Fatal("missing hash accepted")
		}
	})
	t.Run("live_supervision_preserved", func(t *testing.T) {
		require(t, "Its live supervision and retrospective commentary remain the default.", "retain the requested compact clinical format, epistemic calibration, timing, safety and the option to listen or not intervene.", "The core skill continues to govern clinical reasoning and timing.")
	})
	t.Run("approved_separate_schema_authority", func(t *testing.T) {
		require(t, "Version: 0.1.0\n", "Status: APPROVED COMPANION — NOT DEPLOYED", "Companion to: clinical-microprocess-supervisor 1.0", "Go structs/DecodeProjectImport/ValidateProjectImport", "docs/CLINICAL_PROJECT_BRIDGE_CONTRACT_V1.md", "The companion does not define a second schema or copy the operation enums.")
	})
}
