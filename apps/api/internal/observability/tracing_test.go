package observability

import (
	"context"
	"testing"

	"sessionflow/apps/api/internal/config"
)

func TestNewTracerProviderWithoutExporter(t *testing.T) {
	cfg := config.Config{
		OTELServiceName:    "sessionflow-test",
		OTELTracesExporter: "none",
	}

	tp, shutdown, err := NewTracerProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new tracer provider: %v", err)
	}
	if tp == nil {
		t.Fatalf("expected tracer provider")
	}
	if shutdown == nil {
		t.Fatalf("expected shutdown func")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown tracer provider: %v", err)
	}
}

func TestNewTracerProviderRequiresEndpointForOTLP(t *testing.T) {
	cfg := config.Config{
		OTELServiceName:    "sessionflow-test",
		OTELTracesExporter: "otlp",
	}

	tp, shutdown, err := NewTracerProvider(context.Background(), cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if tp != nil || shutdown != nil {
		t.Fatalf("expected nil provider and shutdown on error")
	}
}
