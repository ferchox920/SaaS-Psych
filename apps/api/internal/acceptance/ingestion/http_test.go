package ingestionacceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"sessionflow/apps/api/internal/infra/ollama"
	"sessionflow/apps/api/internal/infra/transcriber"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"sessionflow/apps/api/internal/usecase/sessionreport"
)

func TestStage2D1HTTPIngestion(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	runHTTPPipeline(t, h, []byte("synthetic audio queue fixture"), &syntheticTranscriber{}, nil)
}
func runHTTPPipeline(t *testing.T, h *harness, audio []byte, tr ingestion.Transcriber, reports sessionreport.Provider) {
	t.Helper()
	artifact := h.upload(audio)
	if artifact.TenantID != h.tenant || artifact.ClientID != h.client || artifact.SessionID != h.session || artifact.Status != "available" || artifact.Hash == nil {
		t.Fatal("HTTP artifact identity")
	}
	job := h.enqueue(artifact.ID)
	w := h.worker(tr, reports)
	if ok, err := w.RunOne(context.Background(), h.tenant); !ok || err != nil {
		t.Fatalf("real transcription worker: %v", err)
	}
	if h.job(job.ID).Status != "succeeded" {
		t.Fatal("transcription did not succeed")
	}
	versions := h.transcripts()
	if len(versions) != 1 {
		t.Fatal("expected exactly one transcript")
	}
	v := versions[0]
	canonical, _ := json.Marshal(v.TranscriptContent)
	if ingestion.Hash(string(canonical)) != v.ContentHash {
		t.Fatal("persisted transcript content cannot reproduce its provenance hash")
	}
	if v.TenantID != h.tenant || v.ClientID != h.client || v.SessionID != h.session || v.Version != 1 || v.ArtifactID != artifact.ID || v.ArtifactHash != *artifact.Hash || v.ArtifactVersion != 1 {
		t.Fatal("HTTP transcript provenance")
	}
	var analysis ingestion.Job
	decode(t, h.want("POST", h.sessionPath("/analysis-jobs"), map[string]any{"transcript_version_id": v.ID}, 202), &analysis)
	if ok, err := w.RunOne(context.Background(), h.tenant); !ok || err != nil {
		t.Fatalf("real analysis worker: %v", err)
	}
	if h.job(analysis.ID).Status != "succeeded" {
		t.Fatal("analysis did not succeed")
	}
	var list struct {
		Items []sessionreport.Report `json:"items"`
	}
	decode(t, h.want("GET", h.sessionPath("/reports"), nil, 200), &list)
	if len(list.Items) != 1 || list.Items[0].Status != "draft" || list.Items[0].SourceAIRunID == nil {
		t.Fatal("report not exact unapproved draft")
	}
	run := *list.Items[0].SourceAIRunID
	if h.scalar(`SELECT count(*) FROM clinical_ingestion_run_attempts a JOIN clinical_ai_runs r ON r.tenant_id=a.tenant_id AND r.id=a.ai_run_id JOIN clinical_ingestion_results x ON x.tenant_id=a.tenant_id AND x.job_id=a.job_id WHERE a.tenant_id=$1 AND a.client_id=$2 AND a.session_id=$3 AND a.job_id=$4 AND a.ai_run_id=$5 AND a.transcript_version_id=$6 AND r.status='succeeded' AND x.report_id=$7 AND r.input_hash IS NOT NULL AND r.context_hash IS NOT NULL AND r.output_hash IS NOT NULL`, h.tenant, h.client, h.session, analysis.ID, run, v.ID, list.Items[0].ID) != 1 {
		t.Fatal("DB provenance chain broken")
	}
	if h.scalar(`SELECT count(*) FROM clinical_ingestion_authorizations WHERE tenant_id=$1 AND client_id=$2 AND entity_id IN($3,$4,$5) AND definition_version=1`, h.tenant, h.client, artifact.ID, job.ID, analysis.ID) < 6 {
		t.Fatal("missing historical consent snapshots")
	}
	for _, table := range []string{"clinical_evidence", "clinical_events", "clinical_processes", "clinical_hypotheses", "clinical_diffs"} {
		if h.scalar(`SELECT count(*) FROM `+table+` WHERE tenant_id=$1 AND client_id=$2`, h.tenant, h.client) != 0 {
			t.Fatal("ingestion mutated longitudinal state")
		}
	}
	h.assertStorageConsistency()
	t.Log("HTTP consent -> session -> encrypted audio -> worker -> transcript -> analysis -> AIRun -> draft report: PASS")
}
func TestStage2D1HTTPRealWhisperAndOllama(t *testing.T) {
	if os.Getenv("RUN_STAGE2D1_REAL") != "1" {
		t.Skip("set RUN_STAGE2D1_REAL=1 for real local providers with synthetic TTS")
	}
	base := os.Getenv("SYNTHETIC_TRANSCRIBER_URL")
	if base == "" {
		base = "http://127.0.0.1:8092"
	}
	tr, err := transcriber.NewProvider(base, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// The synthetic sidecar is prepared by the acceptance operator. Its exact
	// configuration hash is supplied, never inferred or replaced by a fake value.
	hash := os.Getenv("SYNTHETIC_TRANSCRIPTION_CONFIGURATION_HASH")
	if !ingestion.ValidHash(hash) {
		t.Fatal("synthetic sidecar configuration hash required")
	}
	audio, err := os.ReadFile(os.Getenv("SYNTHETIC_TRANSCRIPTION_AUDIO"))
	if err != nil {
		t.Fatal("synthetic TTS audio required")
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "qwen3.5:9b"
	}
	target, _ := url.Parse("http://127.0.0.1:11434")
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = func(response *http.Response) error {
		if response.StatusCode >= 400 {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
			_ = response.Body.Close()
			t.Logf("synthetic local Ollama transport diagnosis: %s", body)
			response.Body = io.NopCloser(bytes.NewReader(body))
		}
		return nil
	}
	observed := httptest.NewServer(proxy)
	defer observed.Close()
	provider, err := ollama.NewProvider(ollama.Config{BaseURL: observed.URL, Model: model, ContextTokens: 4096, Temperature: 0.1, Timeout: 45 * time.Second, KeepAlive: "15m", MaxOutputTokens: 384, ReviewContextTokens: 8192, ReviewTemperature: 0.15, ReviewTimeout: 180 * time.Second, ReviewMaxOutputTokens: 1024})
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, hash)
	h.model = model
	h.grants()
	runHTTPPipeline(t, h, audio, tr, observedLocalReports{t, provider})
}

type observedLocalReports struct {
	t        *testing.T
	provider sessionreport.Provider
}

func (p observedLocalReports) GenerateSessionReport(ctx context.Context, prompt string, input []byte, schema map[string]any) (sessionreport.ProviderOutput, error) {
	out, err := p.provider.GenerateSessionReport(ctx, prompt, input, schema)
	if err != nil {
		p.t.Logf("real local provider failure: %v", err)
		return out, err
	}
	if _, e := sessionreport.DecodeAndValidate(out.JSON); e != nil {
		p.t.Logf("synthetic report rejected by typed validator: %v (bytes=%d repaired=%v)", e, len(out.JSON), out.Repaired)
	}
	return out, err
}
func TestStage2D1NegativeHTTP(t *testing.T) {
	h := newHarness(t, "")
	t.Run("no-audio-consent", func(t *testing.T) {
		h.want("POST", h.sessionPath("/audio"), []byte("synthetic"), 403)
		if h.scalar(`SELECT count(*) FROM clinical_session_artifacts WHERE tenant_id=$1`, h.tenant) != 0 || h.scalar(`SELECT count(*) FROM clinical_ingestion_jobs WHERE tenant_id=$1`, h.tenant) != 0 {
			t.Fatal("denial produced state")
		}
	})
	h.grant(consent.Audio)
	artifact := h.upload([]byte("synthetic audio"))
	t.Run("no-transcription-consent", func(t *testing.T) {
		h.want("POST", h.sessionPath("/transcriptions"), map[string]any{"artifact_id": artifact.ID}, 403)
		if h.scalar(`SELECT count(*) FROM clinical_ingestion_jobs WHERE tenant_id=$1`, h.tenant) != 0 {
			t.Fatal("denial queued job")
		}
	})
	h.grant(consent.Transcription)
	t.Run("unknown-fields-rejected", func(t *testing.T) {
		h.want("POST", h.sessionPath("/transcriptions"), map[string]any{"artifact_id": artifact.ID, "storage_path": "C:/synthetic/outside.wav"}, 400)
	})
	t.Run("foreign-session-and-client", func(t *testing.T) {
		other := newHarness(t, "")
		h.want("POST", "/api/v1/clinical-sessions/"+other.session.String()+"/audio", []byte("synthetic"), 404)
		h.want("POST", h.sessionPath("/transcriptions"), map[string]any{"artifact_id": uuid.New()}, 404)
		denied := uuid.New()
		h.sql(`INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Foreign synthetic client')`, denied, h.tenant)
		h.want("POST", "/api/v1/clients/"+denied.String()+"/consents", map[string]any{"scope": consent.Audio, "definition_version": 1}, 403)
	})
	t.Run("queued-cancel", func(t *testing.T) {
		job := h.enqueue(artifact.ID)
		h.want("POST", "/api/v1/clinical-jobs/"+job.ID.String()+"/cancel", nil, 200)
		tr := &syntheticTranscriber{}
		if worked, err := h.worker(tr, nil).RunOne(context.Background(), h.tenant); err != nil || worked {
			t.Fatal("cancelled job claimed")
		}
		if h.job(job.ID).Status != "cancelled" {
			t.Fatal("cancel state changed")
		}
	})
	h.assertStorageConsistency()
}
func TestStage2D1ConcurrentHTTPIdempotency(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	artifact := h.upload([]byte("synthetic idempotency audio"))
	const requests = 24
	var wg sync.WaitGroup
	results := make(chan struct {
		code int
		id   uuid.UUID
		err  error
	}, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, raw, err := h.request("POST", h.sessionPath("/transcriptions"), map[string]any{"artifact_id": artifact.ID})
			var j ingestion.Job
			if err == nil {
				err = json.Unmarshal(raw, &j)
			}
			results <- struct {
				code int
				id   uuid.UUID
				err  error
			}{code, j.ID, err}
		}()
	}
	wg.Wait()
	close(results)
	var id uuid.UUID
	created := 0
	for result := range results {
		if result.err != nil || (result.code != 200 && result.code != 202) {
			t.Fatalf("request race: %+v", result)
		}
		if result.code == 202 {
			created++
		}
		if id == uuid.Nil {
			id = result.id
		}
		if id != result.id {
			t.Fatal("idempotency created divergent jobs")
		}
	}
	if created != 1 {
		t.Fatalf("created=%d", created)
	}
	if _, err := h.worker(&syntheticTranscriber{}, nil).RunOne(context.Background(), h.tenant); err != nil {
		t.Fatal(err)
	}
	if len(h.transcripts()) != 1 {
		t.Fatal("duplicate transcript")
	}
	h.assertStorageConsistency()
	t.Logf("requests=%d created=1 reused=%d durable_outputs=1", requests, requests-1)
}
