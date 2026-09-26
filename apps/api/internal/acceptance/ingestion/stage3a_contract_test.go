package ingestionacceptance

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"sessionflow/apps/api/internal/usecase/auth"
	"sessionflow/apps/api/internal/usecase/ingestion"
)

func TestStage3AWorkspaceReadContracts(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	a := h.upload([]byte("synthetic Stage3A artifact"))
	if a.CreatedAt.IsZero() {
		t.Fatal("missing artifact creation timestamp")
	}
	raw := h.want("GET", h.sessionPath("/artifacts"), nil, 200)
	for _, secret := range []string{"storage_key", "key_id", "storage_path"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("internal storage metadata exposed")
		}
	}
	j := h.enqueue(a.ID)
	var jobs struct {
		Items []ingestion.Job `json:"items"`
	}
	decode(t, h.want("GET", h.sessionPath("/jobs"), nil, 200), &jobs)
	if len(jobs.Items) != 1 || jobs.Items[0].ID != j.ID || jobs.Items[0].Status != "queued" {
		t.Fatal("session jobs are not reloadable")
	}
	h.want("POST", "/api/v1/clinical-jobs/"+j.ID.String()+"/cancel", nil, 200)
	decode(t, h.want("GET", h.sessionPath("/jobs"), nil, 200), &jobs)
	if jobs.Items[0].Status != "cancelled" {
		t.Fatal("list did not reflect durable cancellation")
	}
	path := "/api/v1/clients/" + h.client.String() + "/clinical-sessions"
	var sessions struct {
		CanWrite bool `json:"can_write"`
	}
	decode(t, h.want("GET", path, nil, 200), &sessions)
	if !sessions.CanWrite {
		t.Fatal("treating actor lost write capability")
	}
	supervisor := uuid.New()
	h.sql(`INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'synthetic-unused')`, supervisor, h.tenant, supervisor.String()+"@example.invalid")
	h.sql(`INSERT INTO client_clinical_assignments(id,tenant_id,client_id,user_id,relationship,granted_by_user_id,starts_at) VALUES($1,$2,$3,$4,'supervisor',$5,NOW()-INTERVAL '1 hour')`, uuid.New(), h.tenant, h.client, supervisor, h.actor)
	var err error
	h.token, _, err = auth.NewTokenService(acceptanceKey, time.Hour).IssueAccessToken(supervisor, h.tenant, "member")
	if err != nil {
		t.Fatal(err)
	}
	decode(t, h.want("GET", path, nil, 200), &sessions)
	if sessions.CanWrite {
		t.Fatal("supervisor advertised write capability")
	}
	h.want("GET", h.sessionPath("/jobs"), nil, 200)
	h.want("POST", h.sessionPath("/audio"), []byte("synthetic forbidden"), 403)
	h.assertStorageConsistency()
}
