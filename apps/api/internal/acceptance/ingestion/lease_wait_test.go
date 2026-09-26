package ingestionacceptance

import (
	"context"
	"errors"
	"testing"
	"time"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

func TestStage2D1LeaseExpiresDuringRowLockWait(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	a := h.upload([]byte("synthetic lease wait"))
	h.enqueue(a.ID)
	ctx := context.Background()
	j, err := h.repo.Claim(ctx, h.tenant, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = blocker.Rollback(ctx) }()
	if _, err = blocker.Exec(ctx, `SELECT id FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, h.tenant, j.ID); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- h.repo.FailAttempt(ctx, j, "timeout") }()
	// The fault is a genuine competing lock, not a SQL timestamp rewrite.
	timer := time.NewTimer(time.Until(*j.LeaseUntil) + 100*time.Millisecond)
	<-timer.C
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-result:
		if !errors.Is(err, domainerrors.ErrConflict) {
			t.Fatalf("expired waiter retained write authority: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waiter deadlocked")
	}
	if current := h.job(j.ID); current.Status != "running" || current.Attempt != 1 {
		t.Fatal("stale failure changed durable state")
	}
	if _, err = h.worker(&syntheticTranscriber{}, nil).RunOne(ctx, h.tenant); err != nil {
		t.Fatal(err)
	}
	if h.job(j.ID).Status != "succeeded" || len(h.transcripts()) != 1 {
		t.Fatal("recovery after lock wait failed")
	}
}
