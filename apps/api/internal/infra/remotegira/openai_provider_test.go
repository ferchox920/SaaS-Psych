package remotegira

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
)

func validProviderFixture() (longitudinal.GIRAProviderRequest, []byte) {
	tenant, client, processID, evidenceID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	strategy := longitudinal.TherapeuticStrategy{Targets: []longitudinal.Target{}, Goals: []longitudinal.Goal{}, Rationales: []longitudinal.TherapeuticRationale{}, GIRAs: []longitudinal.GIRA{}}
	snapshot := longitudinal.GIRABuilderInput{SchemaVersion: longitudinal.GIRAPromptVersion, SelectedProcess: longitudinal.Process{ID: processID, TenantID: tenant, ClientID: client, Title: "Proceso ficticio", Description: "Evitación mantenida por alivio", ApprovalStatus: "approved", ClinicalStatus: "active", Version: 1, TherapeuticStrategy: &strategy}, ApprovedEvidence: []longitudinal.Evidence{{ID: evidenceID, TenantID: tenant, ClientID: client, EpistemicType: "patient_report", Statement: "Evita una conversación y obtiene alivio inmediato", Status: "active", Version: 1}}, ApprovedEvents: []longitudinal.Event{}, ApprovedHypotheses: []longitudinal.Hypothesis{}, CurrentStrategy: strategy, ApproachRegistry: []longitudinal.ApproachDefinition{{ID: uuid.New(), Slug: "cbt", Version: 1, Status: "active", TargetDomains: []string{"behavioral_pattern"}, CoreMechanisms: []string{"contrastar predicciones"}}}, TechniqueRegistry: []longitudinal.TechniqueDefinition{{ID: uuid.New(), Slug: "behavioral_experiment", Version: 1, ApproachSlug: "cbt", ApproachVersion: 1, Status: "active", TargetDomains: []string{"behavioral_pattern"}, Mechanism: "contrastar predicciones"}}, StateVersion: 1}
	request := longitudinal.NewGIRAProviderRequest(longitudinal.BuildGIRAGenerationRequest(snapshot))
	technique := "behavioral_experiment"
	one := 1
	proposal := longitudinal.GIRASemanticProposal{
		Targets:        []longitudinal.GIRASemanticTarget{{Ref: "target_1", Title: "Evitación", Description: "Reversión por alivio", TargetType: "behavioral_pattern", EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}, EventRefs: []string{}}},
		Goals:          []longitudinal.GIRASemanticGoal{{Ref: "goal_1", Title: "Iniciar conversación", Description: "Iniciar una conversación acordada", GoalType: "behavior_change", Priority: "high", TargetRefs: []string{"target_1"}}},
		Indicators:     []longitudinal.GIRASemanticIndicator{{Ref: "indicator_1", GoalRef: "goal_1", Description: "Inicia conversación", IndicatorType: "qualitative"}},
		Rationales:     []longitudinal.GIRASemanticRationale{{Ref: "rationale_1", TargetRef: "target_1", GoalRef: "goal_1", ApproachSlug: "cbt", ApproachVersion: 1, TechniqueSlug: &technique, TechniqueVersion: &one, Rationale: "Contrastar predicción de rechazo", ExpectedEffect: "Reducir evitación", EvidenceRefs: []string{"evidence_1"}, HypothesisRefs: []string{}}},
		GIRA:           &longitudinal.GIRASemanticGIRA{Ref: "gira_1", Title: "Ruta", Summary: "Estrategia mínima", TargetRefs: []string{"target_1"}, GoalRefs: []string{"goal_1"}, RationaleRefs: []string{"rationale_1"}},
		Phases:         []longitudinal.GIRASemanticPhase{{Ref: "phase_1", GIRARef: "gira_1", Position: 1, Title: "Inicio", Description: "Experimento", GoalRefs: []string{"goal_1"}, RationaleRefs: []string{"rationale_1"}, IndicatorRefs: []string{"indicator_1"}}},
		IndicatorLinks: []longitudinal.GIRASemanticIndicatorLink{}, Uncertainties: []longitudinal.GIRASemanticUncertainty{},
	}
	raw, _ := json.Marshal(proposal)
	return request, raw
}

func openAIResponseBody(output string) []byte {
	raw, _ := json.Marshal(map[string]any{"id": "req_synthetic_1", "model": "gpt-test-snapshot", "output_text": output, "usage": map[string]any{"input_tokens": 100, "output_tokens": 50, "input_tokens_details": map[string]any{"cached_tokens": 10}, "output_tokens_details": map[string]any{"reasoning_tokens": 5}}})
	return raw
}

func newFakeProvider(t *testing.T, handler http.HandlerFunc, timeout time.Duration) (*OpenAIGIRABuilderProvider, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	transport, err := NewHTTPTransport(HTTPTransportConfig{BaseURL: server.URL, APIKey: "synthetic-api-key", Timeout: timeout, Client: server.Client()})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	provider, err := NewOpenAIGIRABuilderProvider(transport, OpenAIConfig{Model: "gpt-test", MaxOutputTokens: 1536, ProviderRegion: "test", DataControlMode: "zero_data_retention", DataControlStatus: "confirmed", Pricing: PricingConfig{Version: "test-prices-v1", InputUSDPerMillion: 4, CachedInputUSDPerMillion: .4, OutputUSDPerMillion: 20}})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return provider, server
}

func TestOpenAIProviderUsesMinimizedContextAndStructuredOutput(t *testing.T) {
	request, valid := validProviderFixture()
	var calls atomic.Int32
	provider, server := newFakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer synthetic-api-key" {
			t.Errorf("unexpected transport request path=%s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Error(err)
		}
		if payload["store"] != false || payload["instructions"] != longitudinal.GIRASystemPromptV1() {
			t.Errorf("unsafe or changed request: %s", body)
		}
		text, _ := payload["text"].(map[string]any)
		format, _ := text["format"].(map[string]any)
		if format["type"] != "json_schema" || format["strict"] != true {
			t.Errorf("strict structured output missing: %s", body)
		}
		input, _ := payload["input"].(string)
		if strings.Contains(input, "tenant_id") || strings.Contains(input, "client_id") || strings.Contains(input, "33333333-") {
			t.Errorf("internal identifier reached fake remote: %s", input)
		}
		w.Header().Set("x-request-id", "req_header_1")
		_, _ = w.Write(openAIResponseBody(string(valid)))
	}, time.Second)
	defer server.Close()
	out, err := provider.BuildGIRA(context.Background(), longitudinal.GIRASystemPromptV1(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || out.Metadata.RemoteRequestID != "req_header_1" || out.Metadata.ModelSnapshot != "gpt-test-snapshot" || out.Metadata.EstimatedCostMicros != 1364 {
		t.Fatalf("unexpected output metadata calls=%d metadata=%+v", calls.Load(), out.Metadata)
	}
}

func TestOpenAIProviderAllowsExactlyOneRepair(t *testing.T) {
	request, valid := validProviderFixture()
	var calls atomic.Int32
	provider, server := newFakeProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		call := calls.Add(1)
		// This test asserts positive measured latency. A zero-delay loopback
		// response may fall within one Windows clock tick; make latency explicit.
		time.Sleep(2 * time.Millisecond)
		output := "not-json"
		if call == 2 {
			output = string(valid)
		}
		_, _ = w.Write(openAIResponseBody(output))
	}, time.Second)
	defer server.Close()
	out, err := provider.BuildGIRA(context.Background(), longitudinal.GIRASystemPromptV1(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || !out.Metrics.Repaired || out.Metrics.PrimaryDuration <= 0 || out.Metrics.RepairDuration != out.Metrics.Total-out.Metrics.PrimaryDuration || out.Metadata.RepairAttempts != 1 || out.Metadata.InputTokens != 200 || out.Metadata.OutputTokens != 100 {
		t.Fatalf("bounded repair metadata mismatch calls=%d out=%+v", calls.Load(), out)
	}
}

func TestOpenAIProviderRepairsSemanticallyInvalidStructuredOutput(t *testing.T) {
	request, valid := validProviderFixture()
	var proposal longitudinal.GIRASemanticProposal
	if err := json.Unmarshal(valid, &proposal); err != nil {
		t.Fatal(err)
	}
	proposal.Targets[0].EvidenceRefs = []string{"evidence_not_in_context"}
	invalid, _ := json.Marshal(proposal)
	var calls atomic.Int32
	provider, server := newFakeProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			_, _ = w.Write(openAIResponseBody(string(invalid)))
			return
		}
		_, _ = w.Write(openAIResponseBody(string(valid)))
	}, time.Second)
	defer server.Close()
	out, err := provider.BuildGIRA(context.Background(), longitudinal.GIRASystemPromptV1(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || !out.Metrics.Repaired || out.Metrics.RepairReason != "invalid_semantic_output" || out.Metadata.RepairAttempts != 1 {
		t.Fatalf("semantic repair was not bounded and recorded: calls=%d out=%+v", calls.Load(), out)
	}
}

func TestOpenAIProviderRejectsFailedRepairWithoutThirdAttempt(t *testing.T) {
	request, _ := validProviderFixture()
	var calls atomic.Int32
	provider, server := newFakeProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write(openAIResponseBody(`{"targets":[]}`))
	}, time.Second)
	defer server.Close()
	_, err := provider.BuildGIRA(context.Background(), longitudinal.GIRASystemPromptV1(), request, nil)
	if err == nil || ErrorCode(err) != "invalid_model_output" || calls.Load() != 2 {
		t.Fatalf("expected one failed repair, calls=%d err=%v", calls.Load(), err)
	}
}

func TestOpenAIProviderRepairTransportFailureIsTerminal(t *testing.T) {
	request, _ := validProviderFixture()
	var calls atomic.Int32
	provider, server := newFakeProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			_, _ = w.Write(openAIResponseBody("not-json"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("SECRET_REPAIR_BODY"))
	}, time.Second)
	defer server.Close()
	_, err := provider.BuildGIRA(context.Background(), longitudinal.GIRASystemPromptV1(), request, nil)
	if err == nil || ErrorCode(err) != "provider_unavailable" || calls.Load() != 2 {
		t.Fatalf("repair transport failure was not terminal: calls=%d err=%v", calls.Load(), err)
	}
	if strings.Contains(err.Error(), "SECRET_REPAIR_BODY") {
		t.Fatalf("repair error leaked provider body: %q", err.Error())
	}
}

func TestRemoteFailureMatrixAndErrorSanitization(t *testing.T) {
	request, valid := validProviderFixture()
	tests := []struct {
		name        string
		status      int
		delay       time.Duration
		body        string
		expected    string
		expectCalls int32
	}{
		{name: "rate_limit", status: http.StatusTooManyRequests, body: "SECRET_CLINICAL_BODY api-key-secret", expected: "rate_limited", expectCalls: 1},
		{name: "server_error", status: http.StatusBadGateway, body: "SECRET_CLINICAL_BODY", expected: "provider_unavailable", expectCalls: 1},
		{name: "timeout", status: http.StatusOK, delay: 80 * time.Millisecond, body: string(openAIResponseBody(string(valid))), expected: "timeout", expectCalls: 1},
		{name: "invalid_provider_json", status: http.StatusOK, body: "SECRET_CLINICAL_BODY", expected: "invalid_provider_response", expectCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			provider, server := newFakeProvider(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				if test.delay > 0 {
					time.Sleep(test.delay)
				}
				w.Header().Set("x-request-id", "safe-request-id")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}, 20*time.Millisecond)
			defer server.Close()
			_, err := provider.BuildGIRA(context.Background(), longitudinal.GIRASystemPromptV1(), request, nil)
			if err == nil || ErrorCode(err) != test.expected || calls.Load() != test.expectCalls {
				t.Fatalf("failure mismatch code=%s calls=%d err=%v", ErrorCode(err), calls.Load(), err)
			}
			message := err.Error()
			for _, forbidden := range []string{"SECRET_CLINICAL_BODY", "api-key-secret", string(valid), "Evitación mantenida"} {
				if strings.Contains(message, forbidden) {
					t.Fatalf("remote error leaked sensitive content: %q", message)
				}
			}
		})
	}
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("synthetic network failure containing SECRET_PAYLOAD")
}

func TestRemoteNetworkFailureIsNormalizedWithoutCauseLeak(t *testing.T) {
	transport, err := NewHTTPTransport(HTTPTransportConfig{BaseURL: "https://example.invalid", APIKey: "secret-key", Timeout: time.Second, Client: &http.Client{Transport: failingRoundTripper{}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = transport.Do(context.Background(), TransportRequest{Path: "/v1/responses", Body: []byte("SECRET_REQUEST_BODY")})
	if err == nil || ErrorCode(err) != "provider_unavailable" {
		t.Fatalf("unexpected network error: %v", err)
	}
	if strings.Contains(err.Error(), "SECRET_PAYLOAD") || strings.Contains(err.Error(), "SECRET_REQUEST_BODY") || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("network error leaked content: %q", err.Error())
	}
}
