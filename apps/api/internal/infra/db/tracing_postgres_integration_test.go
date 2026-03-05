package db

import (
	"context"
	"os"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestPostgresTracingEmitsSpanPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run postgres integration tracing test")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
	}

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() {
		if err := tp.Shutdown(t.Context()); err != nil {
			t.Fatalf("shutdown tracer provider: %v", err)
		}
	}()

	pool, err := NewPostgresPoolWithTracing(context.Background(), databaseURL, PoolTracingConfig{
		Tracer:             tp.Tracer("test-db"),
		DBStatementEnabled: false,
	})
	if err != nil {
		t.Fatalf("create postgres pool with tracing: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(context.Background(), "select 1"); err != nil {
		t.Fatalf("exec simple query: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatalf("expected at least one db span")
	}

	var foundDBSystem bool
	for _, attr := range spans[0].Attributes() {
		if string(attr.Key) == "db.system" && attr.Value.AsString() == "postgresql" {
			foundDBSystem = true
			break
		}
	}
	if !foundDBSystem {
		t.Fatalf("expected db.system=postgresql attribute in first span")
	}
}
