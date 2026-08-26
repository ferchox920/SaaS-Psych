package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	sessionreport "sessionflow/apps/api/internal/usecase/sessionreport"
)

type SessionReportRepository struct{ pool *pgxpool.Pool }

func NewSessionReportRepository(pool *pgxpool.Pool) *SessionReportRepository {
	return &SessionReportRepository{pool: pool}
}
func (r *SessionReportRepository) SessionDetails(ctx context.Context, tenantID, sessionID uuid.UUID) (sessionreport.SessionDetails, error) {
	var item sessionreport.SessionDetails
	item.ID = sessionID
	item.TenantID = tenantID
	err := r.pool.QueryRow(ctx, `SELECT client_id,appointment_id,therapist_user_id,status FROM clinical_sessions WHERE tenant_id=$1 AND id=$2`, tenantID, sessionID).Scan(&item.ClientID, &item.AppointmentID, &item.TherapistUserID, &item.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return sessionreport.SessionDetails{}, domainerrors.ErrNotFound
	}
	return item, err
}
func (r *SessionReportRepository) CreateDraft(ctx context.Context, tenantID, sessionID, actorID uuid.UUID, runID *uuid.UUID, document sessionreport.ReportV1) (sessionreport.Report, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return sessionreport.Report{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSessionReports(ctx, tx, tenantID, sessionID); err != nil {
		return sessionreport.Report{}, err
	}
	encoded, _ := json.Marshal(document)
	const query = `INSERT INTO session_reports(tenant_id,clinical_session_id,version,schema_version,status,report_json,created_by_user_id,source_ai_run_id) SELECT $1,$2,COALESCE(MAX(version),0)+1,$3,'draft',$4,$5,$6 FROM session_reports WHERE tenant_id=$1 AND clinical_session_id=$2 RETURNING id,tenant_id,clinical_session_id,version,revision,schema_version,status,report_json,created_by_user_id,source_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at`
	out, err := scanSessionReport(tx.QueryRow(ctx, query, tenantID, sessionID, sessionreport.SchemaVersion, encoded, actorID, runID))
	if err != nil {
		return sessionreport.Report{}, err
	}
	if err := insertAuditEvent(ctx, tx, tenantID, actorID, "session_report.generated", "session_report", out.ID, map[string]any{"clinical_session_id": sessionID, "version": out.Version, "schema_version": out.SchemaVersion, "source_ai_run_id": runID}); err != nil {
		return sessionreport.Report{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sessionreport.Report{}, err
	}
	return out, nil
}
func (r *SessionReportRepository) List(ctx context.Context, tenantID, sessionID uuid.UUID) ([]sessionreport.Report, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,tenant_id,clinical_session_id,version,revision,schema_version,status,report_json,created_by_user_id,source_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at FROM session_reports WHERE tenant_id=$1 AND clinical_session_id=$2 ORDER BY version DESC`, tenantID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []sessionreport.Report{}
	for rows.Next() {
		item, scanErr := scanSessionReport(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *SessionReportRepository) Get(ctx context.Context, tenantID, reportID uuid.UUID) (sessionreport.Report, error) {
	item, err := scanSessionReport(r.pool.QueryRow(ctx, `SELECT id,tenant_id,clinical_session_id,version,revision,schema_version,status,report_json,created_by_user_id,source_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at FROM session_reports WHERE tenant_id=$1 AND id=$2`, tenantID, reportID))
	if errors.Is(err, pgx.ErrNoRows) {
		return sessionreport.Report{}, domainerrors.ErrNotFound
	}
	return item, err
}
func (r *SessionReportRepository) Update(ctx context.Context, tenantID, reportID, actorID uuid.UUID, expectedRevision int, document sessionreport.ReportV1) (sessionreport.Report, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return sessionreport.Report{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := scanSessionReport(tx.QueryRow(ctx, `SELECT id,tenant_id,clinical_session_id,version,revision,schema_version,status,report_json,created_by_user_id,source_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at FROM session_reports WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, reportID))
	if errors.Is(err, pgx.ErrNoRows) {
		return sessionreport.Report{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return sessionreport.Report{}, err
	}
	if current.Revision != expectedRevision {
		return sessionreport.Report{}, domainerrors.ErrConflict
	}
	encoded, _ := json.Marshal(document)
	var out sessionreport.Report
	if current.Status == "draft" {
		out, err = scanSessionReport(tx.QueryRow(ctx, `UPDATE session_reports SET report_json=$3,schema_version=$5,revision=revision+1,updated_at=NOW() WHERE tenant_id=$1 AND id=$2 AND revision=$4 AND status='draft' RETURNING id,tenant_id,clinical_session_id,version,revision,schema_version,status,report_json,created_by_user_id,source_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at`, tenantID, reportID, encoded, expectedRevision, document.SchemaVersion))
	} else {
		if err = lockSessionReports(ctx, tx, tenantID, current.ClinicalSessionID); err == nil {
			out, err = scanSessionReport(tx.QueryRow(ctx, `INSERT INTO session_reports(tenant_id,clinical_session_id,version,schema_version,status,report_json,created_by_user_id) SELECT $1,$2,COALESCE(MAX(version),0)+1,$3,'draft',$4,$5 FROM session_reports WHERE tenant_id=$1 AND clinical_session_id=$2 RETURNING id,tenant_id,clinical_session_id,version,revision,schema_version,status,report_json,created_by_user_id,source_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at`, tenantID, current.ClinicalSessionID, sessionreport.SchemaVersion, encoded, actorID))
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sessionreport.Report{}, domainerrors.ErrConflict
	}
	if err != nil {
		return sessionreport.Report{}, err
	}
	if err := insertAuditEvent(ctx, tx, tenantID, actorID, "session_report.updated", "session_report", out.ID, map[string]any{"clinical_session_id": out.ClinicalSessionID, "version": out.Version, "revision": out.Revision, "new_version": out.ID != current.ID}); err != nil {
		return sessionreport.Report{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sessionreport.Report{}, err
	}
	return out, nil
}
func (r *SessionReportRepository) Approve(ctx context.Context, tenantID, reportID, actorID uuid.UUID) (sessionreport.Report, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return sessionreport.Report{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := scanSessionReport(tx.QueryRow(ctx, `SELECT id,tenant_id,clinical_session_id,version,revision,schema_version,status,report_json,created_by_user_id,source_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at FROM session_reports WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, reportID))
	if errors.Is(err, pgx.ErrNoRows) {
		return sessionreport.Report{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return sessionreport.Report{}, err
	}
	if current.Status != "draft" {
		return sessionreport.Report{}, domainerrors.ErrConflict
	}
	if err := lockSessionReports(ctx, tx, tenantID, current.ClinicalSessionID); err != nil {
		return sessionreport.Report{}, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE session_reports SET status='superseded',updated_at=$3 WHERE tenant_id=$1 AND clinical_session_id=$2 AND status='approved'`, tenantID, current.ClinicalSessionID, now); err != nil {
		return sessionreport.Report{}, err
	}
	out, err := scanSessionReport(tx.QueryRow(ctx, `UPDATE session_reports SET status='approved',approved_by_user_id=$3,approved_at=$4,updated_at=$4 WHERE tenant_id=$1 AND id=$2 AND status='draft' RETURNING id,tenant_id,clinical_session_id,version,revision,schema_version,status,report_json,created_by_user_id,source_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at`, tenantID, reportID, actorID, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return sessionreport.Report{}, domainerrors.ErrConflict
	}
	if err != nil {
		return sessionreport.Report{}, err
	}
	if err := insertAuditEvent(ctx, tx, tenantID, actorID, "session_report.approved", "session_report", out.ID, map[string]any{"clinical_session_id": out.ClinicalSessionID, "version": out.Version, "schema_version": out.SchemaVersion}); err != nil {
		return sessionreport.Report{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sessionreport.Report{}, err
	}
	return out, nil
}
func (r *SessionReportRepository) ListApprovedByClient(ctx context.Context, tenantID, clientID uuid.UUID, limit int) ([]sessionreport.Report, error) {
	if limit < 1 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx, `SELECT sr.id,sr.tenant_id,sr.clinical_session_id,sr.version,sr.revision,sr.schema_version,sr.status,sr.report_json,sr.created_by_user_id,sr.source_ai_run_id,sr.approved_by_user_id,sr.approved_at,sr.created_at,sr.updated_at FROM session_reports sr JOIN clinical_sessions cs ON cs.tenant_id=sr.tenant_id AND cs.id=sr.clinical_session_id WHERE sr.tenant_id=$1 AND cs.client_id=$2 AND sr.status='approved' ORDER BY cs.started_at DESC,cs.id DESC,sr.version DESC,sr.id DESC LIMIT $3`, tenantID, clientID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []sessionreport.Report{}
	for rows.Next() {
		item, scanErr := scanSessionReport(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func lockSessionReports(ctx context.Context, tx pgx.Tx, tenantID, sessionID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text || ':' || $2::text || ':reports',0))`, tenantID, sessionID)
	return err
}
func scanSessionReport(row pgx.Row) (sessionreport.Report, error) {
	var item sessionreport.Report
	var raw []byte
	err := row.Scan(&item.ID, &item.TenantID, &item.ClinicalSessionID, &item.Version, &item.Revision, &item.SchemaVersion, &item.Status, &raw, &item.CreatedByUserID, &item.SourceAIRunID, &item.ApprovedByUserID, &item.ApprovedAt, &item.CreatedAt, &item.UpdatedAt)
	item.ReportJSON = raw
	return item, err
}
