package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestClinicalMetricsExposeOnlyBoundedOperationalLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewClinicalMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	metrics.RecordClinicalAnalysis("live", time.Second, 5*time.Second, 120, 12.5, false, 850, nil)
	metrics.RecordSuggestionDecision("corrected")
	metrics.RecordTranscription("success", "none", "faster-whisper", "medium", 5.6, 0.4)
	metrics.RecordLongitudinalAnalysis("success", 3*time.Second)
	metrics.RecordClinicalDiffCreated("success")
	metrics.RecordClinicalDiffDecision("modified", "success")
	metrics.RecordClinicalDiffMerge("conflict")
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	analysis := findFamily(t, families, "clinical_ai_analysis_total")
	if !hasLabelSet(analysis.GetMetric(), map[string]string{"mode": "live", "result": "success", "reason": "none", "repaired": "false"}) {
		t.Fatal("missing analysis labels")
	}
	decision := findFamily(t, families, "clinical_ai_suggestion_decisions_total")
	if !hasLabelSet(decision.GetMetric(), map[string]string{"disposition": "corrected"}) {
		t.Fatal("missing decision labels")
	}
	if !hasLabelSet(findFamily(t, families, "clinical_diff_operation_decision_total").GetMetric(), map[string]string{"decision": "modified", "result": "success"}) {
		t.Fatal("missing bounded diff decision labels")
	}
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "tenant" || label.GetName() == "patient" || label.GetName() == "appointment" || label.GetName() == "text" {
					t.Fatalf("sensitive metric label: %s", label.GetName())
				}
			}
		}
	}
}
