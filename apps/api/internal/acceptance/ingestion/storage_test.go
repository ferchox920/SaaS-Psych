package ingestionacceptance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"sessionflow/apps/api/internal/usecase/ingestion"
)

func TestStage2D1StoragePartialFailureAndRecovery(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	ctx := context.Background()
	// Inject only the availability UPDATE failure for this synthetic tenant.
	// HTTP upload still traverses real reserve, encrypted write and compensation.
	fn := "stage2d1_fault_" + strings.ReplaceAll(h.tenant.String(), "-", "")
	h.sql(`CREATE FUNCTION ` + fn + `() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.tenant_id='` + h.tenant.String() + `'::uuid AND NEW.status='available' THEN RAISE EXCEPTION 'synthetic finalize fault'; END IF; RETURN NEW; END $$`)
	h.sql(`CREATE TRIGGER ` + fn + ` BEFORE UPDATE ON clinical_session_artifacts FOR EACH ROW EXECUTE FUNCTION ` + fn + `() `)
	t.Cleanup(func() {
		h.sql(`DROP TRIGGER IF EXISTS ` + fn + ` ON clinical_session_artifacts`)
		h.sql(`DROP FUNCTION IF EXISTS ` + fn + `()`)
	})
	h.want("POST", h.sessionPath("/audio"), []byte("synthetic partial upload"), 400)
	if h.scalar(`SELECT count(*) FROM clinical_session_artifacts WHERE tenant_id=$1 AND status='deleted'`, h.tenant) != 1 {
		t.Fatal("failed finalize not compensated")
	}
	h.assertStorageConsistency()
	h.sql(`DROP TRIGGER ` + fn + ` ON clinical_session_artifacts`)
	h.sql(`DROP FUNCTION ` + fn + `() `)

	v := h.upload([]byte("synthetic interrupted deletion"))
	actual, err := h.repo.BeginDeleteArtifact(ctx, h.tenant, v.ID, h.actor)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.store.Delete(ctx, actual.StorageKey); err != nil {
		t.Fatal(err)
	}
	// Crash boundary: physical removal happened, DB completion did not.
	maintenance := ingestion.NewMaintenance(h.repo, h.store)
	if _, _, err = maintenance.RunBatch(ctx, h.tenant, uuid.Nil, 0); err != nil {
		t.Fatal(err)
	}
	if err = h.repo.FinishDeleteArtifact(ctx, h.tenant, v.ID); err != nil {
		t.Fatal(err)
	}
	if h.scalar(`SELECT count(*) FROM audit_logs WHERE tenant_id=$1 AND entity_id=$2 AND action='clinical_audio.deleted'`, h.tenant, v.ID) != 1 {
		t.Fatal("completion audit missing/duplicated")
	}
	h.assertStorageConsistency()

	missing := h.upload([]byte("synthetic missing file"))
	job := h.enqueue(missing.ID)
	stored, err := h.repo.GetArtifact(ctx, h.tenant, missing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.store.Delete(ctx, stored.StorageKey); err != nil {
		t.Fatal(err)
	}
	if _, _, err = maintenance.RunBatch(ctx, h.tenant, uuid.Nil, 0); err != nil {
		t.Fatal(err)
	}
	if h.job(job.ID).Status != "cancelled" {
		t.Fatal("missing source did not cancel job")
	}
	if h.scalar(`SELECT count(*) FROM clinical_session_artifacts WHERE tenant_id=$1 AND id=$2 AND status='missing'`, h.tenant, missing.ID) != 1 {
		t.Fatal("missing file not classified")
	}
	h.want("DELETE", "/api/v1/clinical-artifacts/"+missing.ID.String(), nil, 200)
	h.assertStorageConsistency()
	t.Log("finalize rollback/compensation; interrupted deletion recovery; missing-source cancellation; idempotent deletion audit: PASS")
}

func TestStage2D1StorageInventoryAndRetention(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	ctx := context.Background()
	maintenance := ingestion.NewMaintenance(h.repo, h.store)
	// Explicit synthetic retention only. It is not a product/legal default.
	id := uuid.New()
	key := strings.Join([]string{h.tenant.String(), h.client.String(), h.session.String(), id.String()}, "/")
	v, err := h.repo.ReserveArtifact(ctx, ingestion.Artifact{ID: id, TenantID: h.tenant, ClientID: h.client, SessionID: h.session, ActorID: h.actor, StorageKey: key, KeyID: "synthetic-v1", MediaType: "wav", RetentionUntil: time.Now().Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := h.store.WriteAudio(ctx, key, strings.NewReader("synthetic retention audio"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.repo.FinalizeArtifact(ctx, v, receipt); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTimer(max(time.Until(v.RetentionUntil)+20*time.Millisecond, time.Millisecond))
	<-timer.C
	if _, _, err = maintenance.RunBatch(ctx, h.tenant, uuid.Nil, 0); err != nil {
		t.Fatal(err)
	}
	if h.scalar(`SELECT count(*) FROM clinical_session_artifacts WHERE tenant_id=$1 AND id=$2 AND status='deleted'`, h.tenant, id) != 1 {
		t.Fatal("configured retention not enforced")
	}

	// Encrypted stale upload; synthetic fault files only, inside t.TempDir.
	temp := filepath.Join(h.root, strings.ReplaceAll(key, "/", "_")+".upload-synthetic.tmp")
	if err = os.WriteFile(temp, []byte("synthetic encrypted envelope placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-25 * time.Hour)
	if err = os.Chtimes(temp, old, old); err != nil {
		t.Fatal(err)
	}
	if _, _, err = maintenance.RunBatch(ctx, h.tenant, uuid.Nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(temp); !os.IsNotExist(err) {
		t.Fatal("stale temp retained")
	}

	// Reappearing final file with an exact deleted tombstone is safely removed.
	if _, err = h.store.WriteAudio(ctx, key, strings.NewReader("synthetic tombstone residue")); err != nil {
		t.Fatal(err)
	}
	if _, _, err = maintenance.RunBatch(ctx, h.tenant, uuid.Nil, 0); err != nil {
		t.Fatal(err)
	}
	h.assertStorageConsistency()

	// Unknown final files fail visibly; never destroy a potential recovery source.
	orphan := strings.Join([]string{h.tenant.String(), h.client.String(), h.session.String(), uuid.NewString()}, "/")
	if _, err = h.store.WriteAudio(ctx, orphan, strings.NewReader("synthetic orphan")); err != nil {
		t.Fatal(err)
	}
	if _, _, err = maintenance.RunBatch(ctx, h.tenant, uuid.Nil, 0); !errors.Is(err, ingestion.ErrStorageInconsistent) {
		t.Fatalf("orphan silently ignored: %v", err)
	}
	exists, err := h.store.Exists(ctx, orphan)
	if err != nil || !exists {
		t.Fatal("unknown recovery source destroyed")
	}
	// Operator reconciliation of the known synthetic injection, not a production policy.
	if err = h.store.Delete(ctx, orphan); err != nil {
		t.Fatal(err)
	}
	h.assertStorageConsistency()
	t.Log("explicit retention, stale temp cleanup, deleted-file reconciliation, orphan alarm without destructive auto-delete: PASS")
}
