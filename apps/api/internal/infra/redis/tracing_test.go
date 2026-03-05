package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestLoginRateLimitStoreIncrementCreatesSpan(t *testing.T) {
	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mini.Close()

	client := goredis.NewClient(&goredis.Options{Addr: mini.Addr()})
	defer client.Close()

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() {
		if err := tp.Shutdown(t.Context()); err != nil {
			t.Fatalf("shutdown tracer provider: %v", err)
		}
	}()

	store := NewLoginRateLimitStoreWithTracer(client, tp.Tracer("test-redis"))
	if _, err := store.Increment(context.Background(), "ratelimit:auth_login:ip:test", time.Minute); err != nil {
		t.Fatalf("increment rate limit key: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatalf("expected at least one redis span")
	}
	if spans[0].Name() != "redis.ratelimit.increment" {
		t.Fatalf("expected span name redis.ratelimit.increment, got %q", spans[0].Name())
	}

	var hasDBSystem bool
	var hasDBOperation bool
	for _, attr := range spans[0].Attributes() {
		if string(attr.Key) == "db.system" && attr.Value.AsString() == "redis" {
			hasDBSystem = true
		}
		if string(attr.Key) == "db.operation" && attr.Value.AsString() == "INCR+EXPIRENX" {
			hasDBOperation = true
		}
	}
	if !hasDBSystem {
		t.Fatalf("expected db.system=redis attribute")
	}
	if !hasDBOperation {
		t.Fatalf("expected db.operation=INCR+EXPIRENX attribute")
	}
}
