package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"sessionflow/apps/api/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func NewTracerProvider(ctx context.Context, cfg config.Config) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	opts := []resource.Option{
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(semconv.ServiceName(cfg.OTELServiceName)),
	}

	if cfg.OTELResourceAttrs != "" {
		attrs, err := parseResourceAttributes(cfg.OTELResourceAttrs)
		if err != nil {
			return nil, nil, err
		}
		opts = append(opts, resource.WithAttributes(attrs...))
	}

	res, err := resource.New(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("create otel resource: %w", err)
	}

	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}

	switch strings.ToLower(cfg.OTELTracesExporter) {
	case "", "none":
		// keep provider active with no exporter; spans stay local/no-op.
	case "otlp":
		if cfg.OTLPEndpoint == "" {
			return nil, nil, errors.New("OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_TRACES_EXPORTER=otlp")
		}
		endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.OTLPEndpoint, "http://"), "https://")
		exp, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("create otlp trace exporter: %w", err)
		}
		tpOpts = append(tpOpts,
			sdktrace.WithBatcher(exp),
		)
	default:
		return nil, nil, fmt.Errorf("unsupported OTEL_TRACES_EXPORTER: %s", cfg.OTELTracesExporter)
	}

	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, tp.Shutdown, nil
}

func parseResourceAttributes(raw string) ([]attribute.KeyValue, error) {
	parts := strings.Split(raw, ",")
	attrs := make([]attribute.KeyValue, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items := strings.SplitN(part, "=", 2)
		if len(items) != 2 {
			return nil, fmt.Errorf("invalid OTEL_RESOURCE_ATTRIBUTES segment: %q", part)
		}
		key := strings.TrimSpace(items[0])
		val := strings.TrimSpace(items[1])
		if key == "" {
			return nil, fmt.Errorf("invalid OTEL_RESOURCE_ATTRIBUTES key in segment: %q", part)
		}
		attrs = append(attrs, attribute.String(key, val))
	}
	return attrs, nil
}
