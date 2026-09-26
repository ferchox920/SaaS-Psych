package ollama

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	giracertification "sessionflow/apps/api/internal/certification/gira"
	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
)

// TestOllamaGIRAEvaluationAThroughJ is opt-in and reuses the exact same
// synthetic fixture inventory as remote certification.
func TestOllamaGIRAEvaluationAThroughJ(t *testing.T) {
	if os.Getenv("RUN_OLLAMA_GIRA_EVAL") != "1" {
		t.Skip("set RUN_OLLAMA_GIRA_EVAL=1 to run the local GIRA evaluation")
	}
	model := os.Getenv("OLLAMA_GIRA_MODEL")
	if model == "" {
		model = "qwen3.5:9b"
	}
	provider, err := NewProvider(Config{
		BaseURL:             envOr("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
		Model:               model,
		GIRAContextTokens:   envInt("OLLAMA_GIRA_CONTEXT_TOKENS", 12288),
		GIRATemperature:     envFloat("OLLAMA_GIRA_TEMPERATURE", 0.1),
		GIRATimeout:         time.Duration(envInt("OLLAMA_GIRA_TIMEOUT_SECONDS", 240)) * time.Second,
		GIRAMaxOutputTokens: envInt("OLLAMA_GIRA_MAX_OUTPUT_TOKENS", 4096),
		GIRATopP:            envFloat("OLLAMA_GIRA_TOP_P", 0.8),
		GIRATopK:            envInt("OLLAMA_GIRA_TOP_K", 20),
		GIRASeed:            envInt("OLLAMA_GIRA_SEED", 42),
	})
	if err != nil {
		t.Fatal(err)
	}
	runs := envInt("OLLAMA_GIRA_RUNS", 1)
	if runs < 1 || runs > 3 {
		t.Fatal("OLLAMA_GIRA_RUNS must be between 1 and 3")
	}
	filter := os.Getenv("OLLAMA_GIRA_FIXTURE")
	runner := giracertification.Runner{Provider: provider, ProviderName: "ollama", Model: model}
	for run := 1; run <= runs; run++ {
		for _, fixture := range giracertification.FixturesAThroughJ() {
			if filter != "" && fixture.Name != filter {
				continue
			}
			t.Run(fmt.Sprintf("run_%d/%s", run, fixture.Name), func(t *testing.T) {
				outcome, runErr := runner.Run(context.Background(), fixture)
				if runErr != nil && fixture.AllowBackendRejection && errors.Is(runErr, clinicalanalysis.ErrInvalidModelOutput) {
					t.Logf("safe backend rejection model=%s fixture=%s err=%v", model, fixture.Name, runErr)
					return
				}
				if runErr != nil {
					var outputErr *GIRAOutputError
					if errors.As(runErr, &outputErr) {
						t.Logf("diagnostics=%+v", outputErr.Diagnostics)
					}
					t.Fatal(runErr)
				}
				if outcome.Diff.Status != "pending_review" {
					t.Fatalf("expected pending_review diff, got %s", outcome.Diff.Status)
				}
				t.Logf("model=%s fixture=%s payload_bytes=%d repair=%t repair_reason=%s total=%s", model, fixture.Name, outcome.PayloadBytes, outcome.Provider.Metrics.Repaired, outcome.Provider.Metrics.RepairReason, outcome.Provider.Metrics.Total)
			})
		}
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func envFloat(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return fallback
	}
	return value
}
