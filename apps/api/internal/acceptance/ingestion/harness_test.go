package ingestionacceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	api "sessionflow/apps/api/internal/http"
	"sessionflow/apps/api/internal/http/handlers"
	middleware "sessionflow/apps/api/internal/http/middleware"
	"sessionflow/apps/api/internal/infra/artifactstore"
	"sessionflow/apps/api/internal/infra/db"
	"sessionflow/apps/api/internal/usecase/auth"
	"sessionflow/apps/api/internal/usecase/clinicalsession"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"sessionflow/apps/api/internal/usecase/sessionreport"
	"sessionflow/apps/api/internal/usecase/tenant"
)

const acceptanceKey = "stage2d1-synthetic-jwt-key-not-for-deployment"

var storageKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{17}, 32))

const syntheticReport = `{"schema_version":"session-report-v1.1","summary":"Sesión ficticia de aceptación técnica.","facts":[],"relevant_changes":[],"interventions":[],"patient_responses":[],"affective_nodes":[],"inference_candidates":[],"hypothesis_candidates":[],"safety_signals":[],"open_questions":[],"longitudinal_candidates":[]}`

type harness struct {
	t                              *testing.T
	pool                           *pgxpool.Pool
	repo                           *db.ClinicalIngestionRepository
	store                          *artifactstore.Store
	root                           string
	server                         *httptest.Server
	tenant, client, actor, session uuid.UUID
	token, trHash                  string
	model                          string
}

func newHarness(t *testing.T, trHash string) *harness {
	t.Helper()
	if os.Getenv("RUN_STAGE2D1_ACCEPTANCE") != "1" {
		t.Skip("set RUN_STAGE2D1_ACCEPTANCE=1 with isolated synthetic PostgreSQL")
	}
	ctx := context.Background()
	pool, err := db.NewPostgresPool(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	h := &harness{t: t, pool: pool, repo: db.NewClinicalIngestionRepository(pool), root: t.TempDir(), tenant: uuid.New(), client: uuid.New(), actor: uuid.New(), trHash: trHash}
	if h.trHash == "" {
		h.trHash = ingestion.Hash("stage2d1-synthetic-transcriber")
	}
	h.sql(`INSERT INTO tenants(id,name) VALUES($1,'Stage2D1 synthetic acceptance')`, h.tenant)
	h.sql(`INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'synthetic-unused-password')`, h.actor, h.tenant, h.actor.String()+"@example.invalid")
	h.sql(`INSERT INTO clients(id,tenant_id,fullname) VALUES($1,$2,'Synthetic acceptance client')`, h.client, h.tenant)
	h.sql(`INSERT INTO client_clinical_assignments(id,tenant_id,client_id,user_id,relationship,granted_by_user_id,starts_at) VALUES($1,$2,$3,$4,'treating',$4,NOW()-INTERVAL '1 hour')`, uuid.New(), h.tenant, h.client, h.actor)
	h.token, _, err = auth.NewTokenService(acceptanceKey, time.Hour).IssueAccessToken(h.actor, h.tenant, "member")
	if err != nil {
		t.Fatal(err)
	}
	h.store, err = artifactstore.New(h.root, "synthetic-v1", storageKey, 25*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	h.startServer()
	t.Cleanup(func() { h.server.Close() })
	raw := h.want("POST", "/api/v1/clinical-sessions", map[string]any{"client_id": h.client, "started_at": time.Now().UTC().Add(-time.Minute)}, 201)
	var session struct {
		ID          uuid.UUID  `json:"id"`
		Client      uuid.UUID  `json:"client_id"`
		Appointment *uuid.UUID `json:"appointment_id"`
	}
	decode(t, raw, &session)
	if session.ID == uuid.Nil || session.Client != h.client || session.Appointment != nil {
		t.Fatal("ClinicalSession root or optional appointment invariant failed")
	}
	h.session = session.ID
	h.want("POST", h.sessionPath("/complete"), nil, 200)
	return h
}
func (h *harness) startServer() {
	access := db.NewClinicalAccessRepository(h.pool)
	reports := db.NewSessionReportRepository(h.pool)
	consents := db.NewClinicalConsentRepository(h.pool)
	service := ingestion.NewService(h.repo, reports, access, h.store, "synthetic-v1", time.Hour)
	router := api.NewServer(api.ServerDeps{TenantMiddleware: middleware.RequireTenant(tenant.NewService(db.NewTenantRepository(h.pool))), AuthMiddleware: middleware.RequireAuth(acceptanceKey), ClinicalSessionHandler: handlers.NewClinicalSessionHandler(clinicalsession.NewService(db.NewClinicalSessionRepository(h.pool), access)), ClinicalConsentHandler: handlers.NewClinicalConsentHandler(consent.NewService(consents, access)), ClinicalIngestionHandler: handlers.NewClinicalIngestionHandler(service, 25*1024*1024, h.trHash, ingestion.Hash("stage2d1-local-report-config")), SessionReportHandler: handlers.NewSessionReportHandler(sessionreport.NewService(reports, access, nil, nil, "ollama", "synthetic-test", nil).WithConsent(consents))})
	h.server = httptest.NewServer(router)
}
func (h *harness) sql(query string, args ...any) {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), query, args...); err != nil {
		h.t.Fatal(err)
	}
}
func decode(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}
func (h *harness) sessionPath(suffix string) string {
	return "/api/v1/clinical-sessions/" + h.session.String() + suffix
}
func (h *harness) request(method, path string, body any) (int, []byte, error) {
	var input io.Reader
	ct := "application/json"
	if audio, ok := body.([]byte); ok {
		input = bytes.NewReader(audio)
		ct = "audio/wav"
	} else if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		input = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, input)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-Tenant-ID", h.tenant.String())
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", ct)
	client := http.Client{Timeout: time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4*1024*1024))
	return res.StatusCode, raw, err
}
func (h *harness) want(method, path string, body any, status int) []byte {
	h.t.Helper()
	code, raw, err := h.request(method, path, body)
	if err != nil || code != status {
		h.t.Fatalf("%s %s: code=%d expected=%d error=%v body=%s", method, path, code, status, err, raw)
	}
	return raw
}
func (h *harness) grant(scope string) uuid.UUID {
	h.t.Helper()
	var out consent.Grant
	decode(h.t, h.want("POST", "/api/v1/clients/"+h.client.String()+"/consents", map[string]any{"scope": scope, "definition_version": 1}, 201), &out)
	return out.ID
}
func (h *harness) grants() {
	for _, scope := range []string{consent.Audio, consent.Transcription, consent.LocalAI} {
		h.grant(scope)
	}
}
func (h *harness) upload(audio []byte) ingestion.Artifact {
	h.t.Helper()
	var out ingestion.Artifact
	decode(h.t, h.want("POST", h.sessionPath("/audio"), audio, 201), &out)
	return out
}
func (h *harness) enqueue(artifact uuid.UUID) ingestion.Job {
	h.t.Helper()
	var out ingestion.Job
	decode(h.t, h.want("POST", h.sessionPath("/transcriptions"), map[string]any{"artifact_id": artifact}, 202), &out)
	return out
}
func (h *harness) job(id uuid.UUID) ingestion.Job {
	h.t.Helper()
	var out ingestion.Job
	decode(h.t, h.want("GET", "/api/v1/clinical-jobs/"+id.String(), nil, 200), &out)
	return out
}
func (h *harness) scalar(query string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}
func (h *harness) transcripts() []ingestion.Transcript {
	var out struct {
		Items []ingestion.Transcript `json:"items"`
	}
	decode(h.t, h.want("GET", h.sessionPath("/transcripts"), nil, 200), &out)
	return out.Items
}
func (h *harness) worker(tr ingestion.Transcriber, report sessionreport.Provider) *ingestion.Worker {
	h.t.Helper()
	if report == nil {
		report = &syntheticReports{pool: h.pool, tenant: h.tenant}
	}
	model := h.model
	if model == "" {
		model = "synthetic-local-report"
	}
	w, err := ingestion.NewWorker(h.repo, h.store, tr, report, model, "stage2d1", "synthetic-build", 5*time.Minute)
	if err != nil {
		h.t.Fatal(err)
	}
	return w
}

type syntheticReports struct {
	pool   *pgxpool.Pool
	tenant uuid.UUID
	calls  atomic.Int32
}

func (p *syntheticReports) GenerateSessionReport(ctx context.Context, _ string, _ []byte, _ map[string]any) (sessionreport.ProviderOutput, error) {
	var n int
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM clinical_ai_runs WHERE tenant_id=$1 AND status='running' AND operation='generate_session_report'`, p.tenant).Scan(&n)
	if err != nil || n < 1 {
		return sessionreport.ProviderOutput{}, fmt.Errorf("AIRun must exist before inference")
	}
	p.calls.Add(1)
	return sessionreport.ProviderOutput{JSON: []byte(syntheticReport)}, nil
}

type syntheticTranscriber struct {
	mu         sync.Mutex
	active     map[uuid.UUID]int
	calls      map[uuid.UUID]int
	duplicates int
	hook       func(context.Context, uuid.UUID, int) error
}

func (tr *syntheticTranscriber) TranscribeDurable(ctx context.Context, _ []byte, _ string, id uuid.UUID, hash string) (ingestion.TranscriptContent, error) {
	tr.mu.Lock()
	if tr.active == nil {
		tr.active = map[uuid.UUID]int{}
		tr.calls = map[uuid.UUID]int{}
	}
	tr.active[id]++
	if tr.active[id] > 1 {
		tr.duplicates++
	}
	tr.calls[id]++
	attempt := tr.calls[id]
	tr.mu.Unlock()
	defer func() { tr.mu.Lock(); tr.active[id]--; tr.mu.Unlock() }()
	if tr.hook != nil {
		if err := tr.hook(ctx, id, attempt); err != nil {
			return ingestion.TranscriptContent{}, err
		}
	}
	return syntheticContent(hash), nil
}
func syntheticContent(hash string) ingestion.TranscriptContent {
	return ingestion.TranscriptContent{DurationSeconds: 2, ProcessingSeconds: 0.1, Text: "Audio completamente ficticio para verificar ingestión durable.", Segments: []ingestion.Segment{{Start: 0, End: 2, Text: "Audio completamente ficticio para verificar ingestión durable."}}, Language: "es", Engine: "faster-whisper", Model: "synthetic-queue-handler", EngineVersion: "test-v1", ConfigurationHash: hash}
}
func (h *harness) assertStorageConsistency() {
	h.t.Helper()
	rows, err := h.pool.Query(context.Background(), `SELECT storage_key,status FROM clinical_session_artifacts WHERE tenant_id=$1`, h.tenant)
	if err != nil {
		h.t.Fatal(err)
	}
	known := map[string]bool{}
	for rows.Next() {
		var key, status string
		if err = rows.Scan(&key, &status); err != nil {
			h.t.Fatal(err)
		}
		path := filepath.Join(h.root, bytesToFilename(key))
		_, statErr := os.Stat(path)
		if status == "available" && statErr != nil {
			h.t.Fatal("available artifact missing file")
		}
		if status == "deleted" && !os.IsNotExist(statErr) {
			h.t.Fatal("deleted artifact retains file")
		}
		known[filepath.Base(path)] = true
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		h.t.Fatal(err)
	}
	entries, err := os.ReadDir(h.root)
	if err != nil {
		h.t.Fatal(err)
	}
	for _, e := range entries {
		if !known[e.Name()] {
			h.t.Fatalf("unmanaged storage entry %s", e.Name())
		}
	}
}
func bytesToFilename(key string) string {
	return string(bytes.ReplaceAll([]byte(key), []byte("/"), []byte("_"))) + ".audio.enc"
}
