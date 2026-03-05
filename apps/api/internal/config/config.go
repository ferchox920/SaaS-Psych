package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv               string
	HTTPPort             string
	DatabaseURL          string
	RedisURL             string
	JWTAccessSecret      string
	AccessTTLMin         int
	RefreshTTLDays       int
	RateLimitLoginPerMin int
	OTELServiceName      string
	OTLPEndpoint         string
	OTELTracesExporter   string
	OTELResourceAttrs    string
	OTELDBStatement      bool
}

func Load() Config {
	return Config{
		AppEnv:               getEnv("APP_ENV", "local"),
		HTTPPort:             getEnv("HTTP_PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", ""),
		RedisURL:             getEnv("REDIS_URL", ""),
		JWTAccessSecret:      getEnv("JWT_ACCESS_SECRET", "change-me"),
		AccessTTLMin:         getEnvAsInt("ACCESS_TTL_MIN", 15),
		RefreshTTLDays:       getEnvAsInt("REFRESH_TTL_DAYS", 30),
		RateLimitLoginPerMin: getEnvAsInt("RATE_LIMIT_LOGIN_PER_MIN", 10),
		OTELServiceName:      getEnv("OTEL_SERVICE_NAME", "sessionflow-api"),
		OTLPEndpoint:         getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTELTracesExporter:   getEnv("OTEL_TRACES_EXPORTER", "none"),
		OTELResourceAttrs:    getEnv("OTEL_RESOURCE_ATTRIBUTES", ""),
		OTELDBStatement:      getEnvAsBool("OTEL_DB_STATEMENT_ENABLED", false),
	}
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

func (c Config) AccessTTL() time.Duration {
	return time.Duration(c.AccessTTLMin) * time.Minute
}

func (c Config) RefreshTTL() time.Duration {
	return time.Duration(c.RefreshTTLDays) * 24 * time.Hour
}
