package gira

import (
	"context"
	"strings"
	"testing"

	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
)

type neverProvider struct{ called bool }

func (p *neverProvider) BuildGIRA(context.Context, string, longitudinal.GIRAProviderRequest, func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	p.called = true
	return clinicalanalysis.ProviderOutput{}, nil
}

func TestRunnerRejectsNonSyntheticFixtureBeforeProvider(t *testing.T) {
	provider := &neverProvider{}
	fixture := FixturesAThroughJ()[0]
	fixture.Synthetic = false
	_, err := (Runner{Provider: provider, ProviderName: "test", Model: "test"}).Run(context.Background(), fixture)
	if err == nil || !strings.Contains(err.Error(), "non-synthetic") || provider.called {
		t.Fatalf("synthetic-only guard failed called=%t err=%v", provider.called, err)
	}
}

func TestRunnerRejectsForbiddenOutboundLiteralBeforeProvider(t *testing.T) {
	provider := &neverProvider{}
	fixture := FixturesAThroughJ()[0]
	fixture.Input.SelectedProcess.Description = "synthetic marker LEAK_TOKEN_123"
	fixture.ForbiddenOutboundLiterals = append(fixture.ForbiddenOutboundLiterals, "LEAK_TOKEN_123")
	_, err := (Runner{Provider: provider, ProviderName: "test", Model: "test"}).Run(context.Background(), fixture)
	if err == nil || !strings.Contains(err.Error(), "forbidden outbound") || provider.called {
		t.Fatalf("outbound preflight guard failed called=%t err=%v", provider.called, err)
	}
}

func TestSharedFixtureInventoryIsExactlyAThroughJAndSynthetic(t *testing.T) {
	fixtures := FixturesAThroughJ()
	if len(fixtures) != 10 {
		t.Fatalf("expected A-J, got %d", len(fixtures))
	}
	for _, fixture := range fixtures {
		if !fixture.Synthetic || fixture.Name == "" || fixture.Assert == nil {
			t.Fatalf("unsafe or incomplete fixture: %+v", fixture)
		}
	}
}
