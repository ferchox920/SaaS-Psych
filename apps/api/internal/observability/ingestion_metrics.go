package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type IngestionMetrics struct {
	jobs     *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewIngestionMetrics(reg prometheus.Registerer) (*IngestionMetrics, error) {
	m := &IngestionMetrics{jobs: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "clinical_job_total", Help: "Local durable job attempt outcomes."}, []string{"type", "status"}), duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "clinical_job_duration_seconds", Help: "Local durable job attempt duration.", Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 1500}}, []string{"type"})}
	if err := reg.Register(m.jobs); err != nil {
		return nil, err
	}
	if err := reg.Register(m.duration); err != nil {
		return nil, err
	}
	return m, nil
}
func (m *IngestionMetrics) RecordIngestionJob(kind, status string, d time.Duration) {
	switch kind {
	case "transcribe_session_audio", "analyze_session", "artifact_cleanup":
	default:
		kind = "unknown"
	}
	switch status {
	case "succeeded", "failed", "cancelled":
	default:
		status = "unknown"
	}
	m.jobs.WithLabelValues(kind, status).Inc()
	m.duration.WithLabelValues(kind).Observe(d.Seconds())
}
