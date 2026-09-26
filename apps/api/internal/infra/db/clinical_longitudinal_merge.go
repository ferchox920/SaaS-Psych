package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

func (r *ClinicalLongitudinalRepository) Merge(ctx context.Context, in longitudinal.MergeInput) (longitudinal.Diff, error) {
	if in.ExpectedDiffRevision < 1 || in.ActorID == uuid.Nil {
		return longitudinal.Diff{}, domainerrors.NewValidation("expected_diff_revision and human actor are required")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var client uuid.UUID
	var status string
	var revision int
	var base int64
	var mergedVersion *int64
	var runID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT client_id,status,revision,base_state_version,merged_state_version,source_ai_run_id FROM clinical_diffs WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, in.TenantID, in.DiffID).Scan(&client, &status, &revision, &base, &mergedVersion, &runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return longitudinal.Diff{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return longitudinal.Diff{}, err
	}
	if status == "merged" {
		if err := tx.Commit(ctx); err != nil {
			return longitudinal.Diff{}, err
		}
		return r.GetDiff(ctx, in.TenantID, in.DiffID)
	}
	if status != "approved" || revision != in.ExpectedDiffRevision {
		return longitudinal.Diff{}, domainerrors.ErrConflict
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text||':'||$2::text,0))`, in.TenantID, client); err != nil {
		return longitudinal.Diff{}, err
	}
	var head int64
	err = tx.QueryRow(ctx, `SELECT revision FROM clinical_longitudinal_heads WHERE tenant_id=$1 AND client_id=$2 FOR UPDATE`, in.TenantID, client).Scan(&head)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	if head != base {
		return longitudinal.Diff{}, domainerrors.ErrConflict
	}
	rows, err := tx.Query(ctx, `SELECT id,operation_type,target_entity_id,expected_entity_version,original_proposal,human_modification,review_status FROM clinical_diff_operations WHERE tenant_id=$1 AND diff_id=$2 AND review_status IN('approved','modified') ORDER BY sequence`, in.TenantID, in.DiffID)
	if err != nil {
		return longitudinal.Diff{}, err
	}
	type mergeOp struct {
		id              uuid.UUID
		kind            string
		target          uuid.UUID
		expected        *int
		original, human []byte
		review          string
	}
	ops := []mergeOp{}
	for rows.Next() {
		var op mergeOp
		if err = rows.Scan(&op.id, &op.kind, &op.target, &op.expected, &op.original, &op.human, &op.review); err != nil {
			rows.Close()
			return longitudinal.Diff{}, err
		}
		ops = append(ops, op)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return longitudinal.Diff{}, err
	}
	rows.Close()
	for _, op := range ops {
		payload := op.original
		if op.review == "modified" {
			payload = op.human
		}
		if err := longitudinal.ValidateProposal(op.kind, payload); err != nil {
			return longitudinal.Diff{}, err
		}
		if err := r.applyLongitudinalOperation(ctx, tx, in, client, runID, op.id, op.kind, op.target, op.expected, payload, op.review == "modified"); err != nil {
			return longitudinal.Diff{}, err
		}
	}
	newHead := head + 1
	if _, err = tx.Exec(ctx, `UPDATE clinical_longitudinal_heads SET revision=$3,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2`, in.TenantID, client, newHead); err != nil {
		return longitudinal.Diff{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE clinical_diffs SET status='merged',merged_by_user_id=$3,merged_at=NOW(),merged_state_version=$4,revision=revision+1,updated_at=NOW() WHERE tenant_id=$1 AND id=$2`, in.TenantID, in.DiffID, in.ActorID, newHead); err != nil {
		return longitudinal.Diff{}, err
	}
	if err := insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, "clinical_diff.merged", "clinical_diff", in.DiffID, map[string]any{"operation_count": len(ops), "from_state_version": head, "to_state_version": newHead}); err != nil {
		return longitudinal.Diff{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return longitudinal.Diff{}, err
	}
	return r.GetDiff(ctx, in.TenantID, in.DiffID)
}

func (r *ClinicalLongitudinalRepository) applyLongitudinalOperation(ctx context.Context, tx pgx.Tx, in longitudinal.MergeInput, client uuid.UUID, runID *uuid.UUID, opID uuid.UUID, kind string, target uuid.UUID, expected *int, payload []byte, modified bool) error {
	now := time.Now().UTC()
	entity, action, toStatus := "", "", ""
	fromVersion := (*int)(nil)
	toVersion := 1
	fromStatus := ""
	requireExpected := func() error {
		if expected == nil {
			return domainerrors.ErrConflict
		}
		return nil
	}
	switch kind {
	case "create_evidence":
		if expected != nil {
			return domainerrors.ErrConflict
		}
		var p longitudinal.CreateEvidenceProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		var reportID uuid.UUID
		var reportVersion int
		var statement string
		err := tx.QueryRow(ctx, `SELECT r.id,r.version,CASE WHEN item->>'statement' IS NOT NULL THEN item->>'statement' ELSE item->>'description' END FROM clinical_diffs d JOIN session_reports r ON r.tenant_id=d.tenant_id AND r.id=d.source_session_report_id CROSS JOIN LATERAL jsonb_array_elements(COALESCE(r.report_json->'facts','[]'::jsonb)||COALESCE(r.report_json->'relevant_changes','[]'::jsonb)||COALESCE(r.report_json->'patient_responses','[]'::jsonb)||COALESCE(r.report_json->'affective_nodes','[]'::jsonb)) item WHERE d.tenant_id=$1 AND d.id=$2 AND item->>'id'=$3`, in.TenantID, in.DiffID, p.SourceItemID).Scan(&reportID, &reportVersion, &statement)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.NewValidation("create_evidence source item is not eligible")
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO clinical_evidence(id,tenant_id,client_id,source_type,source_id,source_version,source_item_id,epistemic_type,statement,status,created_by_user_id,created_from_ai_run_id) VALUES($1,$2,$3,'session_report',$4,$5,$6,$7,$8,'active',$9,$10)`, target, in.TenantID, client, reportID, reportVersion, p.SourceItemID, p.EpistemicType, statement, in.ActorID, runID)
		if isUniqueViolation(err) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		entity, action, toStatus = "evidence", "clinical_evidence.created", "active"
	case "invalidate_evidence":
		if err := requireExpected(); err != nil {
			return err
		}
		var p longitudinal.InvalidateEvidenceProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		var oldV int
		var oldS string
		err := tx.QueryRow(ctx, `UPDATE clinical_evidence SET status='invalidated',version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND status='active' RETURNING version-1,status`, in.TenantID, client, target, *expected).Scan(&oldV, &oldS)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		fromVersion = &oldV
		toVersion = oldV + 1
		fromStatus = "active"
		entity, action, toStatus = "evidence", "clinical_evidence.invalidated", "invalidated"
	case "create_event":
		if expected != nil {
			return domainerrors.ErrConflict
		}
		var p longitudinal.CreateEventProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return err
		}
		if p.InterventionSourceItemID != nil {
			var ok bool
			err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clinical_diffs d JOIN session_reports r ON r.tenant_id=d.tenant_id AND r.id=d.source_session_report_id CROSS JOIN LATERAL jsonb_array_elements(COALESCE(r.report_json->'interventions','[]'::jsonb)) i WHERE d.tenant_id=$1 AND d.id=$2 AND i->>'id'=$3 AND EXISTS(SELECT 1 FROM clinical_evidence e CROSS JOIN LATERAL jsonb_array_elements(COALESCE(r.report_json->'patient_responses','[]'::jsonb)) response WHERE e.tenant_id=d.tenant_id AND e.client_id=d.client_id AND e.id=ANY($4::uuid[]) AND e.source_id=r.id AND response->>'id'=e.source_item_id))`, in.TenantID, in.DiffID, *p.InterventionSourceItemID, p.EvidenceIDs).Scan(&ok)
			if err != nil {
				return err
			}
			if !ok {
				return domainerrors.NewValidation("therapeutic response intervention source not found")
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO clinical_events(id,tenant_id,client_id,event_type,title,description,occurred_at,observed_at,approval_status,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'approved',$9,$10,$9,$11)`, target, in.TenantID, client, p.EventType, p.Title, p.Description, p.OccurredAt, p.ObservedAt, in.ActorID, runID, now)
		if err != nil {
			return err
		}
		for _, e := range p.EvidenceIDs {
			if _, err = tx.Exec(ctx, `INSERT INTO clinical_event_evidence(tenant_id,client_id,event_id,evidence_id) VALUES($1,$2,$3,$4)`, in.TenantID, client, target, e); err != nil {
				return err
			}
		}
		entity, action, toStatus = "event", "clinical_event.approved", "approved"
		if modified {
			action = "clinical_event.corrected"
		}
	case "approve_event":
		if err := requireExpected(); err != nil {
			return err
		}
		var p longitudinal.ApproveEventProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return err
		}
		for _, e := range p.EvidenceIDs {
			_, err := tx.Exec(ctx, `INSERT INTO clinical_event_evidence(tenant_id,client_id,event_id,evidence_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, in.TenantID, client, target, e)
			if err != nil {
				return err
			}
		}
		var oldV int
		err := tx.QueryRow(ctx, `UPDATE clinical_events SET approval_status='approved',approved_by_user_id=$5,approved_at=$6,version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='proposed' RETURNING version-1`, in.TenantID, client, target, *expected, in.ActorID, now).Scan(&oldV)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		fromVersion = &oldV
		toVersion = oldV + 1
		fromStatus = "proposed"
		entity, action, toStatus = "event", "clinical_event.approved", "approved"
	case "create_process":
		if expected != nil {
			return domainerrors.ErrConflict
		}
		var p longitudinal.CreateProcessProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO clinical_processes(id,tenant_id,client_id,title,description,approval_status,clinical_status,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at,opened_at) VALUES($1,$2,$3,$4,$5,'approved',$6,$7,$8,$7,$9,$9)`, target, in.TenantID, client, p.Title, p.Description, p.ClinicalStatus, in.ActorID, runID, now)
		if err != nil {
			return err
		}
		entity, action, toStatus = "process", "clinical_process.approved", p.ClinicalStatus
	case "update_process":
		if err := requireExpected(); err != nil {
			return err
		}
		var p longitudinal.UpdateProcessProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return err
		}
		var oldV int
		err := tx.QueryRow(ctx, `UPDATE clinical_processes SET title=$5,description=$6,version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' RETURNING version-1,clinical_status`, in.TenantID, client, target, *expected, p.Title, p.Description).Scan(&oldV, &toStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		fromVersion = &oldV
		toVersion = oldV + 1
		fromStatus = toStatus
		entity, action = "process", "clinical_process.updated"
	case "close_process", "reopen_process":
		if err := requireExpected(); err != nil {
			return err
		}
		var p longitudinal.TransitionProcessProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return err
		}
		targetStatus := "closed"
		action = "clinical_process.closed"
		if kind == "reopen_process" {
			targetStatus = p.ReopenStatus
			action = "clinical_process.reopened"
		}
		var oldV int
		err := tx.QueryRow(ctx, `UPDATE clinical_processes SET clinical_status=$5::text,closed_at=CASE WHEN $5::text='closed' THEN $6::timestamptz ELSE NULL END,opened_at=CASE WHEN $5::text<>'closed' THEN $6::timestamptz ELSE opened_at END,version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' RETURNING version-1`, in.TenantID, client, target, *expected, targetStatus, now).Scan(&oldV)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		fromVersion = &oldV
		toVersion = oldV + 1
		entity, toStatus = "process", targetStatus
	case "link_event_process":
		if err := requireExpected(); err != nil {
			return err
		}
		var p longitudinal.LinkEventProcessProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return err
		}
		if p.ProcessID != target {
			return domainerrors.NewValidation("target_entity_id must identify linked process")
		}
		tag, err := tx.Exec(ctx, `INSERT INTO clinical_process_events(tenant_id,client_id,process_id,event_id) SELECT $1,$2,$3,$4 WHERE EXISTS(SELECT 1 FROM clinical_events WHERE tenant_id=$1 AND client_id=$2 AND id=$4 AND approval_status='approved') ON CONFLICT DO NOTHING`, in.TenantID, client, p.ProcessID, p.EventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domainerrors.ErrConflict
		}
		var oldV int
		err = tx.QueryRow(ctx, `UPDATE clinical_processes SET version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' RETURNING version-1,clinical_status`, in.TenantID, client, p.ProcessID, *expected).Scan(&oldV, &toStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		fromVersion = &oldV
		toVersion = oldV + 1
		fromStatus = toStatus
		entity, action = "process", "clinical_process.updated"
	case "create_hypothesis":
		if expected != nil {
			return domainerrors.ErrConflict
		}
		var p longitudinal.CreateHypothesisProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		all := append(append([]uuid.UUID{}, p.SupportingEvidenceIDs...), p.ContradictingEvidenceIDs...)
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, all); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO clinical_hypotheses(id,tenant_id,client_id,process_id,statement,hypothesis_type,approval_status,clinical_status,confidence_level,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,$4,$5,$6,'approved','active',$7,$8,$9,$8,$10)`, target, in.TenantID, client, p.ProcessID, p.Statement, p.HypothesisType, p.ConfidenceLevel, in.ActorID, runID, now)
		if err != nil {
			return err
		}
		for _, e := range p.SupportingEvidenceIDs {
			if _, err = tx.Exec(ctx, `INSERT INTO clinical_hypothesis_evidence(tenant_id,client_id,hypothesis_id,evidence_id,relation_type) VALUES($1,$2,$3,$4,'supporting')`, in.TenantID, client, target, e); err != nil {
				return err
			}
		}
		for _, e := range p.ContradictingEvidenceIDs {
			if _, err = tx.Exec(ctx, `INSERT INTO clinical_hypothesis_evidence(tenant_id,client_id,hypothesis_id,evidence_id,relation_type) VALUES($1,$2,$3,$4,'contradicting')`, in.TenantID, client, target, e); err != nil {
				return err
			}
		}
		entity, action, toStatus = "hypothesis", "clinical_hypothesis.approved", "active"
	case "update_hypothesis":
		if err := requireExpected(); err != nil {
			return err
		}
		var p longitudinal.UpdateHypothesisProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return err
		}
		var oldV int
		err := tx.QueryRow(ctx, `UPDATE clinical_hypotheses SET process_id=$5,statement=$6,hypothesis_type=$7,version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' RETURNING version-1,clinical_status`, in.TenantID, client, target, *expected, p.ProcessID, p.Statement, p.HypothesisType).Scan(&oldV, &toStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		fromVersion = &oldV
		toVersion = oldV + 1
		fromStatus = toStatus
		entity, action = "hypothesis", "clinical_hypothesis.updated"
	case "link_supporting_evidence", "link_contradicting_evidence":
		if err := requireExpected(); err != nil {
			return err
		}
		var p longitudinal.LinkHypothesisEvidenceProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		if p.HypothesisID != target {
			return domainerrors.NewValidation("target_entity_id must identify linked hypothesis")
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, append(p.EvidenceIDs, p.EvidenceID)); err != nil {
			return err
		}
		relation := "supporting"
		if kind == "link_contradicting_evidence" {
			relation = "contradicting"
		}
		_, err := tx.Exec(ctx, `INSERT INTO clinical_hypothesis_evidence(tenant_id,client_id,hypothesis_id,evidence_id,relation_type) VALUES($1,$2,$3,$4,$5)`, in.TenantID, client, target, p.EvidenceID, relation)
		if isUniqueViolation(err) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		var oldV int
		err = tx.QueryRow(ctx, `UPDATE clinical_hypotheses SET version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' RETURNING version-1,clinical_status`, in.TenantID, client, target, *expected).Scan(&oldV, &toStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		fromVersion = &oldV
		toVersion = oldV + 1
		fromStatus = toStatus
		entity, action = "hypothesis", "clinical_hypothesis.updated"
	case "strengthen_hypothesis", "weaken_hypothesis", "retire_hypothesis":
		if err := requireExpected(); err != nil {
			return err
		}
		var p longitudinal.TransitionHypothesisProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return err
		}
		var oldV int
		var oldConfidence string
		var oldClinical string
		err := tx.QueryRow(ctx, `SELECT version,confidence_level,clinical_status FROM clinical_hypotheses WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' FOR UPDATE`, in.TenantID, client, target, *expected).Scan(&oldV, &oldConfidence, &oldClinical)
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrConflict
		}
		if err != nil {
			return err
		}
		newConfidence := oldConfidence
		var newClinical string
		action = "clinical_hypothesis.updated"
		switch kind {
		case "strengthen_hypothesis":
			switch oldConfidence {
			case "red":
				newConfidence = "yellow"
			case "yellow":
				newConfidence = "green"
			default:
				return domainerrors.ErrConflict
			}
			newClinical = "active"
		case "weaken_hypothesis":
			switch oldConfidence {
			case "green":
				newConfidence = "yellow"
			case "yellow":
				newConfidence = "red"
			default:
				return domainerrors.ErrConflict
			}
			newClinical = "weakened"
		default:
			newClinical = "retired"
			action = "clinical_hypothesis.retired"
		}
		_, err = tx.Exec(ctx, `UPDATE clinical_hypotheses SET confidence_level=$4,clinical_status=$5,version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3`, in.TenantID, client, target, newConfidence, newClinical)
		if err != nil {
			return err
		}
		fromVersion = &oldV
		toVersion = oldV + 1
		fromStatus = oldClinical + ":" + oldConfidence
		toStatus = newClinical + ":" + newConfidence
		entity = "hypothesis"
	default:
		strategyResult, recognized, strategyErr := r.applyStrategyOperation(ctx, tx, in, client, runID, kind, target, expected, payload, now)
		if strategyErr != nil {
			return strategyErr
		}
		if !recognized {
			return domainerrors.NewValidation("unsupported longitudinal operation")
		}
		entity, action, toStatus, fromStatus, fromVersion, toVersion = strategyResult.entity, strategyResult.action, strategyResult.toStatus, strategyResult.fromStatus, strategyResult.fromVersion, strategyResult.toVersion
	}
	if _, err := tx.Exec(ctx, `INSERT INTO clinical_longitudinal_transitions(tenant_id,client_id,diff_id,operation_id,entity_type,entity_id,action,from_version,to_version,from_status,to_status,actor_user_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, in.TenantID, client, in.DiffID, opID, entity, target, kind, fromVersion, toVersion, nilIfEmpty(fromStatus), nilIfEmpty(toStatus), in.ActorID); err != nil {
		return err
	}
	return insertAuditEvent(ctx, tx, in.TenantID, in.ActorID, action, "clinical_"+entity, target, map[string]any{"diff_id": in.DiffID, "operation_id": opID, "from_version": fromVersion, "to_version": toVersion, "status": toStatus})
}

func validateActiveEvidence(ctx context.Context, tx pgx.Tx, t, c uuid.UUID, ids []uuid.UUID) error {
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clinical_evidence WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND status='active')`, t, c, id).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return domainerrors.NewValidation("operation references missing or inactive evidence")
		}
	}
	return nil
}
func nilIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

var _ = fmt.Sprintf
