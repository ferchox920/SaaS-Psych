package observability

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func NewHTTPMetrics(registry prometheus.Registerer) (*HTTPMetrics, error) {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_total",
			Help: "Total HTTP requests handled by SessionFlow.",
		},
		[]string{"method", "path", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	if err := registry.Register(requestsTotal); err != nil {
		return nil, err
	}
	if err := registry.Register(requestDuration); err != nil {
		return nil, err
	}

	return &HTTPMetrics{
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
	}, nil
}

func (m *HTTPMetrics) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			path := c.Path()
			if path == "" {
				path = "unmatched"
			}
			method := c.Request().Method
			status := responseStatus(c)
			statusText := strconv.Itoa(status)

			m.requestsTotal.WithLabelValues(method, path, statusText).Inc()
			m.requestDuration.WithLabelValues(method, path, statusText).Observe(time.Since(start).Seconds())
			return nil
		}
	}
}

func responseStatus(c echo.Context) int {
	status := c.Response().Status
	if status == 0 {
		return 200
	}
	return status
}
