package db

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
)

type ClinicalConsentRepository struct{ pool *pgxpool.Pool }

func NewClinicalConsentRepository(p *pgxpool.Pool) *ClinicalConsentRepository {
	return &ClinicalConsentRepository{p}
}

const consentColumns = `id,tenant_id,client_id,scope,definition_version,status,granted_by_user_id,granted_at,effective_from,revoked_by_user_id,revoked_at`

func scanConsent(row pgx.Row) (g consent.Grant, err error) {
	err = row.Scan(&g.ID, &g.TenantID, &g.ClientID, &g.Scope, &g.DefinitionVersion, &g.Status, &g.GrantedBy, &g.GrantedAt, &g.EffectiveFrom, &g.RevokedBy, &g.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = domainerrors.ErrNotFound
	}
	return
}
func lockConsentClient(ctx context.Context, tx pgx.Tx, t, c uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('consent:'||$1::text||':'||$2::text,0))`, t, c)
	return err
}
func (r *ClinicalConsentRepository) List(ctx context.Context, t, c uuid.UUID) ([]consent.Grant, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+consentColumns+` FROM clinical_consent_grants WHERE tenant_id=$1 AND client_id=$2 ORDER BY granted_at,id`, t, c)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []consent.Grant{}
	for rows.Next() {
		g, err := scanConsent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
func (r *ClinicalConsentRepository) Grant(ctx context.Context, g consent.Grant) (consent.Grant, error) {
	if !consent.ValidScope(g.Scope) || g.DefinitionVersion != 1 || g.GrantedBy == uuid.Nil {
		return g, domainerrors.NewValidation("invalid consent grant")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return g, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockConsentClient(ctx, tx, g.TenantID, g.ClientID); err != nil {
		return g, err
	}
	out, err := scanConsent(tx.QueryRow(ctx, `INSERT INTO clinical_consent_grants(id,tenant_id,client_id,scope,definition_version,granted_by_user_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+consentColumns, g.ID, g.TenantID, g.ClientID, g.Scope, g.DefinitionVersion, g.GrantedBy))
	if isUniqueViolation(err) {
		return g, domainerrors.ErrConflict
	}
	if err != nil {
		return g, err
	}
	if err := insertAuditEvent(ctx, tx, g.TenantID, g.GrantedBy, "clinical_consent.granted", "clinical_consent", g.ID, map[string]any{"scope": g.Scope, "definition_version": g.DefinitionVersion}); err != nil {
		return g, err
	}
	return out, tx.Commit(ctx)
}
func (r *ClinicalConsentRepository) Revoke(ctx context.Context, t, c, a, id uuid.UUID) (consent.Grant, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return consent.Grant{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockConsentClient(ctx, tx, t, c); err != nil {
		return consent.Grant{}, err
	}
	g, err := scanConsent(tx.QueryRow(ctx, `SELECT `+consentColumns+` FROM clinical_consent_grants WHERE tenant_id=$1 AND client_id=$2 AND id=$3 FOR UPDATE`, t, c, id))
	if err != nil {
		return g, err
	}
	if g.Status == "revoked" {
		return g, tx.Commit(ctx)
	}
	g, err = scanConsent(tx.QueryRow(ctx, `UPDATE clinical_consent_grants SET status='revoked',revoked_by_user_id=$4,revoked_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 RETURNING `+consentColumns, t, c, id, a))
	if err != nil {
		return g, err
	}
	// Fencing running jobs prevents any late result from committing after revocation.
	jobType := ""
	if g.Scope == consent.Transcription {
		jobType = "transcribe_session_audio"
	}
	if g.Scope == consent.LocalAI {
		jobType = "analyze_session"
	}
	if jobType != "" {
		// Lock jobs before runs/attempts, matching cancellation and worker finish.
		_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_jobs SET status='cancelled',error_code='consent_revoked',cancelled_at=NOW(),completed_at=NOW(),updated_at=NOW(),lease_token=NULL,lease_until=NULL WHERE tenant_id=$1 AND client_id=$2 AND job_type=$3 AND status IN('queued','running')`, t, c, jobType)
		if err != nil {
			return g, err
		}
		_, err = tx.Exec(ctx, `UPDATE clinical_ai_runs SET status='cancelled',error_code='consent_revoked',completed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE tenant_id=$1 AND client_id=$2 AND status='running' AND id IN(SELECT a.ai_run_id FROM clinical_ingestion_run_attempts a JOIN clinical_ingestion_jobs j ON j.tenant_id=a.tenant_id AND j.id=a.job_id WHERE a.tenant_id=$1 AND a.client_id=$2 AND j.job_type=$3 AND j.status='cancelled')`, t, c, jobType)
		if err != nil {
			return g, err
		}
		_, err = tx.Exec(ctx, `UPDATE clinical_ingestion_job_attempts SET outcome='cancelled',error_code='consent_revoked',completed_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND outcome='running' AND job_id IN(SELECT id FROM clinical_ingestion_jobs WHERE tenant_id=$1 AND client_id=$2 AND job_type=$3 AND status='cancelled')`, t, c, jobType)
		if err != nil {
			return g, err
		}
	}
	if err := insertAuditEvent(ctx, tx, t, a, "clinical_consent.revoked", "clinical_consent", id, map[string]any{"scope": g.Scope, "definition_version": g.DefinitionVersion}); err != nil {
		return g, err
	}
	return g, tx.Commit(ctx)
}

// authorizeConsentTx is shared by enqueue, capture and worker transactions.
// The caller holds this client lock through its covered boundary commit.
func authorizeConsentTx(ctx context.Context, tx pgx.Tx, t, c uuid.UUID, scope, operation string, entity uuid.UUID) (consent.Grant, error) {
	if !consent.ValidScope(scope) || entity == uuid.Nil {
		return consent.Grant{}, domainerrors.NewValidation("invalid authorization boundary")
	}
	if err := lockConsentClient(ctx, tx, t, c); err != nil {
		return consent.Grant{}, err
	}
	g, err := scanConsent(tx.QueryRow(ctx, `SELECT `+consentColumns+` FROM clinical_consent_grants WHERE tenant_id=$1 AND client_id=$2 AND scope=$3 AND status='granted' AND effective_from<=NOW() FOR SHARE`, t, c, scope))
	if errors.Is(err, domainerrors.ErrNotFound) {
		return g, domainerrors.ErrForbidden
	}
	if err != nil {
		return g, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO clinical_ingestion_authorizations(tenant_id,client_id,grant_id,scope,definition_version,operation,entity_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, t, c, g.ID, g.Scope, g.DefinitionVersion, operation, entity)
	return g, err
}
func (r *ClinicalConsentRepository) Authorize(ctx context.Context, t, c uuid.UUID, scope, operation string, entity uuid.UUID) (consent.Grant, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return consent.Grant{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// tenant_id ownership is enforced in authorizeConsentTx.
	g, err := authorizeConsentTx(ctx, tx, t, c, scope, operation, entity)
	if err != nil {
		return g, err
	}
	return g, tx.Commit(ctx)
}
