package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

type strategyMergeResult struct {
	entity, action, fromStatus, toStatus string
	fromVersion                          *int
	toVersion                            int
}

func (r *ClinicalLongitudinalRepository) applyStrategyOperation(ctx context.Context, tx pgx.Tx, in longitudinal.MergeInput, client uuid.UUID, runID *uuid.UUID, kind string, target uuid.UUID, expected *int, payload []byte, now time.Time) (strategyMergeResult, bool, error) {
	result := strategyMergeResult{toVersion: 1}
	requireExpected := func() error {
		if expected == nil {
			return domainerrors.ErrConflict
		}
		return nil
	}
	conflictNoRows := func(err error) error {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainerrors.ErrConflict
		}
		return err
	}
	insertIDs := func(table, ownerColumn, linkedColumn string, owner uuid.UUID, ids []uuid.UUID) error {
		for _, id := range ids {
			if _, err := tx.Exec(ctx, `INSERT INTO `+table+`(tenant_id,client_id,`+ownerColumn+`,`+linkedColumn+`) VALUES($1,$2,$3,$4)`, in.TenantID, client, owner, id); err != nil {
				return err
			}
		}
		return nil
	}
	validateApproved := func(table string, id uuid.UUID) error {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND approval_status='approved')`, in.TenantID, client, id).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return domainerrors.NewValidation("strategy reference must be approved and client-local")
		}
		return nil
	}
	validateIDs := func(table string, ids []uuid.UUID) error {
		for _, id := range ids {
			if err := validateApproved(table, id); err != nil {
				return err
			}
		}
		return nil
	}
	validateSameProcess := func(table string, ids []uuid.UUID, processID uuid.UUID) error {
		if len(ids) == 0 {
			return nil
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE tenant_id=$1 AND client_id=$2 AND id=ANY($3::uuid[]) AND process_id=$4`, in.TenantID, client, ids, processID).Scan(&count); err != nil {
			return err
		}
		if count != len(ids) {
			return domainerrors.NewValidation("strategy references must remain in the same clinical process")
		}
		return nil
	}
	entityProcess := func(table string, id uuid.UUID) (uuid.UUID, error) {
		var processID uuid.UUID
		err := tx.QueryRow(ctx, `SELECT process_id FROM `+table+` WHERE tenant_id=$1 AND client_id=$2 AND id=$3`, in.TenantID, client, id).Scan(&processID)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, domainerrors.NewValidation("strategy entity is not client-local")
		}
		return processID, err
	}
	validateIndicatorProcess := func(ids []uuid.UUID, processID uuid.UUID) error {
		if len(ids) == 0 {
			return nil
		}
		var count int
		err := tx.QueryRow(ctx, `SELECT count(*) FROM clinical_goal_indicators i JOIN clinical_goals g ON g.tenant_id=i.tenant_id AND g.id=i.goal_id AND g.client_id=i.client_id WHERE i.tenant_id=$1 AND i.client_id=$2 AND i.id=ANY($3::uuid[]) AND g.process_id=$4`, in.TenantID, client, ids, processID).Scan(&count)
		if err != nil {
			return err
		}
		if count != len(ids) {
			return domainerrors.NewValidation("phase indicators must belong to goals in the same process")
		}
		return nil
	}
	validateGoalTarget := func(goalID, targetID uuid.UUID) error {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clinical_goal_targets WHERE tenant_id=$1 AND client_id=$2 AND goal_id=$3 AND target_id=$4)`, in.TenantID, client, goalID, targetID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return domainerrors.NewValidation("therapeutic rationale target must be linked to its goal")
		}
		return nil
	}
	validateOwnerLinks := func(table, ownerColumn, linkedColumn string, owner uuid.UUID, ids []uuid.UUID) error {
		if len(ids) == 0 {
			return nil
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE tenant_id=$1 AND client_id=$2 AND `+ownerColumn+`=$3 AND `+linkedColumn+`=ANY($4::uuid[])`, in.TenantID, client, owner, ids).Scan(&count); err != nil {
			return err
		}
		if count != len(ids) {
			return domainerrors.NewValidation("phase references must belong to the selected GIRA")
		}
		return nil
	}
	validateIndicatorsForGoals := func(indicatorIDs, goalIDs []uuid.UUID, processID uuid.UUID) error {
		if err := validateIndicatorProcess(indicatorIDs, processID); err != nil {
			return err
		}
		if len(indicatorIDs) == 0 {
			return nil
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM clinical_goal_indicators WHERE tenant_id=$1 AND client_id=$2 AND id=ANY($3::uuid[]) AND goal_id=ANY($4::uuid[])`, in.TenantID, client, indicatorIDs, goalIDs).Scan(&count); err != nil {
			return err
		}
		if count != len(indicatorIDs) {
			return domainerrors.NewValidation("phase indicator must belong to a goal linked to that phase")
		}
		return nil
	}
	validateGrounding := func(evidence, hypotheses, events []uuid.UUID) error {
		if len(evidence)+len(hypotheses)+len(events) == 0 {
			return domainerrors.NewValidation("target requires clinical grounding")
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, evidence); err != nil {
			return err
		}
		if err := validateIDs("clinical_hypotheses", hypotheses); err != nil {
			return err
		}
		return validateIDs("clinical_events", events)
	}
	setVersion := func(old int, oldStatus, newStatus string) {
		result.fromVersion = &old
		result.toVersion = old + 1
		result.fromStatus = oldStatus
		result.toStatus = newStatus
	}

	switch kind {
	case "create_target":
		var p longitudinal.CreateTargetProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if expected != nil {
			return result, true, domainerrors.ErrConflict
		}
		if err := validateApproved("clinical_processes", p.ProcessID); err != nil {
			return result, true, err
		}
		if err := validateGrounding(p.EvidenceIDs, p.HypothesisIDs, p.EventIDs); err != nil {
			return result, true, err
		}
		_, err := tx.Exec(ctx, `INSERT INTO clinical_targets(id,tenant_id,client_id,process_id,title,description,target_type,approval_status,clinical_status,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,$4,$5,$6,$7,'approved','active',$8,$9,$8,$10)`, target, in.TenantID, client, p.ProcessID, p.Title, p.Description, p.TargetType, in.ActorID, runID, now)
		if err != nil {
			return result, true, err
		}
		if err := insertIDs("clinical_target_evidence", "target_id", "evidence_id", target, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("clinical_target_hypotheses", "target_id", "hypothesis_id", target, p.HypothesisIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("clinical_target_events", "target_id", "event_id", target, p.EventIDs); err != nil {
			return result, true, err
		}
		result.entity, result.action, result.toStatus = "target", "clinical_target.created", "active"
	case "update_target":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.UpdateTargetProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if err := validateGrounding(p.EvidenceIDs, p.HypothesisIDs, p.EventIDs); err != nil {
			return result, true, err
		}
		var old int
		var status string
		err := tx.QueryRow(ctx, `UPDATE clinical_targets SET title=$5,description=$6,target_type=$7,version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' RETURNING version-1,clinical_status`, in.TenantID, client, target, *expected, p.Title, p.Description, p.TargetType).Scan(&old, &status)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM clinical_target_evidence WHERE tenant_id=$1 AND target_id=$2`, in.TenantID, target); err != nil {
			return result, true, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM clinical_target_hypotheses WHERE tenant_id=$1 AND target_id=$2`, in.TenantID, target); err != nil {
			return result, true, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM clinical_target_events WHERE tenant_id=$1 AND target_id=$2`, in.TenantID, target); err != nil {
			return result, true, err
		}
		if err := insertIDs("clinical_target_evidence", "target_id", "evidence_id", target, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("clinical_target_hypotheses", "target_id", "hypothesis_id", target, p.HypothesisIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("clinical_target_events", "target_id", "event_id", target, p.EventIDs); err != nil {
			return result, true, err
		}
		result.entity, result.action = "target", "clinical_target.updated"
		setVersion(old, status, status)
	case "resolve_target", "retire_target":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.TransitionTargetProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		status := "resolved"
		action := "clinical_target.resolved"
		if kind == "retire_target" {
			status = "retired"
			action = "clinical_target.retired"
		}
		var old int
		var previous string
		err := tx.QueryRow(ctx, `WITH prior AS (SELECT version,clinical_status FROM clinical_targets WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' FOR UPDATE), changed AS (UPDATE clinical_targets t SET clinical_status=$5,version=t.version+1,updated_at=NOW() FROM prior WHERE t.tenant_id=$1 AND t.id=$3 RETURNING prior.version,prior.clinical_status) SELECT version,clinical_status FROM changed`, in.TenantID, client, target, *expected, status).Scan(&old, &previous)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		result.entity, result.action = "target", action
		setVersion(old, previous, status)
	case "create_goal":
		var p longitudinal.CreateGoalProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if expected != nil {
			return result, true, domainerrors.ErrConflict
		}
		if err := validateApproved("clinical_processes", p.ProcessID); err != nil {
			return result, true, err
		}
		if err := validateIDs("clinical_targets", p.TargetIDs); err != nil {
			return result, true, err
		}
		if err := validateSameProcess("clinical_targets", p.TargetIDs, p.ProcessID); err != nil {
			return result, true, err
		}
		_, err := tx.Exec(ctx, `INSERT INTO clinical_goals(id,tenant_id,client_id,process_id,title,description,goal_type,priority,approval_status,clinical_status,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'approved','planned',$9,$10,$9,$11)`, target, in.TenantID, client, p.ProcessID, p.Title, p.Description, p.GoalType, p.Priority, in.ActorID, runID, now)
		if err != nil {
			return result, true, err
		}
		if err := insertIDs("clinical_goal_targets", "goal_id", "target_id", target, p.TargetIDs); err != nil {
			return result, true, err
		}
		result.entity, result.action, result.toStatus = "goal", "clinical_goal.created", "planned"
	case "update_goal":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.UpdateGoalProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if err := validateIDs("clinical_targets", p.TargetIDs); err != nil {
			return result, true, err
		}
		goalProcess, err := entityProcess("clinical_goals", target)
		if err != nil {
			return result, true, err
		}
		if err := validateSameProcess("clinical_targets", p.TargetIDs, goalProcess); err != nil {
			return result, true, err
		}
		var old int
		var status string
		err = tx.QueryRow(ctx, `UPDATE clinical_goals SET title=$5,description=$6,goal_type=$7,priority=$8,version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' RETURNING version-1,clinical_status`, in.TenantID, client, target, *expected, p.Title, p.Description, p.GoalType, p.Priority).Scan(&old, &status)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM clinical_goal_targets WHERE tenant_id=$1 AND goal_id=$2`, in.TenantID, target); err != nil {
			return result, true, err
		}
		if err := insertIDs("clinical_goal_targets", "goal_id", "target_id", target, p.TargetIDs); err != nil {
			return result, true, err
		}
		result.entity, result.action = "goal", "clinical_goal.updated"
		setVersion(old, status, status)
	case "activate_goal", "pause_goal", "achieve_goal", "abandon_goal":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.TransitionGoalProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		status := map[string]string{"activate_goal": "active", "pause_goal": "paused", "achieve_goal": "achieved", "abandon_goal": "abandoned"}[kind]
		var old int
		var previous string
		err := tx.QueryRow(ctx, `WITH prior AS (SELECT version,clinical_status FROM clinical_goals WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' FOR UPDATE), changed AS (UPDATE clinical_goals g SET clinical_status=$5,activated_at=CASE WHEN $5='active' THEN COALESCE(g.activated_at,$6) ELSE g.activated_at END,achieved_at=CASE WHEN $5='achieved' THEN $6 ELSE g.achieved_at END,version=g.version+1,updated_at=NOW() FROM prior WHERE g.tenant_id=$1 AND g.id=$3 RETURNING prior.version,prior.clinical_status) SELECT version,clinical_status FROM changed`, in.TenantID, client, target, *expected, status, now).Scan(&old, &previous)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		result.entity, result.action = "goal", "clinical_goal."+status
		setVersion(old, previous, status)
	case "create_goal_indicator":
		var p longitudinal.CreateGoalIndicatorProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if expected != nil {
			return result, true, domainerrors.ErrConflict
		}
		if err := validateApproved("clinical_goals", p.GoalID); err != nil {
			return result, true, err
		}
		_, err := tx.Exec(ctx, `INSERT INTO clinical_goal_indicators(id,tenant_id,client_id,goal_id,description,indicator_type,measurement_method,baseline,target_value,status,created_by_user_id,created_from_ai_run_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'active',$10,$11)`, target, in.TenantID, client, p.GoalID, p.Description, p.IndicatorType, p.MeasurementMethod, p.Baseline, p.TargetValue, in.ActorID, runID)
		if err != nil {
			return result, true, err
		}
		result.entity, result.action, result.toStatus = "goal_indicator", "goal_indicator.created", "active"
	case "update_goal_indicator":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.UpdateGoalIndicatorProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		var old int
		var previous string
		err := tx.QueryRow(ctx, `WITH prior AS (SELECT version,status FROM clinical_goal_indicators WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 FOR UPDATE), changed AS (UPDATE clinical_goal_indicators i SET description=$5,indicator_type=$6,measurement_method=$7,baseline=$8,target_value=$9,status=$10,version=i.version+1,updated_at=NOW() FROM prior WHERE i.tenant_id=$1 AND i.id=$3 RETURNING prior.version,prior.status) SELECT version,status FROM changed`, in.TenantID, client, target, *expected, p.Description, p.IndicatorType, p.MeasurementMethod, p.Baseline, p.TargetValue, p.Status).Scan(&old, &previous)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		result.entity, result.action = "goal_indicator", "goal_indicator.updated"
		setVersion(old, previous, p.Status)
	case "link_indicator_evidence", "link_indicator_event":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.LinkIndicatorSourceProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if p.IndicatorID != target {
			return result, true, domainerrors.NewValidation("target must identify indicator")
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		table, column := "clinical_indicator_evidence", "evidence_id"
		if kind == "link_indicator_event" {
			table, column = "clinical_indicator_events", "event_id"
			if err := validateApproved("clinical_events", p.SourceID); err != nil {
				return result, true, err
			}
		} else if err := validateActiveEvidence(ctx, tx, in.TenantID, client, []uuid.UUID{p.SourceID}); err != nil {
			return result, true, err
		}
		_, err := tx.Exec(ctx, `INSERT INTO `+table+`(tenant_id,client_id,indicator_id,`+column+`,relation_type) VALUES($1,$2,$3,$4,$5)`, in.TenantID, client, target, p.SourceID, p.RelationType)
		if err != nil {
			return result, true, err
		}
		var old int
		var status string
		err = tx.QueryRow(ctx, `UPDATE clinical_goal_indicators SET version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 RETURNING version-1,status`, in.TenantID, client, target, *expected).Scan(&old, &status)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		result.entity, result.action = "goal_indicator", "goal_indicator.progress_linked"
		setVersion(old, status, status)
	case "create_therapeutic_rationale":
		var p longitudinal.CreateTherapeuticRationaleProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if expected != nil {
			return result, true, domainerrors.ErrConflict
		}
		if p.GroundingStatus != "grounded" {
			return result, true, domainerrors.NewValidation("only grounded rationale may be merged")
		}
		for table, id := range map[string]uuid.UUID{"clinical_processes": p.ProcessID, "clinical_targets": p.TargetID, "clinical_goals": p.GoalID} {
			if err := validateApproved(table, id); err != nil {
				return result, true, err
			}
		}
		if err := validateSameProcess("clinical_targets", []uuid.UUID{p.TargetID}, p.ProcessID); err != nil {
			return result, true, err
		}
		if err := validateSameProcess("clinical_goals", []uuid.UUID{p.GoalID}, p.ProcessID); err != nil {
			return result, true, err
		}
		if err := validateGoalTarget(p.GoalID, p.TargetID); err != nil {
			return result, true, err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		if err := validateIDs("clinical_hypotheses", p.HypothesisIDs); err != nil {
			return result, true, err
		}
		_, err := tx.Exec(ctx, `INSERT INTO therapeutic_rationales(id,tenant_id,client_id,process_id,target_id,goal_id,approach_slug,approach_version,technique_slug,technique_version,rationale,expected_effect,grounding_status,approval_status,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'approved',$14,$15,$14,$16)`, target, in.TenantID, client, p.ProcessID, p.TargetID, p.GoalID, p.ApproachSlug, p.ApproachVersion, p.TechniqueSlug, p.TechniqueVersion, p.Rationale, p.ExpectedEffect, p.GroundingStatus, in.ActorID, runID, now)
		if err != nil {
			return result, true, err
		}
		if err := insertIDs("therapeutic_rationale_evidence", "rationale_id", "evidence_id", target, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("therapeutic_rationale_hypotheses", "rationale_id", "hypothesis_id", target, p.HypothesisIDs); err != nil {
			return result, true, err
		}
		result.entity, result.action, result.toStatus = "therapeutic_rationale", "therapeutic_rationale.created", "approved"
	case "update_therapeutic_rationale":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.UpdateTherapeuticRationaleProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if p.GroundingStatus != "grounded" {
			return result, true, domainerrors.NewValidation("only grounded rationale may be merged")
		}
		for table, id := range map[string]uuid.UUID{"clinical_processes": p.ProcessID, "clinical_targets": p.TargetID, "clinical_goals": p.GoalID} {
			if err := validateApproved(table, id); err != nil {
				return result, true, err
			}
		}
		if err := validateSameProcess("clinical_targets", []uuid.UUID{p.TargetID}, p.ProcessID); err != nil {
			return result, true, err
		}
		if err := validateSameProcess("clinical_goals", []uuid.UUID{p.GoalID}, p.ProcessID); err != nil {
			return result, true, err
		}
		if err := validateGoalTarget(p.GoalID, p.TargetID); err != nil {
			return result, true, err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		if err := validateIDs("clinical_hypotheses", p.HypothesisIDs); err != nil {
			return result, true, err
		}
		var old int
		err := tx.QueryRow(ctx, `UPDATE therapeutic_rationales SET process_id=$5,target_id=$6,goal_id=$7,approach_slug=$8,approach_version=$9,technique_slug=$10,technique_version=$11,rationale=$12,expected_effect=$13,grounding_status=$14,version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 AND approval_status='approved' RETURNING version-1`, in.TenantID, client, target, *expected, p.ProcessID, p.TargetID, p.GoalID, p.ApproachSlug, p.ApproachVersion, p.TechniqueSlug, p.TechniqueVersion, p.Rationale, p.ExpectedEffect, p.GroundingStatus).Scan(&old)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM therapeutic_rationale_evidence WHERE tenant_id=$1 AND rationale_id=$2`, in.TenantID, target); err != nil {
			return result, true, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM therapeutic_rationale_hypotheses WHERE tenant_id=$1 AND rationale_id=$2`, in.TenantID, target); err != nil {
			return result, true, err
		}
		if err := insertIDs("therapeutic_rationale_evidence", "rationale_id", "evidence_id", target, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("therapeutic_rationale_hypotheses", "rationale_id", "hypothesis_id", target, p.HypothesisIDs); err != nil {
			return result, true, err
		}
		result.entity, result.action = "therapeutic_rationale", "therapeutic_rationale.updated"
		setVersion(old, "approved", "approved")
	case "create_gira":
		var p longitudinal.CreateGIRAProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if expected != nil {
			return result, true, domainerrors.ErrConflict
		}
		if err := validateApproved("clinical_processes", p.ProcessID); err != nil {
			return result, true, err
		}
		if err := validateIDs("clinical_targets", p.TargetIDs); err != nil {
			return result, true, err
		}
		if err := validateIDs("clinical_goals", p.GoalIDs); err != nil {
			return result, true, err
		}
		if err := validateIDs("therapeutic_rationales", p.RationaleIDs); err != nil {
			return result, true, err
		}
		if err := validateSameProcess("clinical_targets", p.TargetIDs, p.ProcessID); err != nil {
			return result, true, err
		}
		if err := validateSameProcess("clinical_goals", p.GoalIDs, p.ProcessID); err != nil {
			return result, true, err
		}
		if err := validateSameProcess("therapeutic_rationales", p.RationaleIDs, p.ProcessID); err != nil {
			return result, true, err
		}
		if len(p.RationaleIDs) > 0 {
			var chainCount int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM therapeutic_rationales WHERE tenant_id=$1 AND client_id=$2 AND id=ANY($3::uuid[]) AND target_id=ANY($4::uuid[]) AND goal_id=ANY($5::uuid[])`, in.TenantID, client, p.RationaleIDs, p.TargetIDs, p.GoalIDs).Scan(&chainCount); err != nil {
				return result, true, err
			}
			if chainCount != len(p.RationaleIDs) {
				return result, true, domainerrors.NewValidation("GIRA rationales must connect included targets and goals")
			}
		}
		if p.SupersedesGIRAID == nil {
			if p.GIRAVersion != 1 {
				return result, true, domainerrors.NewValidation("initial GIRA version must be 1")
			}
		} else {
			var oldVersion int
			var oldProcess uuid.UUID
			err := tx.QueryRow(ctx, `SELECT gira_version,process_id FROM giras WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND approval_status='approved'`, in.TenantID, client, *p.SupersedesGIRAID).Scan(&oldVersion, &oldProcess)
			if errors.Is(err, pgx.ErrNoRows) {
				return result, true, domainerrors.NewValidation("superseded GIRA must be approved and client-local")
			}
			if err != nil {
				return result, true, err
			}
			if oldProcess != p.ProcessID || p.GIRAVersion != oldVersion+1 {
				return result, true, domainerrors.NewValidation("GIRA supersession must remain in process and increment one version")
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO giras(id,tenant_id,client_id,process_id,gira_version,title,summary,approval_status,clinical_status,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at,supersedes_gira_id) VALUES($1,$2,$3,$4,$5,$6,$7,'approved','planned',$8,$9,$8,$10,$11)`, target, in.TenantID, client, p.ProcessID, p.GIRAVersion, p.Title, p.Summary, in.ActorID, runID, now, p.SupersedesGIRAID)
		if err != nil {
			return result, true, err
		}
		if err := insertIDs("gira_targets", "gira_id", "target_id", target, p.TargetIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("gira_goals", "gira_id", "goal_id", target, p.GoalIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("gira_rationales", "gira_id", "rationale_id", target, p.RationaleIDs); err != nil {
			return result, true, err
		}
		result.entity, result.action, result.toStatus = "gira", "gira.created", "planned"
	case "supersede_gira", "activate_gira", "pause_gira", "complete_gira":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.TransitionGIRAProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		status := map[string]string{"supersede_gira": "superseded", "activate_gira": "active", "pause_gira": "paused", "complete_gira": "completed"}[kind]
		var old int
		var previous string
		err := tx.QueryRow(ctx, `WITH prior AS (SELECT entity_version,clinical_status FROM giras WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND entity_version=$4 AND approval_status='approved' FOR UPDATE), changed AS (UPDATE giras g SET clinical_status=$5,activated_at=CASE WHEN $5='active' THEN COALESCE(g.activated_at,$6) ELSE g.activated_at END,completed_at=CASE WHEN $5='completed' THEN $6 ELSE g.completed_at END,entity_version=g.entity_version+1,updated_at=NOW() FROM prior WHERE g.tenant_id=$1 AND g.id=$3 RETURNING prior.entity_version,prior.clinical_status) SELECT entity_version,clinical_status FROM changed`, in.TenantID, client, target, *expected, status, now).Scan(&old, &previous)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		result.entity, result.action = "gira", "gira."+status
		setVersion(old, previous, status)
	case "create_gira_phase":
		var p longitudinal.CreateGIRAPhaseProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if expected != nil {
			return result, true, domainerrors.ErrConflict
		}
		if err := validateApproved("giras", p.GIRAID); err != nil {
			return result, true, err
		}
		if err := validateIDs("clinical_goals", p.GoalIDs); err != nil {
			return result, true, err
		}
		if err := validateIDs("therapeutic_rationales", p.RationaleIDs); err != nil {
			return result, true, err
		}
		giraProcess, err := entityProcess("giras", p.GIRAID)
		if err != nil {
			return result, true, err
		}
		if err := validateSameProcess("clinical_goals", p.GoalIDs, giraProcess); err != nil {
			return result, true, err
		}
		if err := validateSameProcess("therapeutic_rationales", p.RationaleIDs, giraProcess); err != nil {
			return result, true, err
		}
		if err := validateOwnerLinks("gira_goals", "gira_id", "goal_id", p.GIRAID, p.GoalIDs); err != nil {
			return result, true, err
		}
		if err := validateOwnerLinks("gira_rationales", "gira_id", "rationale_id", p.GIRAID, p.RationaleIDs); err != nil {
			return result, true, err
		}
		if err := validateIndicatorsForGoals(p.IndicatorIDs, p.GoalIDs, giraProcess); err != nil {
			return result, true, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO gira_phases(id,tenant_id,client_id,gira_id,position,title,description,clinical_status,entry_criteria,exit_criteria,created_by_user_id,created_from_ai_run_id) VALUES($1,$2,$3,$4,$5,$6,$7,'planned',$8,$9,$10,$11)`, target, in.TenantID, client, p.GIRAID, p.Position, p.Title, p.Description, p.EntryCriteria, p.ExitCriteria, in.ActorID, runID)
		if err != nil {
			return result, true, err
		}
		if err := insertIDs("gira_phase_goals", "phase_id", "goal_id", target, p.GoalIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("gira_phase_rationales", "phase_id", "rationale_id", target, p.RationaleIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("gira_phase_indicators", "phase_id", "indicator_id", target, p.IndicatorIDs); err != nil {
			return result, true, err
		}
		result.entity, result.action, result.toStatus = "gira_phase", "gira_phase.created", "planned"
	case "update_gira_phase":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.UpdateGIRAPhaseProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if err := validateIDs("clinical_goals", p.GoalIDs); err != nil {
			return result, true, err
		}
		if err := validateIDs("therapeutic_rationales", p.RationaleIDs); err != nil {
			return result, true, err
		}
		var phaseProcess uuid.UUID
		err := tx.QueryRow(ctx, `SELECT g.process_id FROM gira_phases p JOIN giras g ON g.tenant_id=p.tenant_id AND g.id=p.gira_id AND g.client_id=p.client_id WHERE p.tenant_id=$1 AND p.client_id=$2 AND p.id=$3`, in.TenantID, client, target).Scan(&phaseProcess)
		if errors.Is(err, pgx.ErrNoRows) {
			return result, true, domainerrors.NewValidation("phase is not client-local")
		}
		if err != nil {
			return result, true, err
		}
		if err := validateSameProcess("clinical_goals", p.GoalIDs, phaseProcess); err != nil {
			return result, true, err
		}
		if err := validateSameProcess("therapeutic_rationales", p.RationaleIDs, phaseProcess); err != nil {
			return result, true, err
		}
		var phaseGIRA uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT gira_id FROM gira_phases WHERE tenant_id=$1 AND client_id=$2 AND id=$3`, in.TenantID, client, target).Scan(&phaseGIRA); err != nil {
			return result, true, err
		}
		if err := validateOwnerLinks("gira_goals", "gira_id", "goal_id", phaseGIRA, p.GoalIDs); err != nil {
			return result, true, err
		}
		if err := validateOwnerLinks("gira_rationales", "gira_id", "rationale_id", phaseGIRA, p.RationaleIDs); err != nil {
			return result, true, err
		}
		if err := validateIndicatorsForGoals(p.IndicatorIDs, p.GoalIDs, phaseProcess); err != nil {
			return result, true, err
		}
		var old int
		var status string
		err = tx.QueryRow(ctx, `UPDATE gira_phases SET position=$5,title=$6,description=$7,entry_criteria=$8,exit_criteria=$9,version=version+1,updated_at=NOW() WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 RETURNING version-1,clinical_status`, in.TenantID, client, target, *expected, p.Position, p.Title, p.Description, p.EntryCriteria, p.ExitCriteria).Scan(&old, &status)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM gira_phase_goals WHERE tenant_id=$1 AND phase_id=$2`, in.TenantID, target); err != nil {
			return result, true, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM gira_phase_rationales WHERE tenant_id=$1 AND phase_id=$2`, in.TenantID, target); err != nil {
			return result, true, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM gira_phase_indicators WHERE tenant_id=$1 AND phase_id=$2`, in.TenantID, target); err != nil {
			return result, true, err
		}
		if err := insertIDs("gira_phase_goals", "phase_id", "goal_id", target, p.GoalIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("gira_phase_rationales", "phase_id", "rationale_id", target, p.RationaleIDs); err != nil {
			return result, true, err
		}
		if err := insertIDs("gira_phase_indicators", "phase_id", "indicator_id", target, p.IndicatorIDs); err != nil {
			return result, true, err
		}
		result.entity, result.action = "gira_phase", "gira_phase.updated"
		setVersion(old, status, status)
	case "activate_gira_phase", "complete_gira_phase", "pause_gira_phase":
		if err := requireExpected(); err != nil {
			return result, true, err
		}
		var p longitudinal.TransitionGIRAPhaseProposal
		if err := json.Unmarshal(payload, &p); err != nil {
			return result, true, err
		}
		if err := validateActiveEvidence(ctx, tx, in.TenantID, client, p.EvidenceIDs); err != nil {
			return result, true, err
		}
		status := map[string]string{"activate_gira_phase": "active", "complete_gira_phase": "completed", "pause_gira_phase": "paused"}[kind]
		var old int
		var previous string
		err := tx.QueryRow(ctx, `WITH prior AS (SELECT version,clinical_status FROM gira_phases WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND version=$4 FOR UPDATE), changed AS (UPDATE gira_phases p SET clinical_status=$5,activated_at=CASE WHEN $5='active' THEN COALESCE(p.activated_at,$6) ELSE p.activated_at END,completed_at=CASE WHEN $5='completed' THEN $6 ELSE p.completed_at END,version=p.version+1,updated_at=NOW() FROM prior WHERE p.tenant_id=$1 AND p.id=$3 RETURNING prior.version,prior.clinical_status) SELECT version,clinical_status FROM changed`, in.TenantID, client, target, *expected, status, now).Scan(&old, &previous)
		if err := conflictNoRows(err); err != nil {
			return result, true, err
		}
		result.entity, result.action = "gira_phase", "gira_phase."+status
		setVersion(old, previous, status)
	default:
		return result, false, nil
	}
	return result, true, nil
}
