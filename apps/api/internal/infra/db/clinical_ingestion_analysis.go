package db

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/clinicalairun"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"sessionflow/apps/api/internal/usecase/sessionreport"
)

func (r *ClinicalIngestionRepository) StartAnalysis(ctx context.Context, j ingestion.Job, model, app, revision string) (uuid.UUID, ingestion.Transcript, error) {
	if strings.TrimSpace(model) == "" || app == "" || revision == "" {
		return uuid.Nil, ingestion.Transcript{}, domainerrors.NewValidation("analysis provenance required")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, ingestion.Transcript{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = authorizeConsentTx(ctx, tx, j.TenantID, j.ClientID, consent.LocalAI, "analysis_start", j.ID); err != nil {
		return uuid.Nil, ingestion.Transcript{}, err
	}
	j, err = lockIngestionAttemptTx(ctx, tx, j)
	if err != nil {
		return uuid.Nil, ingestion.Transcript{}, err
	}
	if j.Type != ingestion.Analyze || j.TranscriptID == nil {
		return uuid.Nil, ingestion.Transcript{}, domainerrors.ErrConflict
	}
	var sessionStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM clinical_sessions WHERE tenant_id=$1 AND id=$2 AND client_id=$3 FOR SHARE`, j.TenantID, j.SessionID, j.ClientID).Scan(&sessionStatus)
	if err != nil {
		return uuid.Nil, ingestion.Transcript{}, err
	}
	if sessionStatus != "completed" {
		return uuid.Nil, ingestion.Transcript{}, domainerrors.ErrConflict
	}
	v, err := scanTranscript(tx.QueryRow(ctx, `SELECT `+transcriptColumns+` FROM clinical_transcript_versions WHERE tenant_id=$1 AND id=$2 AND client_id=$3 AND session_id=$4 AND status='available' FOR SHARE`, j.TenantID, j.TranscriptID, j.ClientID, j.SessionID))
	if err != nil {
		return uuid.Nil, v, err
	}
	if n := len([]rune(strings.TrimSpace(v.Text))); n < 20 || n > 40000 {
		return uuid.Nil, v, domainerrors.NewValidation("transcript outside report input limits")
	}
	payload, _ := json.Marshal(map[string]any{"schema_version": sessionreport.SchemaVersion, "session_text": v.Text})
	run := uuid.New()
	contextHash := clinicalairun.Hash(struct {
		TranscriptID uuid.UUID
		Version      int
		Hash         string
	}{v.ID, v.Version, v.ContentHash})
	parameters, _ := json.Marshal(map[string]any{"configuration_hash": j.ConfigurationHash, "job_id": j.ID, "attempt": j.Attempt})
	_, err = tx.Exec(ctx, `INSERT INTO clinical_ai_runs(id,tenant_id,client_id,clinical_session_id,created_by_user_id,provider,model,operation,prompt_name,prompt_version,app_version,build_revision,parameters_json,input_hash,context_hash,status,started_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'ollama',$6,'generate_session_report','session-report',$7,$8,$9,$10,$11,$12,'running',clock_timestamp(),clock_timestamp(),clock_timestamp())`, run, j.TenantID, j.ClientID, j.SessionID, j.ActorID, model, sessionreport.SchemaVersion, app, revision, parameters, ingestion.Hash(string(payload)), contextHash)
	if err != nil {
		return uuid.Nil, v, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO clinical_ingestion_run_attempts(tenant_id,client_id,session_id,job_id,attempt,ai_run_id,transcript_version_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, j.TenantID, j.ClientID, j.SessionID, j.ID, j.Attempt, run, v.ID)
	if err != nil {
		return uuid.Nil, v, err
	}
	if err := insertAuditEvent(ctx, tx, j.TenantID, j.ActorID, "clinical_ai_run.started", "clinical_ai_run", run, map[string]any{"operation": "generate_session_report", "transcript_version_id": v.ID, "transcript_version": v.Version, "input_hash": ingestion.Hash(string(payload)), "context_hash": contextHash}); err != nil {
		return uuid.Nil, v, err
	}
	return run, v, tx.Commit(ctx)
}
func (r *ClinicalIngestionRepository) CompleteAnalysis(ctx context.Context, j ingestion.Job, run uuid.UUID, document sessionreport.ReportV1) (sessionreport.Report, error) {
	if err := sessionreport.Validate(document); err != nil {
		return sessionreport.Report{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return sessionreport.Report{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = authorizeConsentTx(ctx, tx, j.TenantID, j.ClientID, consent.LocalAI, "analysis_complete", j.ID); err != nil {
		return sessionreport.Report{}, err
	}
	j, err = lockIngestionAttemptTx(ctx, tx, j)
	if err != nil {
		return sessionreport.Report{}, err
	}
	if j.Type != ingestion.Analyze || j.TranscriptID == nil {
		return sessionreport.Report{}, domainerrors.ErrConflict
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clinical_ingestion_run_attempts a JOIN clinical_transcript_versions v ON v.tenant_id=a.tenant_id AND v.id=a.transcript_version_id JOIN clinical_sessions s ON s.tenant_id=a.tenant_id AND s.id=a.session_id WHERE a.tenant_id=$1 AND a.job_id=$2 AND a.attempt=$3 AND a.ai_run_id=$4 AND a.transcript_version_id=$5 AND v.status='available' AND s.status='completed')`, j.TenantID, j.ID, j.Attempt, run, j.TranscriptID).Scan(&valid)
	if err != nil {
		return sessionreport.Report{}, err
	}
	if !valid {
		return sessionreport.Report{}, domainerrors.ErrConflict
	}
	if err := lockSessionReports(ctx, tx, j.TenantID, j.SessionID); err != nil {
		return sessionreport.Report{}, err
	}
	encoded, _ := json.Marshal(document)
	out, err := scanSessionReport(tx.QueryRow(ctx, `INSERT INTO session_reports(tenant_id,clinical_session_id,version,schema_version,status,report_json,created_by_user_id,source_ai_run_id) SELECT $1,$2,COALESCE(MAX(version),0)+1,$3,'draft',$4,$5,$6 FROM session_reports WHERE tenant_id=$1 AND clinical_session_id=$2 RETURNING id,tenant_id,clinical_session_id,version,revision,schema_version,status,report_json,created_by_user_id,source_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at`, j.TenantID, j.SessionID, sessionreport.SchemaVersion, encoded, j.ActorID, run))
	if err != nil {
		return out, err
	}
	tag, err := tx.Exec(ctx, `UPDATE clinical_ai_runs SET status='succeeded',output_hash=$3,completed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND status='running'`, j.TenantID, run, clinicalairun.Hash(document))
	if err != nil {
		return out, err
	}
	if tag.RowsAffected() != 1 {
		return out, domainerrors.ErrConflict
	}
	if err := completeIngestionAttemptTx(ctx, tx, j, j.TranscriptID, &run, &out.ID); err != nil {
		return out, err
	}
	if err := insertAuditEvent(ctx, tx, j.TenantID, j.ActorID, "session_report.generated", "session_report", out.ID, map[string]any{"version": out.Version, "source_ai_run_id": run, "transcript_version_id": j.TranscriptID}); err != nil {
		return out, err
	}
	if err := insertAuditEvent(ctx, tx, j.TenantID, j.ActorID, "clinical_ai_run.succeeded", "clinical_ai_run", run, map[string]any{"output_hash": clinicalairun.Hash(document)}); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func terminateIngestionRunsTx(ctx context.Context, tx pgx.Tx, t, job uuid.UUID, code string) error {
	rows, err := tx.Query(ctx, `UPDATE clinical_ai_runs SET status='failed',error_code=$3,completed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE tenant_id=$1 AND status='running' AND id IN(SELECT ai_run_id FROM clinical_ingestion_run_attempts WHERE tenant_id=$1 AND job_id=$2) RETURNING id,created_by_user_id`, t, job, code)
	if err != nil {
		return err
	}
	type pair struct{ id, actor uuid.UUID }
	items := []pair{}
	for rows.Next() {
		var p pair
		if err = rows.Scan(&p.id, &p.actor); err != nil {
			rows.Close()
			return err
		}
		items = append(items, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range items {
		if err := insertAuditEvent(ctx, tx, t, p.actor, "clinical_ai_run.failed", "clinical_ai_run", p.id, map[string]any{"error_code": code}); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClinicalIngestionRepository) AuthorizeAttempt(ctx context.Context, j ingestion.Job) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	scope := consent.Transcription
	if j.Type == ingestion.Analyze {
		scope = consent.LocalAI
	} else if j.Type != ingestion.Transcribe {
		return domainerrors.ErrConflict
	}
	if _, err = authorizeConsentTx(ctx, tx, j.TenantID, j.ClientID, scope, "job_execute", j.ID); err != nil {
		return err
	}
	current, err := lockIngestionAttemptTx(ctx, tx, j)
	if err != nil {
		return err
	}
	var ok bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM client_clinical_assignments WHERE tenant_id=$1 AND client_id=$2 AND user_id=$3 AND relationship='treating' AND starts_at<=clock_timestamp() AND (ends_at IS NULL OR ends_at>clock_timestamp()))`, current.TenantID, current.ClientID, current.ActorID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainerrors.ErrForbidden
	}
	if err != nil {
		return err
	}
	if !ok {
		return domainerrors.ErrForbidden
	}
	return tx.Commit(ctx)
}
