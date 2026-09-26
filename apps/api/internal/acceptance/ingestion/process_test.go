package ingestionacceptance

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"sessionflow/apps/api/internal/infra/artifactstore"
	"sessionflow/apps/api/internal/infra/db"
	"sessionflow/apps/api/internal/usecase/ingestion"
)

func TestStage2D1WorkerProcess(t *testing.T) {
	mode := os.Getenv("STAGE2D1_CHILD_MODE")
	if mode == "" {
		t.Skip("subprocess harness only")
	}
	pool, err := db.NewPostgresPool(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := db.NewClinicalIngestionRepository(pool)
	tenant, err := uuid.Parse(os.Getenv("STAGE2D1_CHILD_TENANT"))
	if err != nil {
		t.Fatal(err)
	}
	if mode == "claim-and-die" {
		j, err := repo.Claim(context.Background(), tenant, 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(struct {
			ID         uuid.UUID
			LeaseUntil *time.Time
		}{j.ID, j.LeaseUntil})
		fmt.Println("STAGE2D1_CLAIMED " + string(raw))
		// The parent kills this process, leaving a real persisted running lease.
		for {
			time.Sleep(time.Second)
		}
	}
	store, err := artifactstore.New(os.Getenv("STAGE2D1_CHILD_ROOT"), "synthetic-v1", storageKey, 25*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := ingestion.NewWorker(repo, store, &syntheticTranscriber{}, &syntheticReports{pool: pool, tenant: tenant}, "synthetic-local-report", "stage2d1", "subprocess", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if worked, err := worker.RunOne(context.Background(), tenant); err != nil || !worked {
		t.Fatalf("subprocess worker: worked=%v error=%v", worked, err)
	}
}
func childCommand(t *testing.T, h *harness, mode string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestStage2D1WorkerProcess$", "-test.timeout=30s")
	cmd.Env = append(os.Environ(), "STAGE2D1_CHILD_MODE="+mode, "STAGE2D1_CHILD_TENANT="+h.tenant.String(), "STAGE2D1_CHILD_ROOT="+h.root)
	return cmd
}
func TestStage2D1ProcessDeathAndRestart(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	v := h.upload([]byte("synthetic process death"))
	j := h.enqueue(v.ID)
	child := childCommand(t, h, "claim-and-die")
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "STAGE2D1_CLAIMED ") {
				ready <- strings.TrimPrefix(scanner.Text(), "STAGE2D1_CLAIMED ")
				return
			}
		}
		ready <- ""
	}()
	var receipt struct {
		ID         uuid.UUID
		LeaseUntil *time.Time
	}
	select {
	case raw := <-ready:
		if raw == "" {
			t.Fatal("child exited without claim")
		}
		decode(t, []byte(raw), &receipt)
	case <-time.After(10 * time.Second):
		t.Fatal("child never claimed")
	}
	if receipt.ID != j.ID || receipt.LeaseUntil == nil {
		t.Fatal("child claimed wrong job")
	}
	if err = child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	if h.job(j.ID).Status != "running" {
		t.Fatal("process death erased durable job")
	}
	// Other workers continue useful work while the dead worker's lease is held.
	for i := 0; i < 50; i++ {
		v := h.upload([]byte(fmt.Sprintf("synthetic concurrent death fixture %d", i)))
		h.enqueue(v.ID)
	}
	drainWorkers(t, h, &syntheticTranscriber{}, &syntheticReports{pool: h.pool, tenant: h.tenant}, 4)
	timer := time.NewTimer(max(time.Until(*receipt.LeaseUntil)+50*time.Millisecond, time.Millisecond))
	<-timer.C
	replacement := childCommand(t, h, "run")
	// Under very slow CI a surviving worker may already have recovered the
	// naturally expired lease. Otherwise a fresh OS process does so now.
	if h.job(j.ID).Status != "succeeded" {
		if out, err := replacement.CombinedOutput(); err != nil {
			t.Fatalf("replacement failed: %v %s", err, out)
		}
	}
	if current := h.job(j.ID); current.Status != "succeeded" || current.Attempt != 2 || len(h.transcripts()) != 51 {
		t.Fatal("replacement duplicated or lost work")
	}
	if h.scalar(`SELECT count(*) FROM clinical_ingestion_job_attempts WHERE tenant_id=$1 AND job_id=$2 AND error_code='worker_lost'`, h.tenant, j.ID) != 1 {
		t.Fatal("death history lost")
	}
	// API object and worker process both restart; queued data must survive.
	next := h.upload([]byte("synthetic restart queued work"))
	nextJob := h.enqueue(next.ID)
	h.server.Close()
	h.startServer()
	replacement = childCommand(t, h, "run")
	if out, err := replacement.CombinedOutput(); err != nil {
		t.Fatalf("restart failed: %v %s", err, out)
	}
	if h.job(nextJob.ID).Status != "succeeded" || len(h.transcripts()) != 52 {
		t.Fatal("restart lost queued job")
	}
	h.assertStorageConsistency()
	t.Log("killed child process + four workers/50 concurrent jobs -> natural lease expiry -> one recovered output; API restart + fresh worker process preserved queue; total_outputs=52 duplicates=0")
}
