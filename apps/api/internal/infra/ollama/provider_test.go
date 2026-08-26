package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
	sessionreport "sessionflow/apps/api/internal/usecase/sessionreport"
)

func TestAnalyzeLiveUsesStructuredLocalChatRequest(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("expected /api/chat, got %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"message":{"content":"{\"mode\":\"live\",\"node\":\"Cambio afectivo\",\"hypothesis\":{\"text\":\"Lectura provisional.\",\"epistemic_level\":\"hypothesis\",\"traffic_light\":\"yellow\"},\"evidence\":[{\"source_id\":\"current_fragment\",\"kind\":\"fact\",\"summary\":\"Expresa duda.\"}],\"now\":\"explore\",\"caution\":\"No cerrar.\",\"suggested_interventions\":[\"¿Qué cambió?\"],\"therapist_meta\":null,\"risk\":{\"detected\":false,\"category\":null,\"requires_human_assessment\":false}}"},"done":true,"eval_count":20,"eval_duration":1000000000}` + "\n"))
	}))
	defer server.Close()

	provider, err := NewProvider(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	output, err := provider.AnalyzeLive(context.Background(), "system", clinicalanalysis.LiveRequest{Fragment: "Caso ficticio."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(output.JSON) || output.Metrics.EvalRate != 20 {
		t.Fatalf("unexpected output or metrics: %+v %s", output.Metrics, output.JSON)
	}
	if captured["model"] != "qwen3.5:9b" || captured["think"] != false || captured["stream"] != true {
		t.Fatalf("unexpected local chat options: %+v", captured)
	}
	if _, ok := captured["format"].(map[string]any); !ok {
		t.Fatalf("expected JSON schema format, got %T", captured["format"])
	}
}

func TestNewProviderRejectsRemoteEndpoint(t *testing.T) {
	_, err := NewProvider(testConfig("https://ollama.example.com"))
	if err == nil {
		t.Fatal("expected remote inference endpoint to be rejected")
	}
}

func TestAnalyzeLiveRepairsInvalidOutputOnlyOnce(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/x-ndjson")
		content := "not-json"
		if calls == 2 {
			content = `{"mode":"live","node":"Regla","hypothesis":{"text":"Provisional.","epistemic_level":"hypothesis","traffic_light":"red"},"evidence":[],"now":"explore","caution":"Explorar.","suggested_interventions":[],"therapist_meta":null,"risk":{"detected":false,"category":null,"requires_human_assessment":false}}`
		}
		encoded, _ := json.Marshal(map[string]any{"message": map[string]string{"content": content}, "done": true})
		_, _ = w.Write(append(encoded, '\n'))
	}))
	defer server.Close()
	provider, _ := NewProvider(testConfig(server.URL))

	output, err := provider.AnalyzeLive(context.Background(), "system", clinicalanalysis.LiveRequest{Fragment: "Caso ficticio."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !output.Metrics.Repaired {
		t.Fatalf("expected exactly one repair, calls=%d metrics=%+v", calls, output.Metrics)
	}
}

func TestGenerateSessionReportRepairsInvalidJSONOnlyOnce(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		content := "not-json"
		if calls == 2 {
			raw, _ := json.Marshal(map[string]any{"schema_version": sessionreport.SchemaVersion, "summary": "Ficticio", "facts": []any{}, "relevant_changes": []any{}, "interventions": []any{}, "patient_responses": []any{}, "affective_nodes": []any{}, "inference_candidates": []any{}, "hypothesis_candidates": []any{}, "safety_signals": []any{}, "open_questions": []any{}, "longitudinal_candidates": []any{}})
			content = string(raw)
		}
		encoded, _ := json.Marshal(map[string]any{"message": map[string]string{"content": content}, "done": true})
		_, _ = w.Write(append(encoded, '\n'))
	}))
	defer server.Close()
	provider, _ := NewProvider(testConfig(server.URL))
	output, err := provider.GenerateSessionReport(context.Background(), sessionreport.SystemPromptV1(), []byte(`{"session_text":"ficticio"}`), sessionreport.JSONSchemaV1())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !output.Repaired || !json.Valid(output.JSON) {
		t.Fatalf("calls=%d output=%#v", calls, output)
	}
}

func TestInterpretLongitudinalUsesIndependentStrictSchema(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		content := `{"operations":[],"uncertainties":[{"type":"insufficient_evidence","question":"Falta evidencia.","evidence_ids":[]}]}`
		encoded, _ := json.Marshal(map[string]any{"message": map[string]string{"content": content}, "done": true})
		_, _ = w.Write(append(encoded, '\n'))
	}))
	defer server.Close()
	provider, _ := NewProvider(testConfig(server.URL))
	input := longitudinal.InterpreterInput{SchemaVersion: longitudinal.PromptVersion, SessionReport: json.RawMessage(`{"schema_version":"session-report-v1.1"}`), CurrentState: longitudinal.State{Processes: []longitudinal.Process{}, UnassignedHypotheses: []longitudinal.Hypothesis{}, RecentEvents: []longitudinal.Event{}, ActiveEvidence: []longitudinal.Evidence{}, OpenProposals: []longitudinal.Diff{}}}
	out, err := provider.InterpretLongitudinal(context.Background(), longitudinal.SystemPromptV1(), input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = longitudinal.DecodeInterpreterResult(out.JSON); err != nil {
		t.Fatal(err)
	}
	if _, ok := captured["format"].(map[string]any); !ok {
		t.Fatalf("longitudinal generation must carry its strict schema: %#v", captured)
	}
}

func TestLiveAnalysisPreemptsBackgroundReview(t *testing.T) {
	reviewStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if len(request.Messages) > 1 && strings.Contains(request.Messages[1].Content, "session_text") {
			close(reviewStarted)
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		content := `{"mode":"live","node":"Dato nuevo","hypothesis":{"text":"Provisional.","epistemic_level":"hypothesis","traffic_light":"yellow"},"evidence":[{"source_id":"current_fragment","kind":"fact","summary":"Dato ficticio."}],"now":"explore","caution":"Contrastar.","suggested_interventions":[],"therapist_meta":null,"risk":{"detected":false,"category":null,"requires_human_assessment":false}}`
		encoded, _ := json.Marshal(map[string]any{"message": map[string]string{"content": content}, "done": true})
		_, _ = w.Write(append(encoded, '\n'))
	}))
	defer server.Close()
	provider, _ := NewProvider(testConfig(server.URL))
	reviewDone := make(chan error, 1)
	go func() {
		_, err := provider.ReviewSession(context.Background(), "review", clinicalanalysis.ReviewRequest{SessionText: "Material ficticio completo de sesión."}, nil)
		reviewDone <- err
	}()
	select {
	case <-reviewStarted:
	case <-time.After(time.Second):
		t.Fatal("review did not start")
	}
	if _, err := provider.AnalyzeLive(context.Background(), "live", clinicalanalysis.LiveRequest{Fragment: "Fragmento ficticio."}, nil); err != nil {
		t.Fatalf("live analysis should preempt review: %v", err)
	}
	select {
	case err := <-reviewDone:
		if err == nil {
			t.Fatal("expected canceled review")
		}
	case <-time.After(time.Second):
		t.Fatal("review was not canceled")
	}
}

func TestOllamaIntegrationFictitiousFixture(t *testing.T) {
	if os.Getenv("RUN_OLLAMA_INTEGRATION") != "1" {
		t.Skip("set RUN_OLLAMA_INTEGRATION=1 to test installed local model")
	}
	provider, err := NewProvider(Config{
		BaseURL: "http://127.0.0.1:11434", Model: "qwen3.5:9b", ContextTokens: 4096,
		Temperature: 0.1, Timeout: 90 * time.Second, KeepAlive: "15m", MaxOutputTokens: 384,
	})
	if err != nil {
		t.Fatal(err)
	}
	output, err := provider.AnalyzeLive(context.Background(), clinicalanalysis.LiveSystemPrompt(), clinicalanalysis.LiveRequest{
		Fragment: "Paciente ficticio: digo que quiero decidir por mí mismo, pero pregunto varias veces qué debería hacer.",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var result clinicalanalysis.Result
	if err := json.Unmarshal(output.JSON, &result); err != nil {
		t.Fatal(err)
	}
	result = clinicalanalysis.ApplyDeterministicGuards(clinicalanalysis.LiveRequest{Fragment: "Paciente ficticio: expresa ambivalencia."}, result)
	if err := clinicalanalysis.ValidateResult(result); err != nil {
		t.Fatalf("local model output failed contract: %v; output=%s", err, output.JSON)
	}
	t.Logf("first_token=%s total=%s rate=%.2f repaired=%t chars=%d", output.Metrics.FirstToken, output.Metrics.Total, output.Metrics.EvalRate, output.Metrics.Repaired, len(output.JSON))
}

func TestOllamaReviewIntegrationFictitiousFixture(t *testing.T) {
	if os.Getenv("RUN_CLINICAL_REVIEW_INTEGRATION") != "1" {
		t.Skip("set RUN_CLINICAL_REVIEW_INTEGRATION=1 to test local review mode")
	}
	provider, err := NewProvider(Config{BaseURL: "http://127.0.0.1:11434", Model: "qwen3.5:9b", ContextTokens: 4096, Temperature: 0.1, Timeout: 90 * time.Second, KeepAlive: "15m", MaxOutputTokens: 384, ReviewContextTokens: 8192, ReviewTemperature: 0.15, ReviewTimeout: 180 * time.Second, ReviewMaxOutputTokens: 1024})
	if err != nil {
		t.Fatal(err)
	}
	input := clinicalanalysis.ReviewRequest{SessionText: "current_session. Caso ficticio. Paciente: Quiero decidir por mí, aunque vuelvo a pedirte que elijas. Terapeuta: Pareces buscar permiso. Paciente: No exactamente; quiero ordenar opciones. Terapeuta: Entiendo, sigamos aclarando. Paciente: Eso sí me ayuda."}
	output, err := provider.ReviewSession(context.Background(), clinicalanalysis.ReviewSystemPrompt(), input, nil)
	if err != nil {
		t.Fatal(err)
	}
	var result clinicalanalysis.ReviewResult
	if err := json.Unmarshal(output.JSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.Mode != "review" || result.EmergingFormulation == "" {
		t.Fatalf("invalid review: %s", output.JSON)
	}
	t.Logf("first_token=%s total=%s rate=%.2f repaired=%t evidence=%d", output.Metrics.FirstToken, output.Metrics.Total, output.Metrics.EvalRate, output.Metrics.Repaired, len(result.Evidence))
}

func testConfig(baseURL string) Config {
	return Config{BaseURL: strings.TrimRight(baseURL, "/"), Model: "qwen3.5:9b", ContextTokens: 4096, Temperature: 0.1, Timeout: 5 * time.Second, KeepAlive: "1m", MaxOutputTokens: 180}
}
