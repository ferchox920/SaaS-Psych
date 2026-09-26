package ingestionacceptance

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"sessionflow/apps/api/internal/usecase/transcription"
)

func TestStage2D1FourWorkerSoak(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	tr := &syntheticTranscriber{}
	reports := &syntheticReports{pool: h.pool, tenant: h.tenant}
	const batches = 3
	const batchSize = 100
	const workerCount = 4
	started := time.Now()
	for batch := 0; batch < batches; batch++ {
		for i := 0; i < batchSize; i++ {
			v := h.upload([]byte(fmt.Sprintf("synthetic soak audio %d/%d", batch, i)))
			h.enqueue(v.ID)
		}
		drainWorkers(t, h, tr, reports, workerCount)
		rows, err := h.pool.Query(context.Background(), `SELECT v.id FROM clinical_transcript_versions v WHERE v.tenant_id=$1 AND NOT EXISTS(SELECT 1 FROM clinical_ingestion_jobs j WHERE j.tenant_id=v.tenant_id AND j.transcript_version_id=v.id) ORDER BY v.id`, h.tenant)
		if err != nil {
			t.Fatal(err)
		}
		ids := []uuid.UUID{}
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		for _, id := range ids {
			h.want("POST", h.sessionPath("/analysis-jobs"), map[string]any{"transcript_version_id": id}, 202)
		}
		drainWorkers(t, h, tr, reports, workerCount)
		h.assertStorageConsistency()
	}
	if tr.duplicates != 0 {
		t.Fatalf("concurrent same-job executions=%d", tr.duplicates)
	}
	expected := batches * batchSize * 2
	if n := h.scalar(`SELECT count(*) FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND status='succeeded'`, h.tenant); n != expected {
		t.Fatalf("succeeded=%d expected=%d", n, expected)
	}
	if n := h.scalar(`SELECT count(*) FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND status<>'succeeded'`, h.tenant); n != 0 {
		t.Fatalf("stuck/failed=%d", n)
	}
	if n := h.scalar(`SELECT count(*) FROM clinical_ingestion_job_attempts WHERE tenant_id=$1 AND outcome='succeeded'`, h.tenant); n != expected {
		t.Fatalf("attempt count=%d", n)
	}
	for _, table := range []string{"clinical_transcript_versions", "clinical_ai_runs", "session_reports"} {
		if n := h.scalar(`SELECT count(*) FROM `+table+` WHERE tenant_id=$1`, h.tenant); n != batches*batchSize {
			t.Fatalf("%s outputs=%d", table, n)
		}
	}
	if n := h.scalar(`SELECT count(*) FROM (SELECT job_id FROM clinical_ingestion_results WHERE tenant_id=$1 GROUP BY job_id HAVING count(*)>1) duplicate_results`, h.tenant); n != 0 {
		t.Fatal("duplicate durable outputs")
	}
	t.Logf("workers=%d batches=%d submitted=%d succeeded=%d failed=0 retried=0 duplicate_executions=0 duplicate_outputs=0 stuck=0 recoveries=0 cancelled=0 duration=%s", workerCount, batches, expected, expected, time.Since(started))
}
func drainWorkers(t *testing.T, h *harness, tr ingestion.Transcriber, reports *syntheticReports, count int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		w := h.worker(tr, reports)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				worked, err := w.RunOne(ctx, h.tenant)
				if err != nil {
					errs <- err
					return
				}
				if !worked {
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
func TestStage2D1RunningBoundaryRaces(t *testing.T) {
	for _, boundary := range []string{"cancel", "revoke", "delete"} {
		t.Run(boundary, func(t *testing.T) {
			h := newHarness(t, "")
			h.grant(consent.Audio)
			grant := h.grant(consent.Transcription)
			v := h.upload([]byte("synthetic race audio"))
			j := h.enqueue(v.ID)
			entered, release := make(chan struct{}), make(chan struct{})
			tr := &syntheticTranscriber{hook: func(context.Context, uuid.UUID, int) error { close(entered); <-release; return nil }}
			result := make(chan error, 1)
			w := h.worker(tr, nil)
			go func() { _, err := w.RunOne(context.Background(), h.tenant); result <- err }()
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				close(release)
				t.Fatal("worker did not enter provider")
			}
			switch boundary {
			case "cancel":
				h.want("POST", "/api/v1/clinical-jobs/"+j.ID.String()+"/cancel", nil, 200)
			case "revoke":
				h.want("POST", "/api/v1/clients/"+h.client.String()+"/consents/"+grant.String()+"/revoke", nil, 200)
			case "delete":
				h.want("DELETE", "/api/v1/clinical-artifacts/"+v.ID.String(), nil, 200)
			}
			close(release)
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("late result accepted")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("race deadlock")
			}
			if h.job(j.ID).Status != "cancelled" || len(h.transcripts()) != 0 {
				t.Fatal("cancelled provider published a result")
			}
			h.assertStorageConsistency()
		})
	}
}
func TestStage2D1QueuedRevokeAndDelete(t *testing.T) {
	for _, boundary := range []string{"revoke", "delete"} {
		t.Run(boundary, func(t *testing.T) {
			h := newHarness(t, "")
			h.grant(consent.Audio)
			grant := h.grant(consent.Transcription)
			v := h.upload([]byte("synthetic queued race"))
			j := h.enqueue(v.ID)
			if boundary == "revoke" {
				h.want("POST", "/api/v1/clients/"+h.client.String()+"/consents/"+grant.String()+"/revoke", nil, 200)
			} else {
				h.want("DELETE", "/api/v1/clinical-artifacts/"+v.ID.String(), nil, 200)
			}
			if worked, err := h.worker(&syntheticTranscriber{}, nil).RunOne(context.Background(), h.tenant); worked || err != nil {
				t.Fatal("revoked/deleted job executed")
			}
			if h.job(j.ID).Status != "cancelled" {
				t.Fatal("unsafe queue state")
			}
			h.assertStorageConsistency()
		})
	}
}
func TestStage2D1RetryContention(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	v := h.upload([]byte("synthetic retry audio"))
	j := h.enqueue(v.ID)
	tr := &syntheticTranscriber{hook: func(_ context.Context, _ uuid.UUID, attempt int) error {
		if attempt == 1 {
			return transcription.ErrUnavailable
		}
		return nil
	}}
	if _, err := h.worker(tr, nil).RunOne(context.Background(), h.tenant); err == nil {
		t.Fatal("transient failure not recorded")
	}
	failed := h.job(j.ID)
	if failed.Status != "queued" || failed.Attempt != 1 {
		t.Fatal("retry not durably scheduled")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 21)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _, err := h.request("POST", "/api/v1/clinical-jobs/"+j.ID.String()+"/retry", nil)
			if err != nil {
				errs <- err
			} else if code != 202 && code != 409 {
				errs <- fmt.Errorf("retry HTTP status=%d", code)
			}
		}()
	}
	w := h.worker(tr, nil)
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for {
			worked, err := w.RunOne(ctx, h.tenant)
			if err != nil {
				errs <- err
				return
			}
			if worked {
				return
			}
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	final := h.job(j.ID)
	if final.Status != "succeeded" || final.Attempt != 2 || len(h.transcripts()) != 1 || tr.duplicates != 0 {
		t.Fatal("retry race duplicated or lost result")
	}
	if h.scalar(`SELECT count(*) FROM clinical_ingestion_job_attempts WHERE tenant_id=$1 AND job_id=$2`, h.tenant, j.ID) != 2 {
		t.Fatal("retry history lost")
	}
	h.want("POST", "/api/v1/clinical-jobs/"+j.ID.String()+"/retry", nil, 409)
	h.assertStorageConsistency()
	t.Log("automatic retry + 20 manual requests: attempts=2 durable_outputs=1")
}
func TestStage2D1NaturalLeaseExpiryRejectsStaleCompletion(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	v := h.upload([]byte("synthetic stale worker"))
	j := h.enqueue(v.ID)
	a, err := h.repo.Claim(context.Background(), h.tenant, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// Natural lease expiry, never SQL updates to force success/recovery.
	timer := time.NewTimer(time.Until(*a.LeaseUntil) + 50*time.Millisecond)
	<-timer.C
	if n, err := h.repo.RecoverExpired(context.Background(), h.tenant, 10); err != nil || n != 1 {
		t.Fatalf("natural recovery=%d error=%v", n, err)
	}
	if _, err = h.worker(&syntheticTranscriber{}, nil).RunOne(context.Background(), h.tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = h.repo.CompleteTranscription(context.Background(), a, syntheticContent(h.trHash)); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("stale completion=%v", err)
	}
	if h.job(j.ID).Status != "succeeded" || h.job(j.ID).Attempt != 2 || len(h.transcripts()) != 1 {
		t.Fatal("stale worker damaged legitimate output")
	}
	h.assertStorageConsistency()
}
