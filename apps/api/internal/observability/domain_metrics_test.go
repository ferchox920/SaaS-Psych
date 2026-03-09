package observability

import (
	"testing"

	domainerrors "sessionflow/apps/api/internal/domain/errors"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestDomainMetricsRecordBusinessCounters(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewDomainMetrics(registry)
	if err != nil {
		t.Fatalf("new domain metrics: %v", err)
	}

	metrics.RecordAppointmentCreated(nil)
	metrics.RecordAppointmentCreated(domainerrors.ErrConflict)
	metrics.RecordAppointmentCanceled(nil)
	metrics.RecordAppointmentCanceled(domainerrors.NewValidation("missing tenant"))
	metrics.RecordAuthError("login", domainerrors.ErrUnauthorized)
	metrics.RecordAuthError("refresh", domainerrors.NewValidation("missing refresh token"))

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	created := findFamily(t, families, "appointments_created_total")
	if created.GetType() != dto.MetricType_COUNTER {
		t.Fatalf("expected counter type, got %v", created.GetType())
	}
	if !hasLabelSet(created.GetMetric(), map[string]string{"result": "success", "reason": "none"}) {
		t.Fatalf("expected appointments_created_total success labels")
	}
	if !hasLabelSet(created.GetMetric(), map[string]string{"result": "error", "reason": "conflict"}) {
		t.Fatalf("expected appointments_created_total conflict labels")
	}

	canceled := findFamily(t, families, "appointments_canceled_total")
	if !hasLabelSet(canceled.GetMetric(), map[string]string{"result": "success", "reason": "none"}) {
		t.Fatalf("expected appointments_canceled_total success labels")
	}
	if !hasLabelSet(canceled.GetMetric(), map[string]string{"result": "error", "reason": "validation"}) {
		t.Fatalf("expected appointments_canceled_total validation labels")
	}

	authErrors := findFamily(t, families, "auth_errors_total")
	if !hasLabelSet(authErrors.GetMetric(), map[string]string{"endpoint": "login", "reason": "unauthorized"}) {
		t.Fatalf("expected auth_errors_total login unauthorized labels")
	}
	if !hasLabelSet(authErrors.GetMetric(), map[string]string{"endpoint": "refresh", "reason": "validation"}) {
		t.Fatalf("expected auth_errors_total refresh validation labels")
	}
}
