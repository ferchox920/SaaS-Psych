package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("APP_VERSION", "")
	t.Setenv("BUILD_REVISION", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")
	t.Setenv("OTEL_DB_STATEMENT_ENABLED", "")

	cfg := Load()

	if cfg.AppEnv != "local" {
		t.Fatalf("expected APP_ENV local, got %q", cfg.AppEnv)
	}
	if cfg.AppVersion != "development" || cfg.BuildRevision != "unknown" {
		t.Fatalf("safe build provenance defaults missing: version=%q revision=%q", cfg.AppVersion, cfg.BuildRevision)
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
	if cfg.ClinicalGIRAProvider != "none" {
		t.Fatalf("remote GIRA must be disabled by default, got %q", cfg.ClinicalGIRAProvider)
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

func TestValidateOpenAIGIRARequiresExplicitCredentialsAndModel(t *testing.T) {
	cfg := validTestConfig()
	cfg.ClinicalGIRAProvider = "openai"
	cfg.ExperimentalRemoteGIRA = true
	cfg.OpenAIBaseURL = "https://api.openai.com"
	cfg.OpenAIGIRATimeoutSeconds = 180
	cfg.OpenAIGIRAMaxOutputTokens = 2048
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "CLINICAL_GIRA_MODEL") {
		t.Fatalf("expected missing model rejection, got %v", err)
	}
	cfg.ClinicalGIRAModel = "gpt-test"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected missing key rejection, got %v", err)
	}
	cfg.OpenAIAPIKey = "synthetic-key"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected explicit OpenAI config to pass, got %v", err)
	}
}

func TestValidateGIRADataControlDoesNotTreatRequestedAsConfirmed(t *testing.T) {
	cfg := validTestConfig()
	cfg.ClinicalGIRAProvider = "none"
	cfg.ClinicalGIRADataControlMode = "zero_data_retention"
	cfg.ClinicalGIRADataControlStatus = "requested"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("requested is a representable but unconfirmed state: %v", err)
	}
	cfg.ClinicalGIRADataControlStatus = "active"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unverifiable data-control status accepted")
	}
}

func TestValidateAllowsDefaultSecretInLocal(t *testing.T) {
	cfg := validTestConfig()
	cfg.JWTAccessSecret = defaultJWTAccessSecret

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected local config to allow default secret, got %v", err)
	}
}

func TestValidateRejectsMissingRequiredDependenciesOutsideLocal(t *testing.T) {
	cfg := validTestConfig()
	cfg.AppEnv = "production"
	cfg.JWTAccessSecret = "a-production-secret-that-is-not-the-default"
	cfg.AuthCookieSecure = true
	cfg.RateLimitLoginPerMin = 10
	cfg.DatabaseURL = ""
	cfg.RedisURL = "redis://redis:6379"

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected missing production database rejection, got %v", err)
	}

	cfg.DatabaseURL = "postgres://sessionflow:secret@postgres:5432/sessionflow"
	cfg.RedisURL = ""
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("expected missing production Redis rejection, got %v", err)
	}
}

func TestValidateRejectsMalformedTrustedProxyCIDR(t *testing.T) {
	cfg := validTestConfig()
	cfg.TrustedProxyCIDRs = "192.0.2.0/24,not-a-cidr"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
		t.Fatalf("invalid trusted proxy accepted: %v", err)
	}
}

func TestValidateRejectsDefaultSecretOutsideLocal(t *testing.T) {
	cfg := validTestConfig()
	cfg.AppEnv = "production"
	cfg.JWTAccessSecret = defaultJWTAccessSecret

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-local config to reject default secret")
	}
}

func TestValidateRejectsBlankSecret(t *testing.T) {
	cfg := validTestConfig()
	cfg.JWTAccessSecret = "   "

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected blank secret to be rejected")
	}
}

func TestValidateAllowsClinicalAudioAfterEphemeralGate(t *testing.T) {
	cfg := validTestConfig()
	cfg.ClinicalAudioEnabled = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected ephemeral clinical audio to be configurable, got %v", err)
	}
}

func TestValidateRejectsRemoteTranscriberURL(t *testing.T) {
	cfg := validTestConfig()
	cfg.TranscriberBaseURL = "https://speech.example.com"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected remote transcriber URL to be rejected")
	}
}

func TestValidateRejectsRemoteOllamaURL(t *testing.T) {
	cfg := validTestConfig()
	cfg.OllamaBaseURL = "https://ollama.example.com"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected remote Ollama URL to be rejected")
	}
}

func TestValidateAcceptsIPv6LoopbackOllamaURL(t *testing.T) {
	cfg := validTestConfig()
	cfg.OllamaBaseURL = "http://[::1]:11434"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected loopback Ollama URL to be accepted, got %v", err)
	}
}

func validTestConfig() Config {
	return Config{
		AppEnv:                      "local",
		JWTAccessSecret:             "local-secret",
		AuthCookieSameSite:          "lax",
		ClinicalRiskProtocol:        "Protocolo clínico ficticio suficientemente detallado para esta prueba.",
		TranscriberBaseURL:          "http://127.0.0.1:8091",
		TranscriberTimeoutSec:       120,
		TranscriberMaxAudioMB:       25,
		OllamaBaseURL:               "http://127.0.0.1:11434",
		OllamaModel:                 "qwen3.5:9b",
		OllamaContextTokens:         4096,
		OllamaTemperature:           0.1,
		OllamaTimeoutSeconds:        45,
		OllamaMaxOutputTokens:       384,
		OllamaReviewContextTokens:   8192,
		OllamaReviewTemperature:     0.15,
		OllamaReviewTimeoutSeconds:  180,
		OllamaReviewMaxOutputTokens: 1024,
		OllamaGIRAContextTokens:     8192,
		OllamaGIRATemperature:       0,
		OllamaGIRATimeoutSeconds:    180,
		OllamaGIRAMaxOutputTokens:   1536,
		OllamaGIRATopP:              0.8,
		OllamaGIRATopK:              20,
		OllamaGIRASeed:              42,
	}
}

func TestDemoModeRequiresLocalEnvironment(t *testing.T) {
	cfg := validTestConfig()
	cfg.DemoMode = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local demo mode should be valid: %v", err)
	}
	cfg.AppEnv = "production"
	cfg.DatabaseURL = "postgres://example"
	cfg.RedisURL = "redis://localhost:6379"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "DEMO_MODE") {
		t.Fatalf("expected production demo mode to be rejected, got %v", err)
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
