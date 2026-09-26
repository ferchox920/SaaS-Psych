package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const defaultJWTAccessSecret = "change-me"

type Config struct {
	DemoMode                               bool
	ClinicalIngestionEnabled               bool
	ClinicalArtifactRoot                   string
	ClinicalArtifactKeyID                  string
	ClinicalArtifactKey                    string
	ClinicalAudioRetentionHours            int
	ClinicalTranscriptRetentionPolicy      string
	ClinicalJobMetadataRetentionPolicy     string
	ClinicalIngestionWorkerTenants         string
	ClinicalIngestionTimeoutSeconds        int
	ClinicalTranscriptionConfigurationHash string
	AppEnv                                 string
	AppVersion                             string
	BuildRevision                          string
	HTTPPort                               string
	DatabaseURL                            string
	RedisURL                               string
	JWTAccessSecret                        string
	AccessTTLMin                           int
	RefreshTTLDays                         int
	RateLimitLoginPerMin                   int
	TrustedProxyCIDRs                      string
	WebOrigin                              string
	AuthCookieSecure                       bool
	AuthCookieSameSite                     string
	AuthCookieDomain                       string
	ClinicalAudioEnabled                   bool
	ClinicalRiskProtocol                   string
	TranscriberBaseURL                     string
	TranscriberTimeoutSec                  int
	TranscriberMaxAudioMB                  int
	GoogleCalendarEnabled                  bool
	GoogleCalendarClientID                 string
	GoogleCalendarClientSecret             string
	GoogleCalendarRedirectURL              string
	GoogleCalendarTokenKey                 string
	GoogleCalendarTimeoutSec               int
	OllamaBaseURL                          string
	OllamaModel                            string
	OllamaContextTokens                    int
	OllamaTemperature                      float64
	OllamaTimeoutSeconds                   int
	OllamaKeepAlive                        string
	OllamaMaxOutputTokens                  int
	OllamaReviewContextTokens              int
	OllamaReviewTemperature                float64
	OllamaReviewTimeoutSeconds             int
	OllamaReviewMaxOutputTokens            int
	OllamaReviewThink                      bool
	OllamaGIRAContextTokens                int
	OllamaGIRATemperature                  float64
	OllamaGIRATimeoutSeconds               int
	OllamaGIRAMaxOutputTokens              int
	OllamaGIRAThink                        bool
	OllamaGIRATopP                         float64
	OllamaGIRATopK                         int
	OllamaGIRASeed                         int
	ClinicalGIRAProvider                   string
	ExperimentalRemoteGIRA                 bool
	ClinicalGIRAModel                      string
	ClinicalGIRAProviderRegion             string
	ClinicalGIRADataControlMode            string
	ClinicalGIRADataControlStatus          string
	OpenAIAPIKey                           string
	OpenAIBaseURL                          string
	OpenAIGIRATimeoutSeconds               int
	OpenAIGIRAMaxOutputTokens              int
	GIRAPriceConfigVersion                 string
	GIRAInputUSDPerMillion                 float64
	GIRACachedInputUSDPerMillion           float64
	GIRAOutputUSDPerMillion                float64
	OTELServiceName                        string
	OTLPEndpoint                           string
	OTELTracesExporter                     string
	OTELResourceAttrs                      string
	OTELDBStatement                        bool
}

func Load() Config {
	return Config{
		DemoMode:                               getEnvAsBool("DEMO_MODE", false),
		ClinicalIngestionEnabled:               getEnvAsBool("CLINICAL_INGESTION_ENABLED", false),
		ClinicalArtifactRoot:                   getEnv("CLINICAL_ARTIFACT_ROOT", ""),
		ClinicalArtifactKeyID:                  getEnv("CLINICAL_ARTIFACT_KEY_ID", ""),
		ClinicalArtifactKey:                    getEnv("CLINICAL_ARTIFACT_KEY", ""),
		ClinicalAudioRetentionHours:            getEnvAsInt("CLINICAL_AUDIO_RETENTION_HOURS", 0),
		ClinicalTranscriptRetentionPolicy:      getEnv("CLINICAL_TRANSCRIPT_RETENTION_POLICY", ""),
		ClinicalJobMetadataRetentionPolicy:     getEnv("CLINICAL_JOB_METADATA_RETENTION_POLICY", ""),
		ClinicalIngestionWorkerTenants:         getEnv("CLINICAL_INGESTION_WORKER_TENANTS", ""),
		ClinicalIngestionTimeoutSeconds:        getEnvAsInt("CLINICAL_INGESTION_TIMEOUT_SECONDS", 600),
		ClinicalTranscriptionConfigurationHash: getEnv("CLINICAL_TRANSCRIPTION_CONFIGURATION_HASH", ""),
		AppEnv:                                 getEnv("APP_ENV", "local"),
		AppVersion:                             getEnv("APP_VERSION", "development"),
		BuildRevision:                          getEnv("BUILD_REVISION", "unknown"),
		HTTPPort:                               getEnv("HTTP_PORT", "8080"),
		DatabaseURL:                            getEnv("DATABASE_URL", ""),
		RedisURL:                               getEnv("REDIS_URL", ""),
		JWTAccessSecret:                        getEnv("JWT_ACCESS_SECRET", defaultJWTAccessSecret),
		AccessTTLMin:                           getEnvAsInt("ACCESS_TTL_MIN", 15),
		RefreshTTLDays:                         getEnvAsInt("REFRESH_TTL_DAYS", 30),
		RateLimitLoginPerMin:                   getEnvAsInt("RATE_LIMIT_LOGIN_PER_MIN", 10),
		TrustedProxyCIDRs:                      getEnv("TRUSTED_PROXY_CIDRS", ""),
		WebOrigin:                              getEnv("WEB_ORIGIN", "http://127.0.0.1:3000"),
		AuthCookieSecure:                       getEnvAsBool("AUTH_COOKIE_SECURE", false),
		AuthCookieSameSite:                     getEnv("AUTH_COOKIE_SAME_SITE", "lax"),
		AuthCookieDomain:                       getEnv("AUTH_COOKIE_DOMAIN", ""),
		ClinicalAudioEnabled:                   getEnvAsBool("CLINICAL_AUDIO_ENABLED", false),
		ClinicalRiskProtocol:                   getEnv("CLINICAL_RISK_PROTOCOL", "Evaluar seguridad, inmediatez, intencion, medios, proteccion y capacidad de autocuidado; documentar el juicio clinico y seguir el circuito local de emergencia definido por la organizacion."),
		TranscriberBaseURL:                     getEnv("TRANSCRIBER_BASE_URL", "http://127.0.0.1:8091"),
		TranscriberTimeoutSec:                  getEnvAsInt("TRANSCRIBER_TIMEOUT_SECONDS", 120),
		TranscriberMaxAudioMB:                  getEnvAsInt("TRANSCRIBER_MAX_AUDIO_MB", 25),
		GoogleCalendarEnabled:                  getEnvAsBool("GOOGLE_CALENDAR_ENABLED", false),
		GoogleCalendarClientID:                 getEnv("GOOGLE_CALENDAR_CLIENT_ID", ""),
		GoogleCalendarClientSecret:             getEnv("GOOGLE_CALENDAR_CLIENT_SECRET", ""),
		GoogleCalendarRedirectURL:              getEnv("GOOGLE_CALENDAR_REDIRECT_URL", "http://127.0.0.1:8080/api/v1/integrations/google-calendar/callback"),
		GoogleCalendarTokenKey:                 getEnv("GOOGLE_CALENDAR_TOKEN_KEY", ""),
		GoogleCalendarTimeoutSec:               getEnvAsInt("GOOGLE_CALENDAR_TIMEOUT_SECONDS", 20),
		OllamaBaseURL:                          getEnv("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
		OllamaModel:                            getEnv("OLLAMA_MODEL", "qwen3.5:9b"),
		OllamaContextTokens:                    getEnvAsInt("OLLAMA_CONTEXT_TOKENS", 4096),
		OllamaTemperature:                      getEnvAsFloat("OLLAMA_TEMPERATURE", 0.1),
		OllamaTimeoutSeconds:                   getEnvAsInt("OLLAMA_TIMEOUT_SECONDS", 45),
		OllamaKeepAlive:                        getEnv("OLLAMA_KEEP_ALIVE", "15m"),
		OllamaMaxOutputTokens:                  getEnvAsInt("OLLAMA_MAX_OUTPUT_TOKENS", 384),
		OllamaReviewContextTokens:              getEnvAsInt("OLLAMA_REVIEW_CONTEXT_TOKENS", 8192),
		OllamaReviewTemperature:                getEnvAsFloat("OLLAMA_REVIEW_TEMPERATURE", 0.15),
		OllamaReviewTimeoutSeconds:             getEnvAsInt("OLLAMA_REVIEW_TIMEOUT_SECONDS", 180),
		OllamaReviewMaxOutputTokens:            getEnvAsInt("OLLAMA_REVIEW_MAX_OUTPUT_TOKENS", 1024),
		OllamaReviewThink:                      getEnvAsBool("OLLAMA_REVIEW_THINK", false),
		OllamaGIRAContextTokens:                getEnvAsInt("OLLAMA_GIRA_CONTEXT_TOKENS", 8192),
		OllamaGIRATemperature:                  getEnvAsFloat("OLLAMA_GIRA_TEMPERATURE", 0),
		OllamaGIRATimeoutSeconds:               getEnvAsInt("OLLAMA_GIRA_TIMEOUT_SECONDS", 180),
		OllamaGIRAMaxOutputTokens:              getEnvAsInt("OLLAMA_GIRA_MAX_OUTPUT_TOKENS", 1536),
		OllamaGIRAThink:                        getEnvAsBool("OLLAMA_GIRA_THINK", false),
		OllamaGIRATopP:                         getEnvAsFloat("OLLAMA_GIRA_TOP_P", 0.8),
		OllamaGIRATopK:                         getEnvAsInt("OLLAMA_GIRA_TOP_K", 20),
		OllamaGIRASeed:                         getEnvAsInt("OLLAMA_GIRA_SEED", 42),
		ClinicalGIRAProvider:                   getEnv("CLINICAL_GIRA_PROVIDER", "none"),
		ExperimentalRemoteGIRA:                 getEnv("EXPERIMENTAL_REMOTE_GIRA", "false") == "true",
		ClinicalGIRAModel:                      getEnv("CLINICAL_GIRA_MODEL", ""),
		ClinicalGIRAProviderRegion:             getEnv("CLINICAL_GIRA_PROVIDER_REGION", ""),
		ClinicalGIRADataControlMode:            getEnv("CLINICAL_GIRA_DATA_CONTROL_MODE", "standard"),
		ClinicalGIRADataControlStatus:          getEnv("CLINICAL_GIRA_DATA_CONTROL_STATUS", "unknown"),
		OpenAIAPIKey:                           getEnv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:                          getEnv("OPENAI_BASE_URL", "https://api.openai.com"),
		OpenAIGIRATimeoutSeconds:               getEnvAsInt("OPENAI_GIRA_TIMEOUT_SECONDS", 180),
		OpenAIGIRAMaxOutputTokens:              getEnvAsInt("OPENAI_GIRA_MAX_OUTPUT_TOKENS", 2048),
		GIRAPriceConfigVersion:                 getEnv("CLINICAL_GIRA_PRICE_CONFIG_VERSION", "unconfigured"),
		GIRAInputUSDPerMillion:                 getEnvAsFloat("CLINICAL_GIRA_INPUT_USD_PER_MILLION", 0),
		GIRACachedInputUSDPerMillion:           getEnvAsFloat("CLINICAL_GIRA_CACHED_INPUT_USD_PER_MILLION", 0),
		GIRAOutputUSDPerMillion:                getEnvAsFloat("CLINICAL_GIRA_OUTPUT_USD_PER_MILLION", 0),
		OTELServiceName:                        getEnv("OTEL_SERVICE_NAME", "sessionflow-api"),
		OTLPEndpoint:                           getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTELTracesExporter:                     getEnv("OTEL_TRACES_EXPORTER", "none"),
		OTELResourceAttrs:                      getEnv("OTEL_RESOURCE_ATTRIBUTES", ""),
		OTELDBStatement:                        getEnvAsBool("OTEL_DB_STATEMENT_ENABLED", false),
	}
}

func LoadLocalEnv() {
	for _, candidate := range dotenvCandidates() {
		if _, err := os.Stat(candidate); err == nil {
			_ = godotenv.Load(candidate)
			return
		}
	}
}

func dotenvCandidates() []string {
	wd, err := os.Getwd()
	if err != nil {
		return []string{".env"}
	}

	candidates := []string{
		filepath.Join(wd, ".env"),
	}

	for dir := wd; ; dir = filepath.Dir(dir) {
		candidates = append(candidates, filepath.Join(dir, ".env"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	return candidates
}

func (c Config) Validate() error {
	if _, err := c.TrustedProxyRanges(); err != nil {
		return err
	}
	if err := c.validateIngestion(); err != nil {
		return err
	}
	secret := strings.TrimSpace(c.JWTAccessSecret)
	isLocal := normalizeEnv(c.AppEnv) == "local"
	if c.DemoMode && !isLocal {
		return fmt.Errorf("DEMO_MODE is only permitted in the local environment")
	}
	if secret == "" {
		return fmt.Errorf("JWT_ACCESS_SECRET cannot be empty")
	}

	if !isLocal && strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required outside local environment")
	}
	if !isLocal && c.RateLimitLoginPerMin > 0 && strings.TrimSpace(c.RedisURL) == "" {
		return fmt.Errorf("REDIS_URL is required outside local environment when login rate limiting is enabled")
	}
	if !isLocal && secret == defaultJWTAccessSecret {
		return fmt.Errorf("JWT_ACCESS_SECRET cannot use the default value outside local environment")
	}
	if !isLocal && !c.AuthCookieSecure {
		return fmt.Errorf("AUTH_COOKIE_SECURE must be true outside local environment")
	}
	sameSite := normalizeEnv(c.AuthCookieSameSite)
	if sameSite == "" {
		sameSite = "lax"
	}
	if sameSite != "lax" && sameSite != "strict" && sameSite != "none" {
		return fmt.Errorf("AUTH_COOKIE_SAME_SITE must be one of: lax, strict, none")
	}
	if sameSite == "none" && !c.AuthCookieSecure {
		return fmt.Errorf("AUTH_COOKIE_SECURE must be true when AUTH_COOKIE_SAME_SITE=none")
	}
	if err := validateLoopbackURL(c.TranscriberBaseURL); err != nil {
		return fmt.Errorf("TRANSCRIBER_BASE_URL: %w", err)
	}
	if c.TranscriberTimeoutSec < 10 || c.TranscriberTimeoutSec > 600 {
		return fmt.Errorf("TRANSCRIBER_TIMEOUT_SECONDS must be between 10 and 600")
	}
	if c.TranscriberMaxAudioMB < 1 || c.TranscriberMaxAudioMB > 100 {
		return fmt.Errorf("TRANSCRIBER_MAX_AUDIO_MB must be between 1 and 100")
	}
	if c.GoogleCalendarEnabled {
		if strings.TrimSpace(c.GoogleCalendarClientID) == "" || strings.TrimSpace(c.GoogleCalendarClientSecret) == "" {
			return fmt.Errorf("GOOGLE_CALENDAR_CLIENT_ID and GOOGLE_CALENDAR_CLIENT_SECRET are required when Google Calendar is enabled")
		}
		if strings.TrimSpace(c.GoogleCalendarTokenKey) == "" {
			return fmt.Errorf("GOOGLE_CALENDAR_TOKEN_KEY is required when Google Calendar is enabled")
		}
		redirect, err := url.Parse(strings.TrimSpace(c.GoogleCalendarRedirectURL))
		if err != nil || redirect.Scheme == "" || redirect.Hostname() == "" {
			return fmt.Errorf("GOOGLE_CALENDAR_REDIRECT_URL must be a valid absolute URL")
		}
		if !isLocal && redirect.Scheme != "https" {
			return fmt.Errorf("GOOGLE_CALENDAR_REDIRECT_URL must use https outside local environment")
		}
		if c.GoogleCalendarTimeoutSec < 5 || c.GoogleCalendarTimeoutSec > 120 {
			return fmt.Errorf("GOOGLE_CALENDAR_TIMEOUT_SECONDS must be between 5 and 120")
		}
	}
	if length := len([]rune(strings.TrimSpace(c.ClinicalRiskProtocol))); length < 20 || length > 2000 {
		return fmt.Errorf("CLINICAL_RISK_PROTOCOL must contain between 20 and 2000 characters")
	}
	if err := validateLoopbackURL(c.OllamaBaseURL); err != nil {
		return fmt.Errorf("OLLAMA_BASE_URL: %w", err)
	}
	if strings.TrimSpace(c.OllamaModel) == "" {
		return fmt.Errorf("OLLAMA_MODEL cannot be empty")
	}
	if c.OllamaContextTokens < 1024 || c.OllamaContextTokens > 32768 {
		return fmt.Errorf("OLLAMA_CONTEXT_TOKENS must be between 1024 and 32768")
	}
	if c.OllamaTemperature < 0 || c.OllamaTemperature > 1 {
		return fmt.Errorf("OLLAMA_TEMPERATURE must be between 0 and 1")
	}
	if c.OllamaTimeoutSeconds < 5 || c.OllamaTimeoutSeconds > 300 {
		return fmt.Errorf("OLLAMA_TIMEOUT_SECONDS must be between 5 and 300")
	}
	if c.OllamaMaxOutputTokens < 80 || c.OllamaMaxOutputTokens > 512 {
		return fmt.Errorf("OLLAMA_MAX_OUTPUT_TOKENS must be between 80 and 512")
	}
	if c.OllamaReviewContextTokens < 4096 || c.OllamaReviewContextTokens > 32768 {
		return fmt.Errorf("OLLAMA_REVIEW_CONTEXT_TOKENS must be between 4096 and 32768")
	}
	if c.OllamaReviewTemperature < 0 || c.OllamaReviewTemperature > 1 {
		return fmt.Errorf("OLLAMA_REVIEW_TEMPERATURE must be between 0 and 1")
	}
	if c.OllamaReviewTimeoutSeconds < 30 || c.OllamaReviewTimeoutSeconds > 600 {
		return fmt.Errorf("OLLAMA_REVIEW_TIMEOUT_SECONDS must be between 30 and 600")
	}
	if c.OllamaReviewMaxOutputTokens < 384 || c.OllamaReviewMaxOutputTokens > 2048 {
		return fmt.Errorf("OLLAMA_REVIEW_MAX_OUTPUT_TOKENS must be between 384 and 2048")
	}
	if c.OllamaGIRAContextTokens < 4096 || c.OllamaGIRAContextTokens > 32768 {
		return fmt.Errorf("OLLAMA_GIRA_CONTEXT_TOKENS must be between 4096 and 32768")
	}
	if c.OllamaGIRATemperature < 0 || c.OllamaGIRATemperature > 1 {
		return fmt.Errorf("OLLAMA_GIRA_TEMPERATURE must be between 0 and 1")
	}
	if c.OllamaGIRATimeoutSeconds < 30 || c.OllamaGIRATimeoutSeconds > 600 {
		return fmt.Errorf("OLLAMA_GIRA_TIMEOUT_SECONDS must be between 30 and 600")
	}
	if c.OllamaGIRAMaxOutputTokens < 512 || c.OllamaGIRAMaxOutputTokens > 4096 {
		return fmt.Errorf("OLLAMA_GIRA_MAX_OUTPUT_TOKENS must be between 512 and 4096")
	}
	if c.OllamaGIRATopP <= 0 || c.OllamaGIRATopP > 1 {
		return fmt.Errorf("OLLAMA_GIRA_TOP_P must be greater than 0 and at most 1")
	}
	if c.OllamaGIRATopK < 1 || c.OllamaGIRATopK > 200 {
		return fmt.Errorf("OLLAMA_GIRA_TOP_K must be between 1 and 200")
	}
	provider := normalizeEnv(c.ClinicalGIRAProvider)
	if provider == "" {
		provider = "none"
	}
	if provider != "none" && provider != "openai" {
		return fmt.Errorf("CLINICAL_GIRA_PROVIDER must be one of: none, openai")
	}
	dataMode := normalizeEnv(c.ClinicalGIRADataControlMode)
	if dataMode == "" {
		dataMode = "standard"
	}
	if dataMode != "standard" && dataMode != "modified_abuse_monitoring" && dataMode != "zero_data_retention" {
		return fmt.Errorf("CLINICAL_GIRA_DATA_CONTROL_MODE must be one of: standard, modified_abuse_monitoring, zero_data_retention")
	}
	dataStatus := normalizeEnv(c.ClinicalGIRADataControlStatus)
	if dataStatus == "" {
		dataStatus = "unknown"
	}
	if dataStatus != "requested" && dataStatus != "confirmed" && dataStatus != "unknown" {
		return fmt.Errorf("CLINICAL_GIRA_DATA_CONTROL_STATUS must be one of: requested, confirmed, unknown")
	}
	if provider == "openai" {
		if !c.ExperimentalRemoteGIRA {
			return fmt.Errorf("remote GIRA is dormant; selected product workflow is manual project import")
		}
		if strings.TrimSpace(c.ClinicalGIRAModel) == "" {
			return fmt.Errorf("CLINICAL_GIRA_MODEL is required when CLINICAL_GIRA_PROVIDER=openai")
		}
		if strings.TrimSpace(c.OpenAIAPIKey) == "" {
			return fmt.Errorf("OPENAI_API_KEY is required when CLINICAL_GIRA_PROVIDER=openai")
		}
		remoteURL, err := url.Parse(strings.TrimSpace(c.OpenAIBaseURL))
		if err != nil || remoteURL.Scheme != "https" || remoteURL.Hostname() == "" {
			return fmt.Errorf("OPENAI_BASE_URL must be an absolute HTTPS URL")
		}
		if c.OpenAIGIRATimeoutSeconds < 10 || c.OpenAIGIRATimeoutSeconds > 600 {
			return fmt.Errorf("OPENAI_GIRA_TIMEOUT_SECONDS must be between 10 and 600")
		}
		if c.OpenAIGIRAMaxOutputTokens < 512 || c.OpenAIGIRAMaxOutputTokens > 8192 {
			return fmt.Errorf("OPENAI_GIRA_MAX_OUTPUT_TOKENS must be between 512 and 8192")
		}
	}
	if c.GIRAInputUSDPerMillion < 0 || c.GIRACachedInputUSDPerMillion < 0 || c.GIRAOutputUSDPerMillion < 0 {
		return fmt.Errorf("clinical GIRA token prices cannot be negative")
	}

	return nil
}

func (c Config) TrustedProxyRanges() ([]*net.IPNet, error) {
	if strings.TrimSpace(c.TrustedProxyCIDRs) == "" {
		return nil, nil
	}
	parts := strings.Split(c.TrustedProxyCIDRs, ",")
	ranges := make([]*net.IPNet, 0, len(parts))
	for _, part := range parts {
		_, network, err := net.ParseCIDR(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS must contain only CIDR ranges: %w", err)
		}
		ranges = append(ranges, network)
	}
	return ranges, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getEnvAsInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return n
}

func getEnvAsBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	b, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return b
}

func getEnvAsFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return n
}

func validateLoopbackURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return fmt.Errorf("must be a valid absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("must resolve explicitly to a loopback host")
	}
	return nil
}

func (c Config) AccessTTL() time.Duration {
	return time.Duration(c.AccessTTLMin) * time.Minute
}

func (c Config) RefreshTTL() time.Duration {
	return time.Duration(c.RefreshTTLDays) * 24 * time.Hour
}

func normalizeEnv(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
