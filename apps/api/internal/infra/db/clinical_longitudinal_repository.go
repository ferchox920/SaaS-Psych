package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
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
	id, err := createLongitudinalDiffTx(ctx, tx, in)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return longitudinal.Diff{}, err
	}
	return r.GetDiff(ctx, in.TenantID, id)
}

func createLongitudinalDiffTx(ctx context.Context, tx pgx.Tx, in longitudinal.CreateDiffInput) (uuid.UUID, error) {
	var err error
	_, err = tx.Exec(ctx, `INSERT INTO clinical_longitudinal_heads(tenant_id,client_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, in.TenantID, in.ClientID)
	if err != nil {
		return uuid.Nil, err
	}
	uncertainties, _ := json.Marshal(in.Uncertainties)
	id := uuid.New()
	if len(in.OutputHash) != 64 {
		return uuid.Nil, domainerrors.NewValidation("longitudinal output hash is required")
	}
	metadata := in.RunMetadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	runMetadata, err := json.Marshal(metadata)
	if err != nil {
		return uuid.Nil, domainerrors.NewValidation("invalid AI run metadata")
	}
	if in.ExternalProposalID == uuid.Nil {
		tag, err := tx.Exec(ctx, `UPDATE clinical_ai_runs SET status='succeeded',output_hash=$3,parameters_json=parameters_json || $5::jsonb,completed_at=NOW(),updated_at=NOW() WHERE tenant_id=$1 AND id=$2 AND client_id=$4 AND status='running'`, in.TenantID, in.RunID, in.OutputHash, in.ClientID, runMetadata)
		if err != nil {
			return uuid.Nil, err
		}
		if tag.RowsAffected() != 1 {
			return uuid.Nil, domainerrors.ErrConflict
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO clinical_diffs(id,tenant_id,client_id,clinical_session_id,source_session_report_id,source_ai_run_id,source_external_proposal_id,status,base_state_version,uncertainties_json,created_by_user_id) VALUES($1,$2,$3,NULLIF($4,'00000000-0000-0000-0000-000000000000'::uuid),NULLIF($5,'00000000-0000-0000-0000-000000000000'::uuid),NULLIF($6,'00000000-0000-0000-0000-000000000000'::uuid),NULLIF($10,'00000000-0000-0000-0000-000000000000'::uuid),'pending_review',$7,$8,$9)`, id, in.TenantID, in.ClientID, in.SessionID, in.ReportID, in.RunID, in.BaseStateVersion, uncertainties, in.ActorID, in.ExternalProposalID)
	if isUniqueViolation(err) {
		return uuid.Nil, domainerrors.ErrConflict
	}
	if err != nil {
		return uuid.Nil, err
	}
	for i, op := range in.Operations {
		op.ID = uuid.New()
		op.DiffID = id
		op.Sequence = i + 1
		op.TenantID = in.TenantID
		op.ClientID = in.ClientID
		if _, err = tx.Exec(ctx, `INSERT INTO clinical_diff_operations(id,tenant_id,client_id,diff_id,sequence,operation_type,target_entity_id,expected_entity_version,original_proposal) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, op.ID, in.TenantID, in.ClientID, id, op.Sequence, op.OperationType, op.TargetEntityID, op.ExpectedEntityVersion, op.OriginalProposal); err != nil {
			return uuid.Nil, err
		}
	}
	if err := insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, "clinical_diff.created", "clinical_diff", id, map[string]any{"operation_count": len(in.Operations), "base_state_version": in.BaseStateVersion, "source_ai_run_id": in.RunID, "source_external_proposal_id": in.ExternalProposalID}); err != nil {
		return uuid.Nil, err
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
		case "create_target":
			action, entity = "clinical_target.proposed", "clinical_target"
		case "create_goal":
			action, entity = "clinical_goal.proposed", "clinical_goal"
		case "create_goal_indicator":
			action, entity = "goal_indicator.proposed", "goal_indicator"
		case "create_therapeutic_rationale":
			action, entity = "therapeutic_rationale.proposed", "therapeutic_rationale"
		case "create_gira":
			action, entity = "gira.proposed", "gira"
		case "create_gira_phase":
			action, entity = "gira_phase.proposed", "gira_phase"
		}
		if action != "" {
			if err := insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, action, entity, op.TargetEntityID, map[string]any{"diff_id": id, "operation_type": op.OperationType}); err != nil {
				return uuid.Nil, err
			}
		}
	}
	return id, nil
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
	return r.listDiffs(ctx, t, c, false)
}
func (r *ClinicalLongitudinalRepository) listDiffs(ctx context.Context, t, c uuid.UUID, openOnly bool) ([]longitudinal.Diff, error) {
	query := diffSelect + ` WHERE d.tenant_id=$1 AND d.client_id=$2`
	if openOnly {
		query += ` AND d.status IN ('draft','pending_review','partially_reviewed','approved')`
	}
	query += ` ORDER BY d.created_at DESC,d.id DESC`
	rows, err := r.pool.Query(ctx, query, t, c)
	if err != nil {
		return nil, err
	}
	out := []longitudinal.Diff{}
	for rows.Next() {
		d, e := scanDiff(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(out) == 0 {
		return out, err
	}
	byID := make(map[uuid.UUID]int, len(out))
	ids := make([]uuid.UUID, 0, len(out))
	for i := range out {
		out[i].Operations = []longitudinal.Operation{}
		byID[out[i].ID] = i
		ids = append(ids, out[i].ID)
	}
	opRows, err := r.pool.Query(ctx, `SELECT o.id,o.tenant_id,o.client_id,o.diff_id,o.sequence,o.operation_type,o.target_entity_id,o.expected_entity_version,o.original_proposal,o.human_modification,o.review_status,o.reviewed_by_user_id,o.reviewed_at,o.created_at FROM clinical_diff_operations o JOIN clinical_diffs d ON d.tenant_id=o.tenant_id AND d.id=o.diff_id AND d.client_id=o.client_id WHERE d.tenant_id=$1 AND d.client_id=$2 AND d.id=ANY($3::uuid[]) ORDER BY o.diff_id,o.sequence`, t, c, ids)
	if err != nil {
		return nil, err
	}
	defer opRows.Close()
	for opRows.Next() {
		var op longitudinal.Operation
		if err := opRows.Scan(&op.ID, &op.TenantID, &op.ClientID, &op.DiffID, &op.Sequence, &op.OperationType, &op.TargetEntityID, &op.ExpectedEntityVersion, &op.OriginalProposal, &op.HumanModification, &op.ReviewStatus, &op.ReviewedByUserID, &op.ReviewedAt, &op.CreatedAt); err != nil {
			return nil, err
		}
		if i, ok := byID[op.DiffID]; ok {
			out[i].Operations = append(out[i].Operations, op)
		}
	}
	return out, opRows.Err()
}

func (r *ClinicalLongitudinalRepository) Decide(ctx context.Context, in longitudinal.DecisionInput) (longitudinal.Diff, error) {
	if in.ExpectedDiffRevision < 1 || (in.Decision != "approved" && in.Decision != "modified" && in.Decision != "rejected") {
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
	if err := insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, "clinical_diff.reviewed", "clinical_diff", in.DiffID, map[string]any{"operation_id": in.OperationID, "decision": in.Decision, "status": newStatus}); err != nil {
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
			if err := insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, action, entity, targetEntityID, map[string]any{"diff_id": in.DiffID, "operation_id": in.OperationID}); err != nil {
				return longitudinal.Diff{}, err
			}
		}
	}
	if newStatus == "rejected" {
		if err := insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, "clinical_diff.rejected", "clinical_diff", in.DiffID, map[string]any{"revision": revision + 1}); err != nil {
			return longitudinal.Diff{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return longitudinal.Diff{}, err
	}
	return r.GetDiff(ctx, in.TenantID, in.DiffID)
}

func (r *ClinicalLongitudinalRepository) ListEvidence(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Evidence, error) {
	return r.listEvidence(ctx, t, c, false)
}
func (r *ClinicalLongitudinalRepository) listEvidence(ctx context.Context, t, c uuid.UUID, activeOnly bool) ([]longitudinal.Evidence, error) {
	query := evidenceSelect + ` WHERE e.tenant_id=$1 AND e.client_id=$2`
	if activeOnly {
		query += ` AND e.status='active'`
	}
	query += ` ORDER BY e.created_at DESC,e.id DESC`
	rows, err := r.pool.Query(ctx, query, t, c)
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
	return r.listEvents(ctx, t, c, 0, 0, nil, false)
}
func (r *ClinicalLongitudinalRepository) ListEventsPage(ctx context.Context, t, c uuid.UUID, limit, offset int) ([]longitudinal.Event, error) {
	return r.listEvents(ctx, t, c, limit, offset, nil, false)
}
func (r *ClinicalLongitudinalRepository) listEvents(ctx context.Context, t, c uuid.UUID, limit, offset int, processIDs []uuid.UUID, approvedOnly bool) ([]longitudinal.Event, error) {
	query := eventSelect + ` WHERE e.tenant_id=$1 AND e.client_id=$2`
	args := []any{t, c}
	if approvedOnly {
		query += ` AND e.approval_status='approved'`
	}
	if processIDs != nil {
		query += ` AND e.id IN (SELECT event_id FROM clinical_process_events WHERE tenant_id=$1 AND client_id=$2 AND process_id=ANY($3::uuid[]))`
		args = append(args, processIDs)
	}
	query += ` ORDER BY e.observed_at DESC,e.id DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := []longitudinal.Event{}
	for rows.Next() {
		x, e := scanEvent(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(out) == 0 {
		return out, err
	}
	if err := r.attachEventEvidence(ctx, t, c, out); err != nil {
		return nil, err
	}
	return out, nil
}
func (r *ClinicalLongitudinalRepository) ListProcesses(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Process, error) {
	return r.listProcesses(ctx, t, c, 0, 0, false)
}
func (r *ClinicalLongitudinalRepository) ListProcessesPage(ctx context.Context, t, c uuid.UUID, limit, offset int) ([]longitudinal.Process, error) {
	return r.listProcesses(ctx, t, c, limit, offset, false)
}
func (r *ClinicalLongitudinalRepository) listProcesses(ctx context.Context, t, c uuid.UUID, limit, offset int, approvedOnly bool) ([]longitudinal.Process, error) {
	query := processSelect + ` WHERE p.tenant_id=$1 AND p.client_id=$2`
	if approvedOnly {
		query += ` AND p.approval_status='approved'`
	}
	query += ` ORDER BY CASE p.clinical_status WHEN 'active' THEN 1 WHEN 'observing' THEN 2 WHEN 'stabilized' THEN 3 ELSE 4 END,p.updated_at DESC,p.id DESC`
	args := []any{t, c}
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := []longitudinal.Process{}
	for rows.Next() {
		x, e := scanProcess(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(out) == 0 {
		return out, err
	}
	byID := make(map[uuid.UUID]int, len(out))
	processIDs := make([]uuid.UUID, 0, len(out))
	for i := range out {
		byID[out[i].ID] = i
		processIDs = append(processIDs, out[i].ID)
		out[i].TherapeuticStrategy = &longitudinal.TherapeuticStrategy{Targets: []longitudinal.Target{}, Goals: []longitudinal.Goal{}, Rationales: []longitudinal.TherapeuticRationale{}, GIRAs: []longitudinal.GIRA{}}
	}
	events, err := r.listEvents(ctx, t, c, 0, 0, processIDs, false)
	if err != nil {
		return nil, err
	}
	eventsByID := make(map[uuid.UUID]longitudinal.Event, len(events))
	for _, event := range events {
		eventsByID[event.ID] = event
	}
	links, err := r.pool.Query(ctx, `SELECT l.process_id,l.event_id FROM clinical_process_events l JOIN clinical_processes p ON p.tenant_id=l.tenant_id AND p.id=l.process_id JOIN clinical_events e ON e.tenant_id=l.tenant_id AND e.id=l.event_id WHERE p.tenant_id=$1 AND p.client_id=$2 AND e.client_id=p.client_id AND p.id=ANY($3::uuid[])`, t, c, processIDs)
	if err != nil {
		return nil, err
	}
	for links.Next() {
		var processID, eventID uuid.UUID
		if err := links.Scan(&processID, &eventID); err != nil {
			links.Close()
			return nil, err
		}
		if i, ok := byID[processID]; ok {
			if event, found := eventsByID[eventID]; found {
				out[i].Events = append(out[i].Events, event)
			}
		}
	}
	err = links.Err()
	links.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		sort.Slice(out[i].Events, func(a, b int) bool {
			left, right := out[i].Events[a], out[i].Events[b]
			if left.ObservedAt.Equal(right.ObservedAt) {
				return left.ID.String() < right.ID.String()
			}
			return left.ObservedAt.Before(right.ObservedAt)
		})
	}
	hypotheses, err := r.listHypotheses(ctx, t, c, 0, 0, processIDs, false)
	if err != nil {
		return nil, err
	}
	for _, h := range hypotheses {
		if h.ProcessID != nil {
			if i, ok := byID[*h.ProcessID]; ok {
				out[i].Hypotheses = append(out[i].Hypotheses, h)
			}
		}
	}
	for i := range out {
		sort.Slice(out[i].Hypotheses, func(a, b int) bool {
			left, right := out[i].Hypotheses[a], out[i].Hypotheses[b]
			if left.CreatedAt.Equal(right.CreatedAt) {
				return left.ID.String() < right.ID.String()
			}
			return left.CreatedAt.Before(right.CreatedAt)
		})
	}
	targets, err := r.listTargets(ctx, t, c, 0, 0, processIDs)
	if err != nil {
		return nil, err
	}
	for _, x := range targets {
		if i, ok := byID[x.ProcessID]; ok && x.ApprovalStatus == "approved" {
			out[i].TherapeuticStrategy.Targets = append(out[i].TherapeuticStrategy.Targets, x)
		}
	}
	goals, err := r.listGoals(ctx, t, c, 0, 0, processIDs)
	if err != nil {
		return nil, err
	}
	for _, x := range goals {
		if i, ok := byID[x.ProcessID]; ok && x.ApprovalStatus == "approved" {
			out[i].TherapeuticStrategy.Goals = append(out[i].TherapeuticStrategy.Goals, x)
		}
	}
	rationales, err := r.listRationalesByProcessIDs(ctx, t, c, processIDs)
	if err != nil {
		return nil, err
	}
	for _, x := range rationales {
		if i, ok := byID[x.ProcessID]; ok && x.ApprovalStatus == "approved" {
			out[i].TherapeuticStrategy.Rationales = append(out[i].TherapeuticStrategy.Rationales, x)
		}
	}
	giras, err := r.listGIRAs(ctx, t, c, 0, 0, processIDs)
	if err != nil {
		return nil, err
	}
	for _, x := range giras {
		if i, ok := byID[x.ProcessID]; ok && x.ApprovalStatus == "approved" {
			out[i].TherapeuticStrategy.GIRAs = append(out[i].TherapeuticStrategy.GIRAs, x)
		}
	}
	return out, nil
}
func (r *ClinicalLongitudinalRepository) GetProcess(ctx context.Context, t, id uuid.UUID) (longitudinal.Process, error) {
	x, err := scanProcess(r.pool.QueryRow(ctx, processSelect+` WHERE p.tenant_id=$1 AND p.id=$2`, t, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return x, domainerrors.ErrNotFound
	}
	if err != nil {
		return x, err
	}
	x.Events, err = r.processEvents(ctx, t, x.ID)
	if err != nil {
		return x, err
	}
	x.Hypotheses, err = r.processHypotheses(ctx, t, x.ID)
	if err != nil {
		return x, err
	}
	x.TherapeuticStrategy, err = r.processTherapeuticStrategy(ctx, t, x.ClientID, x.ID, true)
	return x, err
}
func (r *ClinicalLongitudinalRepository) ListHypotheses(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Hypothesis, error) {
	return r.listHypotheses(ctx, t, c, 0, 0, nil, false)
}
func (r *ClinicalLongitudinalRepository) ListHypothesesPage(ctx context.Context, t, c uuid.UUID, limit, offset int) ([]longitudinal.Hypothesis, error) {
	return r.listHypotheses(ctx, t, c, limit, offset, nil, false)
}
func (r *ClinicalLongitudinalRepository) listHypotheses(ctx context.Context, t, c uuid.UUID, limit, offset int, processIDs []uuid.UUID, approvedUnassignedOnly bool) ([]longitudinal.Hypothesis, error) {
	query := hypothesisSelect + ` WHERE h.tenant_id=$1 AND h.client_id=$2`
	args := []any{t, c}
	if approvedUnassignedOnly {
		query += ` AND h.approval_status='approved' AND h.process_id IS NULL`
	}
	if processIDs != nil {
		query += ` AND h.process_id=ANY($3::uuid[])`
		args = append(args, processIDs)
	}
	query += ` ORDER BY h.updated_at DESC,h.id DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := []longitudinal.Hypothesis{}
	for rows.Next() {
		x, e := scanHypothesis(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(out) == 0 {
		return out, err
	}
	if err := r.attachHypothesisEvidence(ctx, t, c, out); err != nil {
		return nil, err
	}
	return out, nil
}
func (r *ClinicalLongitudinalRepository) State(ctx context.Context, t, c uuid.UUID) (longitudinal.State, error) {
	out := longitudinal.State{ClientID: c, Processes: []longitudinal.Process{}, UnassignedHypotheses: []longitudinal.Hypothesis{}, RecentEvents: []longitudinal.Event{}, ActiveEvidence: []longitudinal.Evidence{}, OpenProposals: []longitudinal.Diff{}}
	if err := r.pool.QueryRow(ctx, `SELECT COALESCE((SELECT revision FROM clinical_longitudinal_heads WHERE tenant_id=$1 AND client_id=$2),0)`, t, c).Scan(&out.StateVersion); err != nil {
		return out, err
	}
	processes, err := r.listProcesses(ctx, t, c, 0, 0, true)
	if err != nil {
		return out, err
	}
	for _, p := range processes {
		p.Events = filterApprovedEvents(p.Events)
		p.Hypotheses = filterApprovedHypotheses(p.Hypotheses)
		out.Processes = append(out.Processes, p)
	}
	hypotheses, err := r.listHypotheses(ctx, t, c, 0, 0, nil, true)
	if err != nil {
		return out, err
	}
	out.UnassignedHypotheses = hypotheses
	events, err := r.listEvents(ctx, t, c, 20, 0, nil, true)
	if err != nil {
		return out, err
	}
	out.RecentEvents = events
	evidence, err := r.listEvidence(ctx, t, c, true)
	if err != nil {
		return out, err
	}
	out.ActiveEvidence = evidence
	diffs, err := r.listDiffs(ctx, t, c, true)
	if err != nil {
		return out, err
	}
	out.OpenProposals = diffs
	return out, nil
}

const diffSelect = `SELECT d.id,d.tenant_id,d.client_id,d.clinical_session_id,d.source_session_report_id,d.source_ai_run_id,d.source_external_proposal_id,d.status,d.base_state_version,d.revision,(d.base_state_version<h.revision AND d.status<>'merged') AS is_stale,d.uncertainties_json,d.created_by_user_id,d.reviewed_by_user_id,d.reviewed_at,d.merged_by_user_id,d.merged_at,d.merged_state_version,d.created_at,d.updated_at FROM clinical_diffs d JOIN clinical_longitudinal_heads h ON h.tenant_id=d.tenant_id AND h.client_id=d.client_id`
const evidenceSelect = `SELECT e.id,e.tenant_id,e.client_id,e.source_type,e.source_id,e.source_version,e.source_item_id,e.epistemic_type,e.statement,e.status,e.version,e.created_by_user_id,e.created_from_ai_run_id,e.created_at,e.updated_at FROM clinical_evidence e`
const eventSelect = `SELECT e.id,e.tenant_id,e.client_id,e.event_type,e.title,e.description,e.occurred_at,e.observed_at,e.approval_status,e.version,e.created_by_user_id,e.created_from_ai_run_id,e.approved_by_user_id,e.approved_at,e.created_at,e.updated_at FROM clinical_events e`
const processSelect = `SELECT p.id,p.tenant_id,p.client_id,p.title,p.description,p.approval_status,p.clinical_status,p.version,p.created_by_user_id,p.created_from_ai_run_id,p.approved_by_user_id,p.approved_at,p.opened_at,p.closed_at,p.created_at,p.updated_at FROM clinical_processes p`
const hypothesisSelect = `SELECT h.id,h.tenant_id,h.client_id,h.process_id,h.statement,h.hypothesis_type,h.approval_status,h.clinical_status,h.confidence_level,h.version,h.created_by_user_id,h.created_from_ai_run_id,h.approved_by_user_id,h.approved_at,h.created_at,h.updated_at FROM clinical_hypotheses h`

func scanDiff(row pgx.Row) (d longitudinal.Diff, err error) {
	var raw []byte
	err = row.Scan(&d.ID, &d.TenantID, &d.ClientID, &d.ClinicalSessionID, &d.SourceSessionReportID, &d.SourceAIRunID, &d.SourceExternalProposalID, &d.Status, &d.BaseStateVersion, &d.Revision, &d.IsStale, &raw, &d.CreatedByUserID, &d.ReviewedByUserID, &d.ReviewedAt, &d.MergedByUserID, &d.MergedAt, &d.MergedStateVersion, &d.CreatedAt, &d.UpdatedAt)
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
		if err := rows.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.DiffID, &x.Sequence, &x.OperationType, &x.TargetEntityID, &x.ExpectedEntityVersion, &x.OriginalProposal, &x.HumanModification, &x.ReviewStatus, &x.ReviewedByUserID, &x.ReviewedAt, &x.CreatedAt); err != nil {
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
func scanLinkedEvidence(row pgx.Row) (uuid.UUID, string, longitudinal.Evidence, error) {
	var owner uuid.UUID
	var relation string
	var x longitudinal.Evidence
	err := row.Scan(&owner, &relation, &x.ID, &x.TenantID, &x.ClientID, &x.SourceType, &x.SourceID, &x.SourceVersion, &x.SourceItemID, &x.EpistemicType, &x.Statement, &x.Status, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.CreatedAt, &x.UpdatedAt)
	return owner, relation, x, err
}
func (r *ClinicalLongitudinalRepository) attachEventEvidence(ctx context.Context, tenantID, clientID uuid.UUID, events []longitudinal.Event) error {
	byID := make(map[uuid.UUID]int, len(events))
	ids := make([]uuid.UUID, 0, len(events))
	for i := range events {
		byID[events[i].ID] = i
		ids = append(ids, events[i].ID)
	}
	rows, err := r.pool.Query(ctx, `SELECT l.event_id,'' AS relation,`+strings.TrimPrefix(evidenceSelect, "SELECT ")+` JOIN clinical_event_evidence l ON l.tenant_id=e.tenant_id AND l.evidence_id=e.id JOIN clinical_events ev ON ev.tenant_id=l.tenant_id AND ev.id=l.event_id AND ev.client_id=e.client_id WHERE ev.tenant_id=$1 AND ev.client_id=$2 AND ev.id=ANY($3::uuid[]) ORDER BY l.event_id,e.created_at,e.id`, tenantID, clientID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		owner, _, x, err := scanLinkedEvidence(rows)
		if err != nil {
			return err
		}
		if i, ok := byID[owner]; ok {
			events[i].Evidence = append(events[i].Evidence, x)
		}
	}
	return rows.Err()
}
func (r *ClinicalLongitudinalRepository) attachHypothesisEvidence(ctx context.Context, tenantID, clientID uuid.UUID, hypotheses []longitudinal.Hypothesis) error {
	byID := make(map[uuid.UUID]int, len(hypotheses))
	ids := make([]uuid.UUID, 0, len(hypotheses))
	for i := range hypotheses {
		byID[hypotheses[i].ID] = i
		ids = append(ids, hypotheses[i].ID)
	}
	rows, err := r.pool.Query(ctx, `SELECT l.hypothesis_id,l.relation_type,`+strings.TrimPrefix(evidenceSelect, "SELECT ")+` JOIN clinical_hypothesis_evidence l ON l.tenant_id=e.tenant_id AND l.evidence_id=e.id JOIN clinical_hypotheses h ON h.tenant_id=l.tenant_id AND h.id=l.hypothesis_id AND h.client_id=e.client_id WHERE h.tenant_id=$1 AND h.client_id=$2 AND h.id=ANY($3::uuid[]) ORDER BY l.hypothesis_id,l.relation_type,e.created_at,e.id`, tenantID, clientID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		owner, relation, x, err := scanLinkedEvidence(rows)
		if err != nil {
			return err
		}
		if i, ok := byID[owner]; ok {
			if relation == "supporting" {
				hypotheses[i].SupportingEvidence = append(hypotheses[i].SupportingEvidence, x)
			} else {
				hypotheses[i].ContradictingEvidence = append(hypotheses[i].ContradictingEvidence, x)
			}
		}
	}
	return rows.Err()
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
		if e := r.loadHypothesisEvidence(ctx, t, &x); e != nil {
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
		if e := r.pool.QueryRow(ctx, `SELECT relation_type FROM clinical_hypothesis_evidence WHERE tenant_id=$1 AND hypothesis_id=$2 AND evidence_id=$3`, t, h.ID, x.ID).Scan(&relation); e != nil {
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

func (r *ClinicalLongitudinalRepository) GetProcessHistory(ctx context.Context, t, c, id uuid.UUID) (longitudinal.ProcessHistory, error) {
	process, err := scanProcess(r.pool.QueryRow(ctx, processSelect+` WHERE p.tenant_id=$1 AND p.client_id=$2 AND p.id=$3`, t, c, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return longitudinal.ProcessHistory{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return longitudinal.ProcessHistory{}, err
	}
	process.Events, err = r.processEvents(ctx, t, id)
	if err != nil {
		return longitudinal.ProcessHistory{}, err
	}
	process.Hypotheses, err = r.processHypotheses(ctx, t, id)
	if err != nil {
		return longitudinal.ProcessHistory{}, err
	}
	transitions, err := r.entityHistoryTransitions(ctx, t, c, "process", id)
	return longitudinal.ProcessHistory{Process: process, Transitions: transitions}, err
}

func (r *ClinicalLongitudinalRepository) GetHypothesisHistory(ctx context.Context, t, c, id uuid.UUID) (longitudinal.HypothesisHistory, error) {
	hypothesis, err := scanHypothesis(r.pool.QueryRow(ctx, hypothesisSelect+` WHERE h.tenant_id=$1 AND h.client_id=$2 AND h.id=$3`, t, c, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return longitudinal.HypothesisHistory{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return longitudinal.HypothesisHistory{}, err
	}
	if err := r.loadHypothesisEvidence(ctx, t, &hypothesis); err != nil {
		return longitudinal.HypothesisHistory{}, err
	}
	transitions, err := r.entityHistoryTransitions(ctx, t, c, "hypothesis", id)
	return longitudinal.HypothesisHistory{Hypothesis: hypothesis, Transitions: transitions}, err
}

func (r *ClinicalLongitudinalRepository) entityHistoryTransitions(ctx context.Context, t, c uuid.UUID, entityType string, id uuid.UUID) ([]longitudinal.HistoryTransition, error) {
	rows, err := r.pool.Query(ctx, `SELECT tr.id,tr.diff_id,tr.operation_id,tr.entity_type,tr.entity_id,tr.action,tr.from_version,tr.to_version,tr.from_status,tr.to_status,tr.actor_user_id,d.merged_by_user_id,d.merged_at,o.original_proposal,o.human_modification,tr.created_at FROM clinical_longitudinal_transitions tr JOIN clinical_diffs d ON d.tenant_id=tr.tenant_id AND d.id=tr.diff_id JOIN clinical_diff_operations o ON o.tenant_id=tr.tenant_id AND o.id=tr.operation_id WHERE tr.tenant_id=$1 AND tr.client_id=$2 AND tr.entity_type=$3 AND tr.entity_id=$4 ORDER BY tr.created_at,tr.id`, t, c, entityType, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.HistoryTransition{}
	for rows.Next() {
		var item longitudinal.HistoryTransition
		if err := rows.Scan(&item.ID, &item.DiffID, &item.OperationID, &item.EntityType, &item.EntityID, &item.Action, &item.FromVersion, &item.ToVersion, &item.FromStatus, &item.ToStatus, &item.ActorUserID, &item.MergedByUserID, &item.MergedAt, &item.OriginalProposal, &item.HumanModification, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}

var _ = fmt.Sprintf
