package ingestionacceptance

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"sessionflow/apps/api/internal/usecase/auth"
	"sessionflow/apps/api/internal/usecase/consent"
)

func TestStage2D1HTTPIsolationMatrix(t *testing.T) {
	h := newHarness(t, "")
	h.grants()
	a := h.upload([]byte("synthetic isolation audio"))
	j := h.enqueue(a.ID)
	if _, err := h.worker(&syntheticTranscriber{}, nil).RunOne(context.Background(), h.tenant); err != nil {
		t.Fatal(err)
	}
	v := h.transcripts()[0]
	commands := []struct {
		method, path string
		body         any
	}{
		{"GET", h.sessionPath("/artifacts"), nil},
		{"GET", h.sessionPath("/transcripts"), nil},
		{"GET", h.sessionPath("/jobs"), nil},
		{"POST", h.sessionPath("/audio"), []byte("synthetic forbidden")},
		{"POST", h.sessionPath("/transcriptions"), map[string]any{"artifact_id": a.ID}},
		{"POST", h.sessionPath("/analysis-jobs"), map[string]any{"transcript_version_id": v.ID}},
		{"GET", "/api/v1/clinical-transcripts/" + v.ID.String(), nil},
		{"DELETE", "/api/v1/clinical-transcripts/" + v.ID.String(), nil},
		{"POST", "/api/v1/clinical-transcripts/" + v.ID.String() + "/revisions", map[string]any{"text": v.Text, "segments": v.Segments}},
		{"DELETE", "/api/v1/clinical-artifacts/" + a.ID.String(), nil},
		{"GET", "/api/v1/clinical-jobs/" + j.ID.String(), nil},
		{"POST", "/api/v1/clinical-jobs/" + j.ID.String() + "/retry", nil},
		{"POST", "/api/v1/clinical-jobs/" + j.ID.String() + "/cancel", nil},
		{"GET", "/api/v1/clients/" + h.client.String() + "/consents", nil},
		{"POST", "/api/v1/clients/" + h.client.String() + "/consents", map[string]any{"scope": consent.ExternalManual, "definition_version": 1}},
	}
	originalToken, originalTenant := h.token, h.tenant
	outsider := newHarness(t, "")
	h.token, h.tenant = outsider.token, outsider.tenant
	for _, c := range commands {
		t.Run("foreign-tenant/"+c.method+c.path, func(t *testing.T) {
			status := 404
			if strings.HasSuffix(c.path, "/consents") {
				status = 403
			}
			h.want(c.method, c.path, c.body, status)
		})
	}
	h.token, h.tenant = originalToken, originalTenant
	unassigned := uuid.New()
	h.sql(`INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'synthetic-unused')`, unassigned, h.tenant, unassigned.String()+"@example.invalid")
	var err error
	h.token, _, err = auth.NewTokenService(acceptanceKey, time.Hour).IssueAccessToken(unassigned, h.tenant, "member")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range commands {
		t.Run("unassigned-client/"+c.method+c.path, func(t *testing.T) { h.want(c.method, c.path, c.body, 403) })
	}
	h.sql(`INSERT INTO client_clinical_assignments(id,tenant_id,client_id,user_id,relationship,granted_by_user_id,starts_at) VALUES($1,$2,$3,$4,'supervisor',$5,NOW()-INTERVAL '1 hour')`, uuid.New(), h.tenant, h.client, unassigned, h.actor)
	for _, c := range commands {
		t.Run("supervisor/"+c.method+c.path, func(t *testing.T) {
			status := 403
			if c.method == "GET" {
				status = 200
			}
			h.want(c.method, c.path, c.body, status)
		})
	}
	h.token = ""
	h.want("GET", "/api/v1/clinical-jobs/"+j.ID.String(), nil, 401)
	h.token = originalToken
	if h.job(j.ID).Status != "succeeded" || len(h.transcripts()) != 1 {
		t.Fatal("denied requests mutated state")
	}
	h.assertStorageConsistency()
}
