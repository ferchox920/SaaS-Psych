package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ClinicalMetrics intentionally excludes tenant, patient, appointment and text
// labels. It exposes operational aggregates without clinical content.
type ClinicalMetrics struct {
	analysisTotal        *prometheus.CounterVec
	firstTokenSeconds    *prometheus.HistogramVec
	generationSeconds    *prometheus.HistogramVec
	evalTokens           *prometheus.HistogramVec
	evalRate             *prometheus.HistogramVec
	contextCharacters    *prometheus.HistogramVec
	suggestionDecisions  *prometheus.CounterVec
	transcriptionTotal   *prometheus.CounterVec
	transcriptionRTF     *prometheus.HistogramVec
	transcriptionSeconds *prometheus.HistogramVec
	clinicalSessions     *prometheus.CounterVec
	aiRuns               *prometheus.CounterVec
	reportGenerations    *prometheus.CounterVec
	reportSeconds        prometheus.Histogram
	reportApprovals      *prometheus.CounterVec
	longitudinalAnalysis *prometheus.CounterVec
	longitudinalSeconds  prometheus.Histogram
	diffCreated          *prometheus.CounterVec
	diffMerges           *prometheus.CounterVec
	diffDecisions        *prometheus.CounterVec
	giraBuilds           *prometheus.CounterVec
	giraBuildSeconds     prometheus.Histogram
	giraStageSeconds     *prometheus.HistogramVec
	giraContextBytes     *prometheus.HistogramVec
	giraDiffOperations   *prometheus.CounterVec
	giraVersionsCreated  prometheus.Counter
	goalTransitions      *prometheus.CounterVec
}

func NewClinicalMetrics(registry prometheus.Registerer) (*ClinicalMetrics, error) {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}
	m := &ClinicalMetrics{
		analysisTotal:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "clinical_ai_analysis_total", Help: "Local clinical analysis attempts by mode, outcome and repair state."}, []string{"mode", "result", "reason", "repaired"}),
		firstTokenSeconds:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "clinical_ai_first_token_seconds", Help: "Time to first token for successful local clinical analysis.", Buckets: []float64{0.5, 1, 2, 4, 7, 12, 20, 40}}, []string{"mode"}),
		generationSeconds:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "clinical_ai_generation_seconds", Help: "Total local clinical generation time.", Buckets: []float64{1, 3, 7, 15, 30, 60, 120}}, []string{"mode"}),
		evalTokens:           prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "clinical_ai_eval_tokens", Help: "Generated token count without content.", Buckets: []float64{64, 128, 256, 384, 512, 768, 1024}}, []string{"mode"}),
		evalRate:             prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "clinical_ai_eval_tokens_per_second", Help: "Local generation throughput.", Buckets: []float64{2, 5, 8, 10, 12, 15, 20, 30}}, []string{"mode"}),
		contextCharacters:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "clinical_ai_context_characters", Help: "Input character count without content.", Buckets: []float64{250, 500, 1000, 2000, 4000, 8000, 12000, 24000}}, []string{"mode"}),
		suggestionDecisions:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "clinical_ai_suggestion_decisions_total", Help: "Human disposition of local clinical suggestions."}, []string{"disposition"}),
		transcriptionTotal:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "clinical_transcription_total", Help: "Ephemeral local transcription attempts by outcome."}, []string{"result", "reason"}),
		transcriptionRTF:     prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "clinical_transcription_real_time_factor", Help: "Local transcription real-time factor.", Buckets: []float64{0.1, 0.25, 0.5, 0.75, 1, 1.5, 2}}, []string{"engine", "model"}),
		transcriptionSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "clinical_transcription_seconds", Help: "Ephemeral local transcription duration.", Buckets: []float64{1, 2, 5, 10, 20, 40, 90}}, []string{"engine", "model"}),
		clinicalSessions:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "clinical_sessions_total", Help: "Clinical session lifecycle operations by action and outcome."}, []string{"action", "result", "reason"}),
		aiRuns:               prometheus.NewCounterVec(prometheus.CounterOpts{Name: "clinical_ai_runs_total", Help: "Durable clinical AI runs by operation and lifecycle state."}, []string{"operation", "status"}),
		reportGenerations:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "session_report_generation_total", Help: "Session report generation attempts."}, []string{"result"}),
		reportSeconds:        prometheus.NewHistogram(prometheus.HistogramOpts{Name: "session_report_generation_seconds", Help: "Session report generation duration.", Buckets: []float64{5, 15, 30, 60, 120, 240}}),
		reportApprovals:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "session_report_approvals_total", Help: "Session report approval attempts."}, []string{"result"}),
		longitudinalAnalysis: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "longitudinal_analysis_total", Help: "Longitudinal interpretation attempts by bounded outcome."}, []string{"result"}),
		longitudinalSeconds:  prometheus.NewHistogram(prometheus.HistogramOpts{Name: "longitudinal_analysis_duration_seconds", Help: "Longitudinal interpretation duration.", Buckets: []float64{5, 15, 30, 60, 120, 240}}),
		diffCreated:          prometheus.NewCounterVec(prometheus.CounterOpts{Name: "clinical_diff_created_total", Help: "Clinical diffs created by outcome."}, []string{"result"}),
		diffMerges:           prometheus.NewCounterVec(prometheus.CounterOpts{Name: "clinical_diff_merge_total", Help: "Clinical diff merge attempts by outcome."}, []string{"result"}),
		diffDecisions:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "clinical_diff_operation_decision_total", Help: "Clinical diff operation decisions."}, []string{"decision", "result"}),
		giraBuilds:           prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gira_build_total", Help: "GIRA builder attempts by bounded outcome."}, []string{"result"}),
		giraBuildSeconds:     prometheus.NewHistogram(prometheus.HistogramOpts{Name: "gira_build_duration_seconds", Help: "GIRA builder duration.", Buckets: []float64{5, 15, 30, 60, 120, 240}}),
		giraStageSeconds:     prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "gira_build_stage_duration_seconds", Help: "GIRA builder duration by bounded pipeline stage.", Buckets: []float64{0.001, 0.005, 0.02, 0.1, 0.5, 1, 5, 15, 30, 60, 120, 240}}, []string{"stage"}),
		giraContextBytes:     prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "gira_build_context_bytes", Help: "Serialized GIRA context size by bounded component without content labels.", Buckets: []float64{128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768}}, []string{"component"}),
		giraDiffOperations:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "gira_diff_operations_total", Help: "Strategy diff operations proposed by bounded operation type."}, []string{"operation"}),
		giraVersionsCreated:  prometheus.NewCounter(prometheus.CounterOpts{Name: "gira_version_created_total", Help: "GIRA versions created by an approved human merge."}),
		goalTransitions:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "goal_transition_total", Help: "Goal lifecycle transitions applied by an approved human merge."}, []string{"transition"}),
	}
	collectors := []prometheus.Collector{m.analysisTotal, m.firstTokenSeconds, m.generationSeconds, m.evalTokens, m.evalRate, m.contextCharacters, m.suggestionDecisions, m.transcriptionTotal, m.transcriptionRTF, m.transcriptionSeconds, m.clinicalSessions, m.aiRuns, m.reportGenerations, m.reportSeconds, m.reportApprovals, m.longitudinalAnalysis, m.longitudinalSeconds, m.diffCreated, m.diffMerges, m.diffDecisions, m.giraBuilds, m.giraBuildSeconds, m.giraStageSeconds, m.giraContextBytes, m.giraDiffOperations, m.giraVersionsCreated, m.goalTransitions}
	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			return nil, err
		}
	}
	return m, nil
}
func (m *ClinicalMetrics) RecordGIRABuild(result string, duration time.Duration) {
	if m != nil {
		m.giraBuilds.WithLabelValues(result).Inc()
		m.giraBuildSeconds.Observe(duration.Seconds())
	}
}
func (m *ClinicalMetrics) RecordGIRAStage(stage string, duration time.Duration) {
	if m != nil {
		m.giraStageSeconds.WithLabelValues(stage).Observe(duration.Seconds())
	}
}
func (m *ClinicalMetrics) RecordGIRAContextSize(component string, bytes int) {
	if m != nil {
		m.giraContextBytes.WithLabelValues(component).Observe(float64(bytes))
	}
}
func (m *ClinicalMetrics) RecordGIRADiffOperation(operation string) {
	if m != nil {
		m.giraDiffOperations.WithLabelValues(operation).Inc()
	}
}
func (m *ClinicalMetrics) RecordGIRAVersionCreated() {
	if m != nil {
		m.giraVersionsCreated.Inc()
	}
}
func (m *ClinicalMetrics) RecordGoalTransition(transition string) {
	if m != nil {
		m.goalTransitions.WithLabelValues(transition).Inc()
	}
}

func (m *ClinicalMetrics) RecordLongitudinalAnalysis(result string, duration time.Duration) {
	if m != nil {
		m.longitudinalAnalysis.WithLabelValues(result).Inc()
		m.longitudinalSeconds.Observe(duration.Seconds())
	}
}
func (m *ClinicalMetrics) RecordClinicalDiffCreated(result string) {
	if m != nil {
		m.diffCreated.WithLabelValues(result).Inc()
	}
}
func (m *ClinicalMetrics) RecordClinicalDiffDecision(decision, result string) {
	if m != nil {
		m.diffDecisions.WithLabelValues(decision, result).Inc()
	}
}
func (m *ClinicalMetrics) RecordClinicalDiffMerge(result string) {
	if m != nil {
		m.diffMerges.WithLabelValues(result).Inc()
	}
}

func (m *ClinicalMetrics) RecordClinicalSessionTransition(action string, err error) {
	if m != nil {
		m.clinicalSessions.WithLabelValues(action, resultLabel(err), reasonLabel(err)).Inc()
	}
}

func (m *ClinicalMetrics) RecordClinicalAIRun(operation, status string) {
	if m != nil {
		m.aiRuns.WithLabelValues(operation, status).Inc()
	}
}

func (m *ClinicalMetrics) RecordSessionReportGeneration(result string, duration time.Duration) {
	if m != nil {
		m.reportGenerations.WithLabelValues(result).Inc()
		m.reportSeconds.Observe(duration.Seconds())
	}
}
func (m *ClinicalMetrics) RecordSessionReportApproval(result string) {
	if m != nil {
		m.reportApprovals.WithLabelValues(result).Inc()
	}
}

func (m *ClinicalMetrics) RecordClinicalAnalysis(mode string, firstToken, total time.Duration, evalCount int, evalRate float64, repaired bool, contextChars int, err error) {
	if m == nil {
		return
	}
	repairedLabel := "false"
	if repaired {
		repairedLabel = "true"
	}
	m.analysisTotal.WithLabelValues(mode, resultLabel(err), reasonLabel(err), repairedLabel).Inc()
	m.contextCharacters.WithLabelValues(mode).Observe(float64(contextChars))
	if err == nil {
		m.firstTokenSeconds.WithLabelValues(mode).Observe(firstToken.Seconds())
		m.generationSeconds.WithLabelValues(mode).Observe(total.Seconds())
		m.evalTokens.WithLabelValues(mode).Observe(float64(evalCount))
		m.evalRate.WithLabelValues(mode).Observe(evalRate)
	}
}

func (m *ClinicalMetrics) RecordSuggestionDecision(disposition string) {
	if m != nil {
		m.suggestionDecisions.WithLabelValues(disposition).Inc()
	}
}

func (m *ClinicalMetrics) RecordTranscription(result, reason, engine, model string, seconds, realTimeFactor float64) {
	if m == nil {
		return
	}
	m.transcriptionTotal.WithLabelValues(result, reason).Inc()
	if result == "success" {
		m.transcriptionRTF.WithLabelValues(engine, model).Observe(realTimeFactor)
		m.transcriptionSeconds.WithLabelValues(engine, model).Observe(seconds)
	}
}
