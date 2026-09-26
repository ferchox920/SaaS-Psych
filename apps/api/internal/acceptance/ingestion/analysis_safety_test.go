package ingestionacceptance

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"sessionflow/apps/api/internal/usecase/clinicalanalysis"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"sessionflow/apps/api/internal/usecase/sessionreport"
)

type reportFunction func(context.Context, string, []byte, map[string]any) (sessionreport.ProviderOutput, error)

func (f reportFunction) GenerateSessionReport(c context.Context, p string, b []byte, s map[string]any) (sessionreport.ProviderOutput, error) {
	return f(c, p, b, s)
}

func TestStage2D1AnalysisFailureBoundaries(t *testing.T) {
	for _, boundary := range []string{"invalid", "cancel", "revoke", "delete-transcript", "provider-retry-budget"} {
		t.Run(boundary, func(t *testing.T) {
			h := newHarness(t, "")
			h.grant(consent.Audio)
			h.grant(consent.Transcription)
			grant := h.grant(consent.LocalAI)
			a := h.upload([]byte("synthetic analysis safety"))
			h.enqueue(a.ID)
			if _, err := h.worker(&syntheticTranscriber{}, nil).RunOne(context.Background(), h.tenant); err != nil {
				t.Fatal(err)
			}
			v := h.transcripts()[0]
			var j ingestion.Job
			decode(t, h.want("POST", h.sessionPath("/analysis-jobs"), map[string]any{"transcript_version_id": v.ID}, 202), &j)
			entered, release := make(chan struct{}, 1), make(chan struct{})
			provider := reportFunction(func(ctx context.Context, _ string, _ []byte, _ map[string]any) (sessionreport.ProviderOutput, error) {
				// Exact run/attempt must be durable before the provider is invoked.
				if h.scalar(`SELECT count(*) FROM clinical_ai_runs r JOIN clinical_ingestion_run_attempts a ON a.tenant_id=r.tenant_id AND a.ai_run_id=r.id WHERE a.tenant_id=$1 AND a.job_id=$2 AND r.status='running'`, h.tenant, j.ID) != 1 {
					t.Error("AIRun before inference missing")
				}
				if boundary == "provider-retry-budget" {
					return sessionreport.ProviderOutput{}, clinicalanalysis.ErrProviderUnavailable
				}
				entered <- struct{}{}
				<-release
				if boundary == "invalid" {
					return sessionreport.ProviderOutput{JSON: []byte(`{"schema_version":"invalid"}`)}, nil
				}
				return sessionreport.ProviderOutput{JSON: []byte(syntheticReport)}, nil
			})
			w := h.worker(&syntheticTranscriber{}, provider)
			if boundary == "provider-retry-budget" {
				for attempt := 1; attempt <= 3; attempt++ {
					if _, err := w.RunOne(context.Background(), h.tenant); err == nil {
						t.Fatal("failed provider accepted")
					}
					current := h.job(j.ID)
					if current.Attempt != attempt || current.ErrorCode == nil || *current.ErrorCode != "provider_unavailable" {
						t.Fatal("safe provider retry metadata missing")
					}
					if attempt < 3 {
						h.want("POST", "/api/v1/clinical-jobs/"+j.ID.String()+"/retry", nil, 202)
					}
				}
				if h.job(j.ID).Status != "failed" {
					t.Fatal("budget not terminal")
				}
				h.want("POST", "/api/v1/clinical-jobs/"+j.ID.String()+"/retry", nil, 409)
			} else {
				result := make(chan error, 1)
				go func() { _, err := w.RunOne(context.Background(), h.tenant); result <- err }()
				select {
				case <-entered:
				case <-time.After(10 * time.Second):
					close(release)
					t.Fatal("provider not entered")
				}
				switch boundary {
				case "cancel":
					h.want("POST", "/api/v1/clinical-jobs/"+j.ID.String()+"/cancel", nil, 200)
				case "revoke":
					h.want("POST", "/api/v1/clients/"+h.client.String()+"/consents/"+grant.String()+"/revoke", nil, 200)
				case "delete-transcript":
					h.want("DELETE", "/api/v1/clinical-transcripts/"+v.ID.String(), nil, 200)
				}
				close(release)
				select {
				case err := <-result:
					if err == nil {
						t.Fatal("unsafe report accepted")
					}
				case <-time.After(10 * time.Second):
					t.Fatal("analysis deadlocked")
				}
			}
			if h.scalar(`SELECT count(*) FROM session_reports WHERE tenant_id=$1`, h.tenant) != 0 || h.scalar(`SELECT count(*) FROM clinical_ai_runs WHERE tenant_id=$1 AND status IN('running','succeeded')`, h.tenant) != 0 {
				t.Fatal("invalid/cancelled analysis leaked report or nonterminal run")
			}
			// Audit/job metadata must never carry narrative, even on invalid output.
			rows, err := h.pool.Query(context.Background(), `SELECT metadata FROM audit_logs WHERE tenant_id=$1`, h.tenant)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var raw []byte
				if err = rows.Scan(&raw); err != nil {
					t.Fatal(err)
				}
				var metadata map[string]any
				if err = json.Unmarshal(raw, &metadata); err != nil {
					t.Fatal(err)
				}
				for _, key := range []string{"text", "transcript", "prompt", "audio", "summary", "session_text"} {
					if _, ok := metadata[key]; ok {
						t.Fatal("clinical content in audit")
					}
				}
			}
			rows.Close()
			if err = rows.Err(); err != nil {
				t.Fatal(err)
			}
			h.assertStorageConsistency()
		})
	}
}
