package db

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
)

type ClinicalAIRunRepository struct{ pool *pgxpool.Pool }

func NewClinicalAIRunRepository(pool *pgxpool.Pool) *ClinicalAIRunRepository {
	return &ClinicalAIRunRepository{pool: pool}
}
func (r *ClinicalAIRunRepository) Start(ctx context.Context, run clinicalairun.Run) (clinicalairun.Run, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return clinicalairun.Run{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `INSERT INTO clinical_ai_runs(id,tenant_id,client_id,appointment_id,clinical_session_id,created_by_user_id,provider,model,operation,prompt_name,prompt_version,app_version,build_revision,parameters_json,input_hash,context_hash,status,started_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20) RETURNING id,tenant_id,client_id,appointment_id,clinical_session_id,created_by_user_id,provider,model,operation,prompt_name,prompt_version,app_version,build_revision,parameters_json,input_hash,context_hash,output_hash,status,started_at,completed_at,error_code,created_at,updated_at`
	out, err := scanClinicalAIRun(tx.QueryRow(ctx, query, run.ID, run.TenantID, run.ClientID, run.AppointmentID, run.ClinicalSessionID, run.CreatedByUserID, run.Provider, run.Model, run.Operation, run.PromptName, run.PromptVersion, run.AppVersion, run.BuildRevision, run.ParametersJSON, run.InputHash, run.ContextHash, run.Status, run.StartedAt, run.CreatedAt, run.UpdatedAt))
	if err != nil {
		return clinicalairun.Run{}, err
	}
	for _, source := range run.Sources {
		if _, err := tx.Exec(ctx, `INSERT INTO clinical_ai_run_sources(tenant_id,ai_run_id,source_type,source_id,source_version) VALUES($1,$2,$3,$4,$5)`, run.TenantID, run.ID, source.SourceType, source.SourceID, source.SourceVersion); err != nil {
			return clinicalairun.Run{}, err
		}
	}
	out.Sources = append([]clinicalairun.Source(nil), run.Sources...)
	if err := insertAuditEvent(ctx, tx, run.TenantID, run.CreatedByUserID, "clinical_ai_run.started", "clinical_ai_run", run.ID, map[string]any{"provider": run.Provider, "model": run.Model, "operation": run.Operation, "prompt_name": run.PromptName, "prompt_version": run.PromptVersion, "app_version": run.AppVersion, "build_revision": run.BuildRevision, "input_hash": run.InputHash, "context_hash": run.ContextHash, "source_count": len(run.Sources)}); err != nil {
		return clinicalairun.Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return clinicalairun.Run{}, err
	}
	return out, nil
}
func (r *ClinicalAIRunRepository) Finish(ctx context.Context, tenantID, runID uuid.UUID, status string, outputHash, errorCode *string, completedAt time.Time) (clinicalairun.Run, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return clinicalairun.Run{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `UPDATE clinical_ai_runs SET status=$3,output_hash=$4,error_code=$5,completed_at=$6,updated_at=$6 WHERE tenant_id=$1 AND id=$2 AND status='running' RETURNING id,tenant_id,client_id,appointment_id,clinical_session_id,created_by_user_id,provider,model,operation,prompt_name,prompt_version,app_version,build_revision,parameters_json,input_hash,context_hash,output_hash,status,started_at,completed_at,error_code,created_at,updated_at`
	out, err := scanClinicalAIRun(tx.QueryRow(ctx, query, tenantID, runID, status, outputHash, errorCode, completedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return clinicalairun.Run{}, domainerrors.ErrConflict
	}
	if err != nil {
		return clinicalairun.Run{}, err
	}
	metadata := map[string]any{"status": status}
	if outputHash != nil {
		metadata["output_hash"] = *outputHash
	}
	if errorCode != nil {
		metadata["error_code"] = *errorCode
	}
	if err := insertAuditEvent(ctx, tx, tenantID, out.CreatedByUserID, "clinical_ai_run."+status, "clinical_ai_run", runID, metadata); err != nil {
		return clinicalairun.Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return clinicalairun.Run{}, err
	}
	return out, nil
}
func (r *ClinicalAIRunRepository) ListSources(ctx context.Context, tenantID, runID uuid.UUID) ([]clinicalairun.Source, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,tenant_id,ai_run_id,source_type,source_id,source_version,created_at FROM clinical_ai_run_sources WHERE tenant_id=$1 AND ai_run_id=$2 ORDER BY source_type,source_id,source_version NULLS FIRST,id`, tenantID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]clinicalairun.Source, 0)
	for rows.Next() {
		var item clinicalairun.Source
		if err := rows.Scan(&item.ID, &item.TenantID, &item.AIRunID, &item.SourceType, &item.SourceID, &item.SourceVersion, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func scanClinicalAIRun(row pgx.Row) (clinicalairun.Run, error) {
	var run clinicalairun.Run
	err := row.Scan(&run.ID, &run.TenantID, &run.ClientID, &run.AppointmentID, &run.ClinicalSessionID, &run.CreatedByUserID, &run.Provider, &run.Model, &run.Operation, &run.PromptName, &run.PromptVersion, &run.AppVersion, &run.BuildRevision, &run.ParametersJSON, &run.InputHash, &run.ContextHash, &run.OutputHash, &run.Status, &run.StartedAt, &run.CompletedAt, &run.ErrorCode, &run.CreatedAt, &run.UpdatedAt)
	return run, err
}
