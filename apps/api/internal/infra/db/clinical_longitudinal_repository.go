package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

type ClinicalLongitudinalRepository struct{ pool *pgxpool.Pool }

func NewClinicalLongitudinalRepository(pool *pgxpool.Pool) *ClinicalLongitudinalRepository {
	return &ClinicalLongitudinalRepository{pool: pool}
}

func (r *ClinicalLongitudinalRepository) SessionAnalysis(ctx context.Context, t, s uuid.UUID) (longitudinal.SessionAnalysis, error) {
	var out longitudinal.SessionAnalysis
	out.SessionID = s
	err := r.pool.QueryRow(ctx, `SELECT cs.client_id,cs.appointment_id,cs.status,sr.id,sr.version,sr.report_json FROM clinical_sessions cs JOIN session_reports sr ON sr.tenant_id=cs.tenant_id AND sr.clinical_session_id=cs.id AND sr.status='approved' WHERE cs.tenant_id=$1 AND cs.id=$2`, t, s).Scan(&out.ClientID, &out.AppointmentID, &out.Status, &out.ReportID, &out.ReportVersion, &out.ReportJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, domainerrors.ErrNotFound
	}
	return out, err
}
func (r *ClinicalLongitudinalRepository) FindOpenDiff(ctx context.Context, t, report uuid.UUID, base int64) (longitudinal.Diff, error) {
	d, err := scanDiff(r.pool.QueryRow(ctx, diffSelect+` WHERE d.tenant_id=$1 AND d.source_session_report_id=$2 AND d.base_state_version=$3 AND d.status IN ('draft','pending_review','partially_reviewed','approved')`, t, report, base))
	if errors.Is(err, pgx.ErrNoRows) {
		return d, domainerrors.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.Operations, err = r.listOperations(ctx, t, d.ID)
	return d, err
}
func (r *ClinicalLongitudinalRepository) CreateDiff(ctx context.Context, in longitudinal.CreateDiffInput) (longitudinal.Diff, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO clinical_longitudinal_heads(tenant_id,client_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, in.TenantID, in.ClientID)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	uncertainties, _ := json.Marshal(in.Uncertainties)
	id := uuid.New()
	if len(in.OutputHash) != 64 {
		return longitudinal.Diff{}, domainerrors.NewValidation("longitudinal output hash is required")
	}
	tag, err := tx.Exec(ctx, `UPDATE clinical_ai_runs SET status='succeeded',output_hash=$3,completed_at=NOW(),updated_at=NOW() WHERE tenant_id=$1 AND id=$2 AND client_id=$4 AND status='running'`, in.TenantID, in.RunID, in.OutputHash, in.ClientID)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	if tag.RowsAffected() != 1 {
		return longitudinal.Diff{}, domainerrors.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO clinical_diffs(id,tenant_id,client_id,clinical_session_id,source_session_report_id,source_ai_run_id,status,base_state_version,uncertainties_json,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6,'pending_review',$7,$8,$9)`, id, in.TenantID, in.ClientID, in.SessionID, in.ReportID, in.RunID, in.BaseStateVersion, uncertainties, in.ActorID)
	if isUniqueViolation(err) {
		return longitudinal.Diff{}, domainerrors.ErrConflict
	}
	if err != nil {
		return longitudinal.Diff{}, err
	}
	for i, op := range in.Operations {
		op.ID = uuid.New()
		op.DiffID = id
		op.Sequence = i + 1
		op.TenantID = in.TenantID
		op.ClientID = in.ClientID
		if _, err = tx.Exec(ctx, `INSERT INTO clinical_diff_operations(id,tenant_id,client_id,diff_id,sequence,operation_type,target_entity_id,expected_entity_version,original_proposal) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, op.ID, in.TenantID, in.ClientID, id, op.Sequence, op.OperationType, op.TargetEntityID, op.ExpectedEntityVersion, op.OriginalProposal); err != nil {
			return longitudinal.Diff{}, err
		}
	}
	if err = insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, "clinical_diff.created", "clinical_diff", id, map[string]any{"operation_count": len(in.Operations), "base_state_version": in.BaseStateVersion, "source_ai_run_id": in.RunID}); err != nil {
		return longitudinal.Diff{}, err
	}
	for _, op := range in.Operations {
		action := ""
		entity := ""
		switch op.OperationType {
		case "create_event":
			action, entity = "clinical_event.proposed", "clinical_event"
		case "create_process":
			action, entity = "clinical_process.proposed", "clinical_process"
		case "create_hypothesis":
			action, entity = "clinical_hypothesis.proposed", "clinical_hypothesis"
		}
		if action != "" {
			if err = insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, action, entity, op.TargetEntityID, map[string]any{"diff_id": id, "operation_type": op.OperationType}); err != nil {
				return longitudinal.Diff{}, err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return longitudinal.Diff{}, err
	}
	return r.GetDiff(ctx, in.TenantID, id)
}

func (r *ClinicalLongitudinalRepository) GetDiff(ctx context.Context, t, id uuid.UUID) (longitudinal.Diff, error) {
	d, err := scanDiff(r.pool.QueryRow(ctx, diffSelect+` WHERE d.tenant_id=$1 AND d.id=$2`, t, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return d, domainerrors.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.Operations, err = r.listOperations(ctx, t, id)
	return d, err
}
func (r *ClinicalLongitudinalRepository) ListDiffs(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Diff, error) {
	rows, err := r.pool.Query(ctx, diffSelect+` WHERE d.tenant_id=$1 AND d.client_id=$2 ORDER BY d.created_at DESC,d.id DESC`, t, c)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.Diff{}
	for rows.Next() {
		d, e := scanDiff(rows)
		if e != nil {
			return nil, e
		}
		d.Operations, e = r.listOperations(ctx, t, d.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *ClinicalLongitudinalRepository) Decide(ctx context.Context, in longitudinal.DecisionInput) (longitudinal.Diff, error) {
	if in.ExpectedDiffRevision < 1 || !(in.Decision == "approved" || in.Decision == "modified" || in.Decision == "rejected") {
		return longitudinal.Diff{}, domainerrors.NewValidation("invalid decision or expected_diff_revision")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var client uuid.UUID
	var revision int
	var status string
	err = tx.QueryRow(ctx, `SELECT client_id,revision,status FROM clinical_diffs WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, in.TenantID, in.DiffID).Scan(&client, &revision, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return longitudinal.Diff{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return longitudinal.Diff{}, err
	}
	if revision != in.ExpectedDiffRevision || status == "merged" || status == "rejected" {
		return longitudinal.Diff{}, domainerrors.ErrConflict
	}
	var current, operationType string
	var targetEntityID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT review_status,operation_type,target_entity_id FROM clinical_diff_operations WHERE tenant_id=$1 AND diff_id=$2 AND id=$3 FOR UPDATE`, in.TenantID, in.DiffID, in.OperationID).Scan(&current, &operationType, &targetEntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return longitudinal.Diff{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return longitudinal.Diff{}, err
	}
	if current != "pending" {
		return longitudinal.Diff{}, domainerrors.ErrConflict
	}
	var modification any = nil
	if in.Decision == "modified" {
		modification = in.Modification
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_diff_operations SET review_status=$4,human_modification=$5,reviewed_by_user_id=$6,reviewed_at=NOW() WHERE tenant_id=$1 AND diff_id=$2 AND id=$3`, in.TenantID, in.DiffID, in.OperationID, in.Decision, modification, in.ActorID)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	var pending, accepted int
	err = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE review_status='pending'),count(*) FILTER(WHERE review_status IN('approved','modified')) FROM clinical_diff_operations WHERE tenant_id=$1 AND diff_id=$2`, in.TenantID, in.DiffID).Scan(&pending, &accepted)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	newStatus := "partially_reviewed"
	var reviewedAt any = nil
	if pending == 0 {
		reviewedAt = time.Now().UTC()
		if accepted > 0 {
			newStatus = "approved"
		} else {
			newStatus = "rejected"
		}
	}
	_, err = tx.Exec(ctx, `UPDATE clinical_diffs SET status=$3,revision=revision+1,reviewed_by_user_id=CASE WHEN $4::timestamptz IS NULL THEN reviewed_by_user_id ELSE $5 END,reviewed_at=COALESCE($4,reviewed_at),updated_at=NOW() WHERE tenant_id=$1 AND id=$2`, in.TenantID, in.DiffID, newStatus, reviewedAt, in.ActorID)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	if err = insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, "clinical_diff.reviewed", "clinical_diff", in.DiffID, map[string]any{"operation_id": in.OperationID, "decision": in.Decision, "status": newStatus}); err != nil {
		return longitudinal.Diff{}, err
	}
	if in.Decision == "rejected" {
		action, entity := "", ""
		switch operationType {
		case "create_event":
			action, entity = "clinical_event.rejected", "clinical_event"
		case "create_process":
			action, entity = "clinical_process.rejected", "clinical_process"
		case "create_hypothesis":
			action, entity = "clinical_hypothesis.rejected", "clinical_hypothesis"
		}
		if action != "" {
			if err = insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, action, entity, targetEntityID, map[string]any{"diff_id": in.DiffID, "operation_id": in.OperationID}); err != nil {
				return longitudinal.Diff{}, err
			}
		}
	}
	if newStatus == "rejected" {
		if err = insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, "clinical_diff.rejected", "clinical_diff", in.DiffID, map[string]any{"revision": revision + 1}); err != nil {
			return longitudinal.Diff{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return longitudinal.Diff{}, err
	}
	return r.GetDiff(ctx, in.TenantID, in.DiffID)
}

func (r *ClinicalLongitudinalRepository) ListEvidence(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Evidence, error) {
	rows, err := r.pool.Query(ctx, evidenceSelect+` WHERE e.tenant_id=$1 AND e.client_id=$2 ORDER BY e.created_at DESC,e.id DESC`, t, c)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.Evidence{}
	for rows.Next() {
		x, e := scanEvidence(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) ListEvents(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Event, error) {
	rows, err := r.pool.Query(ctx, eventSelect+` WHERE e.tenant_id=$1 AND e.client_id=$2 ORDER BY e.observed_at DESC,e.id DESC`, t, c)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.Event{}
	for rows.Next() {
		x, e := scanEvent(rows)
		if e != nil {
			return nil, e
		}
		x.Evidence, e = r.eventEvidence(ctx, t, x.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) ListProcesses(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Process, error) {
	rows, err := r.pool.Query(ctx, processSelect+` WHERE p.tenant_id=$1 AND p.client_id=$2 ORDER BY CASE p.clinical_status WHEN 'active' THEN 1 WHEN 'observing' THEN 2 WHEN 'stabilized' THEN 3 ELSE 4 END,p.updated_at DESC,p.id DESC`, t, c)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.Process{}
	for rows.Next() {
		x, e := scanProcess(rows)
		if e != nil {
			return nil, e
		}
		x.Events, e = r.processEvents(ctx, t, x.ID)
		if e != nil {
			return nil, e
		}
		x.Hypotheses, e = r.processHypotheses(ctx, t, x.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) ListHypotheses(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Hypothesis, error) {
	rows, err := r.pool.Query(ctx, hypothesisSelect+` WHERE h.tenant_id=$1 AND h.client_id=$2 ORDER BY h.updated_at DESC,h.id DESC`, t, c)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.Hypothesis{}
	for rows.Next() {
		x, e := scanHypothesis(rows)
		if e != nil {
			return nil, e
		}
		if e = r.loadHypothesisEvidence(ctx, t, &x); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) State(ctx context.Context, t, c uuid.UUID) (longitudinal.State, error) {
	out := longitudinal.State{ClientID: c, Processes: []longitudinal.Process{}, UnassignedHypotheses: []longitudinal.Hypothesis{}, RecentEvents: []longitudinal.Event{}, ActiveEvidence: []longitudinal.Evidence{}, OpenProposals: []longitudinal.Diff{}}
	if err := r.pool.QueryRow(ctx, `SELECT COALESCE((SELECT revision FROM clinical_longitudinal_heads WHERE tenant_id=$1 AND client_id=$2),0)`, t, c).Scan(&out.StateVersion); err != nil {
		return out, err
	}
	processes, err := r.ListProcesses(ctx, t, c)
	if err != nil {
		return out, err
	}
	for _, p := range processes {
		if p.ApprovalStatus == "approved" {
			p.Events = filterApprovedEvents(p.Events)
			p.Hypotheses = filterApprovedHypotheses(p.Hypotheses)
			out.Processes = append(out.Processes, p)
		}
	}
	hypotheses, err := r.ListHypotheses(ctx, t, c)
	if err != nil {
		return out, err
	}
	for _, hypothesis := range hypotheses {
		if hypothesis.ApprovalStatus == "approved" && hypothesis.ProcessID == nil {
			out.UnassignedHypotheses = append(out.UnassignedHypotheses, hypothesis)
		}
	}
	events, err := r.ListEvents(ctx, t, c)
	if err != nil {
		return out, err
	}
	for _, e := range events {
		if e.ApprovalStatus == "approved" {
			out.RecentEvents = append(out.RecentEvents, e)
			if len(out.RecentEvents) == 20 {
				break
			}
		}
	}
	evidence, err := r.ListEvidence(ctx, t, c)
	if err != nil {
		return out, err
	}
	for _, e := range evidence {
		if e.Status == "active" {
			out.ActiveEvidence = append(out.ActiveEvidence, e)
		}
	}
	diffs, err := r.ListDiffs(ctx, t, c)
	if err != nil {
		return out, err
	}
	for _, d := range diffs {
		if d.Status == "draft" || d.Status == "pending_review" || d.Status == "partially_reviewed" || d.Status == "approved" {
			out.OpenProposals = append(out.OpenProposals, d)
		}
	}
	return out, nil
}

const diffSelect = `SELECT d.id,d.tenant_id,d.client_id,d.clinical_session_id,d.source_session_report_id,d.source_ai_run_id,d.status,d.base_state_version,d.revision,(d.base_state_version<h.revision AND d.status<>'merged') AS is_stale,d.uncertainties_json,d.created_by_user_id,d.reviewed_by_user_id,d.reviewed_at,d.merged_by_user_id,d.merged_at,d.merged_state_version,d.created_at,d.updated_at FROM clinical_diffs d JOIN clinical_longitudinal_heads h ON h.tenant_id=d.tenant_id AND h.client_id=d.client_id`
const evidenceSelect = `SELECT e.id,e.tenant_id,e.client_id,e.source_type,e.source_id,e.source_version,e.source_item_id,e.epistemic_type,e.statement,e.status,e.version,e.created_by_user_id,e.created_from_ai_run_id,e.created_at,e.updated_at FROM clinical_evidence e`
const eventSelect = `SELECT e.id,e.tenant_id,e.client_id,e.event_type,e.title,e.description,e.occurred_at,e.observed_at,e.approval_status,e.version,e.created_by_user_id,e.created_from_ai_run_id,e.approved_by_user_id,e.approved_at,e.created_at,e.updated_at FROM clinical_events e`
const processSelect = `SELECT p.id,p.tenant_id,p.client_id,p.title,p.description,p.approval_status,p.clinical_status,p.version,p.created_by_user_id,p.created_from_ai_run_id,p.approved_by_user_id,p.approved_at,p.opened_at,p.closed_at,p.created_at,p.updated_at FROM clinical_processes p`
const hypothesisSelect = `SELECT h.id,h.tenant_id,h.client_id,h.process_id,h.statement,h.hypothesis_type,h.approval_status,h.clinical_status,h.confidence_level,h.version,h.created_by_user_id,h.created_from_ai_run_id,h.approved_by_user_id,h.approved_at,h.created_at,h.updated_at FROM clinical_hypotheses h`

func scanDiff(row pgx.Row) (d longitudinal.Diff, err error) {
	var raw []byte
	err = row.Scan(&d.ID, &d.TenantID, &d.ClientID, &d.ClinicalSessionID, &d.SourceSessionReportID, &d.SourceAIRunID, &d.Status, &d.BaseStateVersion, &d.Revision, &d.IsStale, &raw, &d.CreatedByUserID, &d.ReviewedByUserID, &d.ReviewedAt, &d.MergedByUserID, &d.MergedAt, &d.MergedStateVersion, &d.CreatedAt, &d.UpdatedAt)
	if err == nil {
		d.Uncertainties = []longitudinal.Uncertainty{}
		err = json.Unmarshal(raw, &d.Uncertainties)
	}
	return
}
func (r *ClinicalLongitudinalRepository) listOperations(ctx context.Context, t, d uuid.UUID) ([]longitudinal.Operation, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,tenant_id,client_id,diff_id,sequence,operation_type,target_entity_id,expected_entity_version,original_proposal,human_modification,review_status,reviewed_by_user_id,reviewed_at,created_at FROM clinical_diff_operations WHERE tenant_id=$1 AND diff_id=$2 ORDER BY sequence`, t, d)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.Operation{}
	for rows.Next() {
		var x longitudinal.Operation
		if err = rows.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.DiffID, &x.Sequence, &x.OperationType, &x.TargetEntityID, &x.ExpectedEntityVersion, &x.OriginalProposal, &x.HumanModification, &x.ReviewStatus, &x.ReviewedByUserID, &x.ReviewedAt, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func scanEvidence(row pgx.Row) (x longitudinal.Evidence, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.SourceType, &x.SourceID, &x.SourceVersion, &x.SourceItemID, &x.EpistemicType, &x.Statement, &x.Status, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.CreatedAt, &x.UpdatedAt)
	return
}
func scanEvent(row pgx.Row) (x longitudinal.Event, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.EventType, &x.Title, &x.Description, &x.OccurredAt, &x.ObservedAt, &x.ApprovalStatus, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.ApprovedByUserID, &x.ApprovedAt, &x.CreatedAt, &x.UpdatedAt)
	x.Evidence = []longitudinal.Evidence{}
	return
}
func scanProcess(row pgx.Row) (x longitudinal.Process, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.Title, &x.Description, &x.ApprovalStatus, &x.ClinicalStatus, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.ApprovedByUserID, &x.ApprovedAt, &x.OpenedAt, &x.ClosedAt, &x.CreatedAt, &x.UpdatedAt)
	x.Events = []longitudinal.Event{}
	x.Hypotheses = []longitudinal.Hypothesis{}
	return
}
func scanHypothesis(row pgx.Row) (x longitudinal.Hypothesis, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.ProcessID, &x.Statement, &x.HypothesisType, &x.ApprovalStatus, &x.ClinicalStatus, &x.ConfidenceLevel, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.ApprovedByUserID, &x.ApprovedAt, &x.CreatedAt, &x.UpdatedAt)
	x.SupportingEvidence = []longitudinal.Evidence{}
	x.ContradictingEvidence = []longitudinal.Evidence{}
	return
}
func (r *ClinicalLongitudinalRepository) eventEvidence(ctx context.Context, t, e uuid.UUID) ([]longitudinal.Evidence, error) {
	rows, err := r.pool.Query(ctx, evidenceSelect+` JOIN clinical_event_evidence l ON l.tenant_id=e.tenant_id AND l.evidence_id=e.id WHERE l.tenant_id=$1 AND l.event_id=$2 ORDER BY e.created_at,e.id`, t, e)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.Evidence{}
	for rows.Next() {
		x, e := scanEvidence(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) processEvents(ctx context.Context, t, p uuid.UUID) ([]longitudinal.Event, error) {
	rows, err := r.pool.Query(ctx, eventSelect+` JOIN clinical_process_events l ON l.tenant_id=e.tenant_id AND l.event_id=e.id WHERE l.tenant_id=$1 AND l.process_id=$2 ORDER BY e.observed_at,e.id`, t, p)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.Event{}
	for rows.Next() {
		x, e := scanEvent(rows)
		if e != nil {
			return nil, e
		}
		x.Evidence, e = r.eventEvidence(ctx, t, x.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) processHypotheses(ctx context.Context, t, p uuid.UUID) ([]longitudinal.Hypothesis, error) {
	rows, err := r.pool.Query(ctx, hypothesisSelect+` WHERE h.tenant_id=$1 AND h.process_id=$2 ORDER BY h.created_at,h.id`, t, p)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.Hypothesis{}
	for rows.Next() {
		x, e := scanHypothesis(rows)
		if e != nil {
			return nil, e
		}
		if e = r.loadHypothesisEvidence(ctx, t, &x); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) loadHypothesisEvidence(ctx context.Context, t uuid.UUID, h *longitudinal.Hypothesis) error {
	rows, err := r.pool.Query(ctx, evidenceSelect+` JOIN clinical_hypothesis_evidence l ON l.tenant_id=e.tenant_id AND l.evidence_id=e.id WHERE l.tenant_id=$1 AND l.hypothesis_id=$2 ORDER BY l.relation_type,e.created_at,e.id`, t, h.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		x, e := scanEvidence(rows)
		if e != nil {
			return e
		}
		var relation string
		if e = r.pool.QueryRow(ctx, `SELECT relation_type FROM clinical_hypothesis_evidence WHERE tenant_id=$1 AND hypothesis_id=$2 AND evidence_id=$3`, t, h.ID, x.ID).Scan(&relation); e != nil {
			return e
		}
		if relation == "supporting" {
			h.SupportingEvidence = append(h.SupportingEvidence, x)
		} else {
			h.ContradictingEvidence = append(h.ContradictingEvidence, x)
		}
	}
	return rows.Err()
}
func filterApprovedEvents(in []longitudinal.Event) []longitudinal.Event {
	out := []longitudinal.Event{}
	for _, x := range in {
		if x.ApprovalStatus == "approved" {
			out = append(out, x)
		}
	}
	return out
}
func filterApprovedHypotheses(in []longitudinal.Hypothesis) []longitudinal.Hypothesis {
	out := []longitudinal.Hypothesis{}
	for _, x := range in {
		if x.ApprovalStatus == "approved" {
			out = append(out, x)
		}
	}
	return out
}
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}

var _ = fmt.Sprintf
