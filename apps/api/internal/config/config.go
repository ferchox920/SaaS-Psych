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
	AppEnv                      string
	AppVersion                  string
	BuildRevision               string
	HTTPPort                    string
	DatabaseURL                 string
	RedisURL                    string
	JWTAccessSecret             string
	AccessTTLMin                int
	RefreshTTLDays              int
	RateLimitLoginPerMin        int
	WebOrigin                   string
	AuthCookieSecure            bool
	AuthCookieSameSite          string
	AuthCookieDomain            string
	ClinicalAudioEnabled        bool
	ClinicalRiskProtocol        string
	TranscriberBaseURL          string
	TranscriberTimeoutSec       int
	TranscriberMaxAudioMB       int
	GoogleCalendarEnabled       bool
	GoogleCalendarClientID      string
	GoogleCalendarClientSecret  string
	GoogleCalendarRedirectURL   string
	GoogleCalendarTokenKey      string
	GoogleCalendarTimeoutSec    int
	OllamaBaseURL               string
	OllamaModel                 string
	OllamaContextTokens         int
	OllamaTemperature           float64
	OllamaTimeoutSeconds        int
	OllamaKeepAlive             string
	OllamaMaxOutputTokens       int
	OllamaReviewContextTokens   int
	OllamaReviewTemperature     float64
	OllamaReviewTimeoutSeconds  int
	OllamaReviewMaxOutputTokens int
	OllamaReviewThink           bool
	OTELServiceName             string
	OTLPEndpoint                string
	OTELTracesExporter          string
	OTELResourceAttrs           string
	OTELDBStatement             bool
}

func Load() Config {
	return Config{
		AppEnv:                      getEnv("APP_ENV", "local"),
		AppVersion:                  getEnv("APP_VERSION", "development"),
		BuildRevision:               getEnv("BUILD_REVISION", "unknown"),
		HTTPPort:                    getEnv("HTTP_PORT", "8080"),
		DatabaseURL:                 getEnv("DATABASE_URL", ""),
		RedisURL:                    getEnv("REDIS_URL", ""),
		JWTAccessSecret:             getEnv("JWT_ACCESS_SECRET", defaultJWTAccessSecret),
		AccessTTLMin:                getEnvAsInt("ACCESS_TTL_MIN", 15),
		RefreshTTLDays:              getEnvAsInt("REFRESH_TTL_DAYS", 30),
		RateLimitLoginPerMin:        getEnvAsInt("RATE_LIMIT_LOGIN_PER_MIN", 10),
		WebOrigin:                   getEnv("WEB_ORIGIN", "http://127.0.0.1:3000"),
		AuthCookieSecure:            getEnvAsBool("AUTH_COOKIE_SECURE", false),
		AuthCookieSameSite:          getEnv("AUTH_COOKIE_SAME_SITE", "lax"),
		AuthCookieDomain:            getEnv("AUTH_COOKIE_DOMAIN", ""),
		ClinicalAudioEnabled:        getEnvAsBool("CLINICAL_AUDIO_ENABLED", false),
		ClinicalRiskProtocol:        getEnv("CLINICAL_RISK_PROTOCOL", "Evaluar seguridad, inmediatez, intencion, medios, proteccion y capacidad de autocuidado; documentar el juicio clinico y seguir el circuito local de emergencia definido por Fernando."),
		TranscriberBaseURL:          getEnv("TRANSCRIBER_BASE_URL", "http://127.0.0.1:8091"),
		TranscriberTimeoutSec:       getEnvAsInt("TRANSCRIBER_TIMEOUT_SECONDS", 120),
		TranscriberMaxAudioMB:       getEnvAsInt("TRANSCRIBER_MAX_AUDIO_MB", 25),
		GoogleCalendarEnabled:       getEnvAsBool("GOOGLE_CALENDAR_ENABLED", false),
		GoogleCalendarClientID:      getEnv("GOOGLE_CALENDAR_CLIENT_ID", ""),
		GoogleCalendarClientSecret:  getEnv("GOOGLE_CALENDAR_CLIENT_SECRET", ""),
		GoogleCalendarRedirectURL:   getEnv("GOOGLE_CALENDAR_REDIRECT_URL", "http://127.0.0.1:8080/api/v1/integrations/google-calendar/callback"),
		GoogleCalendarTokenKey:      getEnv("GOOGLE_CALENDAR_TOKEN_KEY", ""),
		GoogleCalendarTimeoutSec:    getEnvAsInt("GOOGLE_CALENDAR_TIMEOUT_SECONDS", 20),
		OllamaBaseURL:               getEnv("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
		OllamaModel:                 getEnv("OLLAMA_MODEL", "qwen3.5:9b"),
		OllamaContextTokens:         getEnvAsInt("OLLAMA_CONTEXT_TOKENS", 4096),
		OllamaTemperature:           getEnvAsFloat("OLLAMA_TEMPERATURE", 0.1),
		OllamaTimeoutSeconds:        getEnvAsInt("OLLAMA_TIMEOUT_SECONDS", 45),
		OllamaKeepAlive:             getEnv("OLLAMA_KEEP_ALIVE", "15m"),
		OllamaMaxOutputTokens:       getEnvAsInt("OLLAMA_MAX_OUTPUT_TOKENS", 384),
		OllamaReviewContextTokens:   getEnvAsInt("OLLAMA_REVIEW_CONTEXT_TOKENS", 8192),
		OllamaReviewTemperature:     getEnvAsFloat("OLLAMA_REVIEW_TEMPERATURE", 0.15),
		OllamaReviewTimeoutSeconds:  getEnvAsInt("OLLAMA_REVIEW_TIMEOUT_SECONDS", 180),
		OllamaReviewMaxOutputTokens: getEnvAsInt("OLLAMA_REVIEW_MAX_OUTPUT_TOKENS", 1024),
		OllamaReviewThink:           getEnvAsBool("OLLAMA_REVIEW_THINK", false),
		OTELServiceName:             getEnv("OTEL_SERVICE_NAME", "sessionflow-api"),
		OTLPEndpoint:                getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTELTracesExporter:          getEnv("OTEL_TRACES_EXPORTER", "none"),
		OTELResourceAttrs:           getEnv("OTEL_RESOURCE_ATTRIBUTES", ""),
		OTELDBStatement:             getEnvAsBool("OTEL_DB_STATEMENT_ENABLED", false),
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
	secret := strings.TrimSpace(c.JWTAccessSecret)
	if secret == "" {
		return fmt.Errorf("JWT_ACCESS_SECRET cannot be empty")
	}

	if normalizeEnv(c.AppEnv) != "local" && secret == defaultJWTAccessSecret {
		return fmt.Errorf("JWT_ACCESS_SECRET cannot use the default value outside local environment")
	}
	if normalizeEnv(c.AppEnv) != "local" && !c.AuthCookieSecure {
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
		if normalizeEnv(c.AppEnv) != "local" && redirect.Scheme != "https" {
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

	return nil
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
