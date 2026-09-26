package config

import "testing"

func TestManualBridgeDoesNotRequireAPIKeyAndRemoteIsDormant(t *testing.T) {
	cfg := validTestConfig()
	cfg.ClinicalGIRAProvider = "none"
	cfg.OpenAIAPIKey = ""
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.ClinicalGIRAProvider = "openai"
	cfg.ClinicalGIRAModel = "synthetic"
	cfg.OpenAIAPIKey = "synthetic"
	if err := cfg.Validate(); err == nil {
		t.Fatal("remote product path enabled without experimental opt-in")
	}
}
