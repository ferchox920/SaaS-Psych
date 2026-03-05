package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")

	cfg := Load()

	if cfg.AppEnv != "local" {
		t.Fatalf("expected APP_ENV local, got %q", cfg.AppEnv)
	}
	if cfg.HTTPPort != "8080" {
		t.Fatalf("expected HTTP_PORT 8080, got %q", cfg.HTTPPort)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("expected empty DATABASE_URL, got %q", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "" {
		t.Fatalf("expected empty REDIS_URL, got %q", cfg.RedisURL)
	}
	if cfg.JWTAccessSecret != "change-me" {
		t.Fatalf("expected default JWT_ACCESS_SECRET, got %q", cfg.JWTAccessSecret)
	}
	if cfg.AccessTTLMin != 15 {
		t.Fatalf("expected default ACCESS_TTL_MIN 15, got %d", cfg.AccessTTLMin)
	}
	if cfg.RefreshTTLDays != 30 {
		t.Fatalf("expected default REFRESH_TTL_DAYS 30, got %d", cfg.RefreshTTLDays)
	}
	if cfg.RateLimitLoginPerMin != 10 {
		t.Fatalf("expected default RATE_LIMIT_LOGIN_PER_MIN 10, got %d", cfg.RateLimitLoginPerMin)
	}
	if cfg.OTELServiceName != "sessionflow-api" {
		t.Fatalf("expected default OTEL_SERVICE_NAME sessionflow-api, got %q", cfg.OTELServiceName)
	}
	if cfg.OTLPEndpoint != "" {
		t.Fatalf("expected empty OTEL_EXPORTER_OTLP_ENDPOINT, got %q", cfg.OTLPEndpoint)
	}
	if cfg.OTELTracesExporter != "none" {
		t.Fatalf("expected default OTEL_TRACES_EXPORTER none, got %q", cfg.OTELTracesExporter)
	}
	if cfg.OTELResourceAttrs != "" {
		t.Fatalf("expected empty OTEL_RESOURCE_ATTRIBUTES, got %q", cfg.OTELResourceAttrs)
	}
	if cfg.OTELDBStatement {
		t.Fatalf("expected default OTEL_DB_STATEMENT_ENABLED false, got true")
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("HTTP_PORT", "9000")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_ACCESS_SECRET", "access-secret")
	t.Setenv("ACCESS_TTL_MIN", "20")
	t.Setenv("REFRESH_TTL_DAYS", "14")
	t.Setenv("RATE_LIMIT_LOGIN_PER_MIN", "7")
	t.Setenv("OTEL_SERVICE_NAME", "sessionflow-test")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "deployment.environment=dev,team=platform")
	t.Setenv("OTEL_DB_STATEMENT_ENABLED", "true")

	cfg := Load()

	if cfg.AppEnv != "dev" {
		t.Fatalf("expected APP_ENV dev, got %q", cfg.AppEnv)
	}
	if cfg.HTTPPort != "9000" {
		t.Fatalf("expected HTTP_PORT 9000, got %q", cfg.HTTPPort)
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("expected DATABASE_URL postgres://example, got %q", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://localhost:6379" {
		t.Fatalf("expected REDIS_URL redis://localhost:6379, got %q", cfg.RedisURL)
	}
	if cfg.JWTAccessSecret != "access-secret" {
		t.Fatalf("expected JWT_ACCESS_SECRET access-secret, got %q", cfg.JWTAccessSecret)
	}
	if cfg.AccessTTLMin != 20 {
		t.Fatalf("expected ACCESS_TTL_MIN 20, got %d", cfg.AccessTTLMin)
	}
	if cfg.RefreshTTLDays != 14 {
		t.Fatalf("expected REFRESH_TTL_DAYS 14, got %d", cfg.RefreshTTLDays)
	}
	if cfg.RateLimitLoginPerMin != 7 {
		t.Fatalf("expected RATE_LIMIT_LOGIN_PER_MIN 7, got %d", cfg.RateLimitLoginPerMin)
	}
	if cfg.OTELServiceName != "sessionflow-test" {
		t.Fatalf("expected OTEL_SERVICE_NAME sessionflow-test, got %q", cfg.OTELServiceName)
	}
	if cfg.OTLPEndpoint != "http://localhost:4317" {
		t.Fatalf("expected OTEL_EXPORTER_OTLP_ENDPOINT http://localhost:4317, got %q", cfg.OTLPEndpoint)
	}
	if cfg.OTELTracesExporter != "otlp" {
		t.Fatalf("expected OTEL_TRACES_EXPORTER otlp, got %q", cfg.OTELTracesExporter)
	}
	if cfg.OTELResourceAttrs != "deployment.environment=dev,team=platform" {
		t.Fatalf("expected OTEL_RESOURCE_ATTRIBUTES deployment.environment=dev,team=platform, got %q", cfg.OTELResourceAttrs)
	}
	if !cfg.OTELDBStatement {
		t.Fatalf("expected OTEL_DB_STATEMENT_ENABLED true, got false")
	}
}
