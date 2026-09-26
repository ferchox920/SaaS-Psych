package remotegira

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	giracertification "sessionflow/apps/api/internal/certification/gira"
)

// TestRemoteGIRACertificationAThroughJ is deliberately double opt-in. It is
// the only test in this package permitted to contact a real remote provider,
// and the shared runner rejects every fixture not explicitly marked synthetic.
func TestRemoteGIRACertificationAThroughJ(t *testing.T) {
	if os.Getenv("RUN_REMOTE_GIRA_CERTIFICATION") != "1" {
		t.Skip("set RUN_REMOTE_GIRA_CERTIFICATION=1 to run remote certification")
	}
	if os.Getenv("REMOTE_GIRA_SYNTHETIC_ONLY") != "1" {
		t.Fatal("REMOTE_GIRA_SYNTHETIC_ONLY=1 is required")
	}
	if os.Getenv("CLINICAL_GIRA_PROVIDER") != "openai" {
		t.Fatal("CLINICAL_GIRA_PROVIDER=openai is required")
	}
	key, model := os.Getenv("OPENAI_API_KEY"), os.Getenv("CLINICAL_GIRA_MODEL")
	if key == "" || model == "" {
		t.Fatal("OPENAI_API_KEY and CLINICAL_GIRA_MODEL are required")
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	transport, err := NewHTTPTransport(HTTPTransportConfig{
		BaseURL: baseURL, APIKey: key,
		Timeout: time.Duration(certEnvInt("OPENAI_GIRA_TIMEOUT_SECONDS", 120)) * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewOpenAIGIRABuilderProvider(transport, OpenAIConfig{
		Model: model, MaxOutputTokens: certEnvInt("OPENAI_GIRA_MAX_OUTPUT_TOKENS", 4096),
		ProviderRegion:    os.Getenv("CLINICAL_GIRA_PROVIDER_REGION"),
		DataControlMode:   os.Getenv("CLINICAL_GIRA_DATA_CONTROL_MODE"),
		DataControlStatus: os.Getenv("CLINICAL_GIRA_DATA_CONTROL_STATUS"),
		Pricing: PricingConfig{
			Version:                  os.Getenv("CLINICAL_GIRA_PRICE_CONFIG_VERSION"),
			InputUSDPerMillion:       certEnvFloat("CLINICAL_GIRA_INPUT_USD_PER_MILLION"),
			CachedInputUSDPerMillion: certEnvFloat("CLINICAL_GIRA_CACHED_INPUT_USD_PER_MILLION"),
			OutputUSDPerMillion:      certEnvFloat("CLINICAL_GIRA_OUTPUT_USD_PER_MILLION"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	runs := certEnvInt("REMOTE_GIRA_RUNS", 1)
	if runs < 1 || runs > 3 {
		t.Fatal("REMOTE_GIRA_RUNS must be between 1 and 3")
	}
	runner := giracertification.Runner{Provider: provider, ProviderName: "openai", Model: model}
	for run := 1; run <= runs; run++ {
		for _, fixture := range giracertification.FixturesAThroughJ() {
			t.Run(fmt.Sprintf("run_%d/%s", run, fixture.Name), func(t *testing.T) {
				outcome, runErr := runner.Run(context.Background(), fixture)
				if runErr != nil {
					t.Fatal(runErr)
				}
				if outcome.Diff.Status != "pending_review" {
					t.Fatalf("expected pending_review diff, got %s", outcome.Diff.Status)
				}
				t.Logf("fixture=%s request_id=%s input_tokens=%d output_tokens=%d repair=%d cost_micros=%d total=%s", fixture.Name, outcome.Provider.Metadata.RemoteRequestID, outcome.Provider.Metadata.InputTokens, outcome.Provider.Metadata.OutputTokens, outcome.Provider.Metadata.RepairAttempts, outcome.Provider.Metadata.EstimatedCostMicros, outcome.Provider.Metrics.Total)
			})
		}
	}
}

func certEnvInt(key string, fallback ...int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err == nil {
		return value
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}

func certEnvFloat(key string) float64 {
	value, _ := strconv.ParseFloat(os.Getenv(key), 64)
	return value
}
