package observability

import (
	"errors"

	domainerrors "sessionflow/apps/api/internal/domain/errors"

	"github.com/prometheus/client_golang/prometheus"
)

type DomainMetrics struct {
	appointmentsCreated  *prometheus.CounterVec
	appointmentsCanceled *prometheus.CounterVec
	authErrors           *prometheus.CounterVec
}

func NewDomainMetrics(registry prometheus.Registerer) (*DomainMetrics, error) {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	appointmentsCreated := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "appointments_created_total",
			Help: "Total appointment create attempts grouped by outcome.",
		},
		[]string{"result", "reason"},
	)

	appointmentsCanceled := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "appointments_canceled_total",
			Help: "Total appointment cancel attempts grouped by outcome.",
		},
		[]string{"result", "reason"},
	)

	authErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_errors_total",
			Help: "Total auth errors grouped by endpoint and reason.",
		},
		[]string{"endpoint", "reason"},
	)

	if err := registry.Register(appointmentsCreated); err != nil {
		return nil, err
	}
	if err := registry.Register(appointmentsCanceled); err != nil {
		return nil, err
	}
	if err := registry.Register(authErrors); err != nil {
		return nil, err
	}

	return &DomainMetrics{
		appointmentsCreated:  appointmentsCreated,
		appointmentsCanceled: appointmentsCanceled,
		authErrors:           authErrors,
	}, nil
}

func (m *DomainMetrics) RecordAppointmentCreated(err error) {
	if m == nil {
		return
	}
	m.appointmentsCreated.WithLabelValues(resultLabel(err), reasonLabel(err)).Inc()
}

func (m *DomainMetrics) RecordAppointmentCanceled(err error) {
	if m == nil {
		return
	}
	m.appointmentsCanceled.WithLabelValues(resultLabel(err), reasonLabel(err)).Inc()
}

func (m *DomainMetrics) RecordAuthError(endpoint string, err error) {
	if m == nil || err == nil {
		return
	}
	m.authErrors.WithLabelValues(endpoint, reasonLabel(err)).Inc()
}

func resultLabel(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}

func reasonLabel(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, domainerrors.ErrValidation):
		return "validation"
	case errors.Is(err, domainerrors.ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, domainerrors.ErrForbidden):
		return "forbidden"
	case errors.Is(err, domainerrors.ErrConflict):
		return "conflict"
	case errors.Is(err, domainerrors.ErrNotFound):
		return "not_found"
	default:
		return "internal"
	}
}
