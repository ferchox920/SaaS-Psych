package db

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/infra/artifactstore"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"sessionflow/apps/api/internal/usecase/sessionreport"
)

func TestIngestionDurablePipelineAndRevocationPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newLongitudinalAcceptanceFixture(t, pool)
	store, err := artifactstore.New(t.TempDir(), "test-key", base64.StdEncoding.EncodeToString(make([]byte, 32)), 1024)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewClinicalIngestionRepository(pool)
	cr := NewClinicalConsentRepository(pool)
	cs := consent.NewService(cr, longitudinalAcceptanceAccess{true})
	svc := ingestion.NewService(repo, NewSessionReportRepository(pool), longitudinalAcceptanceAccess{true}, store, "test-key", 24*time.Hour)
	if _, err = svc.Upload(ctx, f.tenant, f.session, f.user, "wav", strings.NewReader("synthetic audio")); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("upload without consent: %v", err)
	}
	for _, scope := range []string{consent.Audio, consent.Transcription, consent.LocalAI} {
		if _, err = cs.Grant(ctx, f.tenant, f.client, f.user, scope, 1); err != nil {
			t.Fatal(err)
		}
	}
	audio, err := svc.Upload(ctx, f.tenant, f.session, f.user, "wav", strings.NewReader("synthetic audio"))
	if err != nil {
		t.Fatal(err)
	}
	if audio.Status != "available" {
		t.Fatal("audio unavailable")
	}
	job, created, err := svc.Enqueue(ctx, f.tenant, f.session, f.user, audio.ID, ingestion.Transcribe, ingestion.Hash("test-config"))
	if err != nil || !created {
		t.Fatalf("enqueue: %v", err)
	}
	again, created, err := svc.Enqueue(ctx, f.tenant, f.session, f.user, audio.ID, ingestion.Transcribe, ingestion.Hash("test-config"))
	if err != nil || created || again.ID != job.ID {
		t.Fatalf("idempotency: %v", err)
	}
	claimed, err := repo.Claim(ctx, f.tenant, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	content := ingestion.TranscriptContent{Text: "Transcripción ficticia con información para una prueba.", Segments: []ingestion.Segment{{Start: 0, End: 3, Text: "Transcripción ficticia con información para una prueba."}}, Language: "es", Engine: "faster-whisper", Model: "synthetic", EngineVersion: "test-v1", ConfigurationHash: job.ConfigurationHash}
	transcript, err := repo.CompleteTranscription(ctx, claimed, content)
	if err != nil {
		t.Fatal(err)
	}
	if transcript.Version != 1 || transcript.ArtifactHash != *audio.Hash {
		t.Fatal("transcript provenance lost")
	}
	if _, err = repo.CompleteTranscription(ctx, claimed, content); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("late completion: %v", err)
	}
	corrected, err := svc.CorrectTranscript(ctx, f.tenant, transcript.ID, f.user, "Corrección ASR ficticia con información para una prueba.", []ingestion.Segment{{Start: 0, End: 3, Text: "Corrección ASR ficticia con información para una prueba."}})
	if err != nil {
		t.Fatal(err)
	}
	if corrected.Version != 2 || corrected.Origin != "human_asr_correction" || *corrected.ParentID != transcript.ID {
		t.Fatal("correction identity lost")
	}
	old, err := repo.GetTranscript(ctx, f.tenant, transcript.ID)
	if err != nil || old.Text != content.Text {
		t.Fatal("original transcript mutated")
	}
	_, _, err = svc.Enqueue(ctx, f.tenant, f.session, f.user, corrected.ID, ingestion.Analyze, ingestion.Hash("local-report-config"))
	if err != nil {
		t.Fatal(err)
	}
	aj, err := repo.Claim(ctx, f.tenant, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	run, source, err := repo.StartAnalysis(ctx, aj, "local-synthetic-model", "test", "test-revision")
	if err != nil {
		t.Fatal(err)
	}
	if source.ID != corrected.ID {
		t.Fatal("wrong transcript input")
	}
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM clinical_ai_runs WHERE tenant_id=$1 AND id=$2`, f.tenant, run).Scan(&status); err != nil || status != "running" {
		t.Fatal("run not persisted before provider")
	}
	document, err := sessionreport.DecodeAndValidate(f.reportJSON)
	if err != nil {
		t.Fatal(err)
	}
	report, err := repo.CompleteAnalysis(ctx, aj, run, document)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "draft" {
		t.Fatal("report auto approved")
	}
	before := stage2BArtifactFingerprint(t, pool, f.tenant, f.client)
	if _, err = repo.CompleteAnalysis(ctx, aj, run, document); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatal("duplicate report published")
	}
	if before != stage2BArtifactFingerprint(t, pool, f.tenant, f.client) {
		t.Fatal("analysis changed longitudinal state")
	}
	// Another configuration permits a distinct run; cancellation must fence its result.
	_, _, err = svc.Enqueue(ctx, f.tenant, f.session, f.user, corrected.ID, ingestion.Analyze, ingestion.Hash("other-config"))
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := repo.Claim(ctx, f.tenant, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	secondRun, _, err := repo.StartAnalysis(ctx, revoked, "local-synthetic-model", "test", "test-revision")
	if err != nil {
		t.Fatal(err)
	}
	grants, err := cr.List(ctx, f.tenant, f.client)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range grants {
		if g.Scope == consent.LocalAI {
			if _, err = cs.Revoke(ctx, f.tenant, f.client, f.user, g.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = repo.CompleteAnalysis(ctx, revoked, secondRun, document); err == nil {
		t.Fatal("revoked analysis published")
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM clinical_ai_runs WHERE tenant_id=$1 AND id=$2`, f.tenant, secondRun).Scan(&status); err != nil || status != "cancelled" {
		t.Fatal("revoked run not terminal")
	}
	if _, err = svc.GetTranscript(ctx, uuid.New(), transcript.ID, f.user); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatal("cross-tenant transcript exposed")
	}
	deleted, err := svc.DeleteArtifact(ctx, f.tenant, audio.ID, f.user)
	if err != nil || deleted.Status != "deleted" {
		t.Fatalf("delete: %v", err)
	}
	if _, err = repo.GetTranscript(ctx, f.tenant, transcript.ID); err != nil {
		t.Fatal("raw deletion destroyed transcript provenance")
	}
	if _, _, err = svc.Enqueue(ctx, f.tenant, f.session, f.user, audio.ID, ingestion.Transcribe, ingestion.Hash("new-config")); err == nil {
		t.Fatal("deleted audio accepted")
	}
}
