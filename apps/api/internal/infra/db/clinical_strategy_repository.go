package db

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

const targetSelect = `SELECT id,tenant_id,client_id,process_id,title,description,target_type,approval_status,clinical_status,version,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at,created_at,updated_at FROM clinical_targets`
const goalSelect = `SELECT id,tenant_id,client_id,process_id,title,description,goal_type,priority,approval_status,clinical_status,version,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at,activated_at,achieved_at,created_at,updated_at FROM clinical_goals`
const indicatorSelect = `SELECT id,tenant_id,client_id,goal_id,description,indicator_type,measurement_method,baseline,target_value,status,version,created_by_user_id,created_from_ai_run_id,created_at,updated_at FROM clinical_goal_indicators`
const rationaleSelect = `SELECT r.id,r.tenant_id,r.client_id,r.process_id,r.target_id,r.goal_id,r.approach_slug,r.approach_version,r.technique_slug,r.technique_version,r.rationale,r.expected_effect,r.grounding_status,r.approval_status,r.version,r.created_by_user_id,r.created_from_ai_run_id,r.approved_by_user_id,r.approved_at,r.created_at,r.updated_at FROM therapeutic_rationales r`
const giraSelect = `SELECT id,tenant_id,client_id,process_id,gira_version,title,summary,approval_status,clinical_status,entity_version,created_by_user_id,created_from_ai_run_id,approved_by_user_id,approved_at,activated_at,completed_at,supersedes_gira_id,created_at,updated_at FROM giras`
const phaseSelect = `SELECT id,tenant_id,client_id,gira_id,position,title,description,clinical_status,entry_criteria,exit_criteria,version,created_by_user_id,created_from_ai_run_id,activated_at,completed_at,created_at,updated_at FROM gira_phases`

func scanTarget(row pgx.Row) (x longitudinal.Target, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.ProcessID, &x.Title, &x.Description, &x.TargetType, &x.ApprovalStatus, &x.ClinicalStatus, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.ApprovedByUserID, &x.ApprovedAt, &x.CreatedAt, &x.UpdatedAt)
	x.EvidenceIDs = []uuid.UUID{}
	x.HypothesisIDs = []uuid.UUID{}
	x.EventIDs = []uuid.UUID{}
	return
}
func scanGoal(row pgx.Row) (x longitudinal.Goal, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.ProcessID, &x.Title, &x.Description, &x.GoalType, &x.Priority, &x.ApprovalStatus, &x.ClinicalStatus, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.ApprovedByUserID, &x.ApprovedAt, &x.ActivatedAt, &x.AchievedAt, &x.CreatedAt, &x.UpdatedAt)
	x.TargetIDs = []uuid.UUID{}
	x.Indicators = []longitudinal.GoalIndicator{}
	return
}
func scanIndicator(row pgx.Row) (x longitudinal.GoalIndicator, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.GoalID, &x.Description, &x.IndicatorType, &x.MeasurementMethod, &x.Baseline, &x.TargetValue, &x.Status, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.CreatedAt, &x.UpdatedAt)
	x.Links = []longitudinal.GoalIndicatorLink{}
	return
}
func scanRationale(row pgx.Row) (x longitudinal.TherapeuticRationale, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.ProcessID, &x.TargetID, &x.GoalID, &x.ApproachSlug, &x.ApproachVersion, &x.TechniqueSlug, &x.TechniqueVersion, &x.Rationale, &x.ExpectedEffect, &x.GroundingStatus, &x.ApprovalStatus, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.ApprovedByUserID, &x.ApprovedAt, &x.CreatedAt, &x.UpdatedAt)
	x.EvidenceIDs = []uuid.UUID{}
	x.HypothesisIDs = []uuid.UUID{}
	return
}
func scanGIRA(row pgx.Row) (x longitudinal.GIRA, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.ProcessID, &x.GIRAVersion, &x.Title, &x.Summary, &x.ApprovalStatus, &x.ClinicalStatus, &x.EntityVersion, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.ApprovedByUserID, &x.ApprovedAt, &x.ActivatedAt, &x.CompletedAt, &x.SupersedesGIRAID, &x.CreatedAt, &x.UpdatedAt)
	x.TargetIDs = []uuid.UUID{}
	x.GoalIDs = []uuid.UUID{}
	x.Rationales = []longitudinal.TherapeuticRationale{}
	x.Phases = []longitudinal.GIRAPhase{}
	return
}
func scanPhase(row pgx.Row) (x longitudinal.GIRAPhase, err error) {
	err = row.Scan(&x.ID, &x.TenantID, &x.ClientID, &x.GIRAID, &x.Position, &x.Title, &x.Description, &x.ClinicalStatus, &x.EntryCriteria, &x.ExitCriteria, &x.Version, &x.CreatedByUserID, &x.CreatedFromAIRunID, &x.ActivatedAt, &x.CompletedAt, &x.CreatedAt, &x.UpdatedAt)
	x.GoalIDs = []uuid.UUID{}
	x.RationaleIDs = []uuid.UUID{}
	x.IndicatorIDs = []uuid.UUID{}
	return
}

func (r *ClinicalLongitudinalRepository) ListTargets(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Target, error) {
	return r.listTargets(ctx, t, c, 0, 0, nil)
}
func (r *ClinicalLongitudinalRepository) ListTargetsPage(ctx context.Context, t, c uuid.UUID, limit, offset int) ([]longitudinal.Target, error) {
	return r.listTargets(ctx, t, c, limit, offset, nil)
}
func (r *ClinicalLongitudinalRepository) listTargets(ctx context.Context, t, c uuid.UUID, limit, offset int, processIDs []uuid.UUID) ([]longitudinal.Target, error) {
	query := targetSelect + ` WHERE tenant_id=$1 AND client_id=$2`
	args := []any{t, c}
	if processIDs != nil {
		query += ` AND process_id=ANY($3::uuid[])`
		args = append(args, processIDs)
	}
	query += ` ORDER BY process_id,CASE clinical_status WHEN 'active' THEN 1 WHEN 'monitoring' THEN 2 WHEN 'resolved' THEN 3 ELSE 4 END,updated_at DESC,id`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := []longitudinal.Target{}
	for rows.Next() {
		x, e := scanTarget(rows)
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
	ids := make([]uuid.UUID, 0, len(out))
	for i := range out {
		byID[out[i].ID] = i
		ids = append(ids, out[i].ID)
	}
	links, err := r.pool.Query(ctx, `
		SELECT l.target_id,'evidence',l.evidence_id FROM clinical_target_evidence l JOIN clinical_targets t ON t.tenant_id=l.tenant_id AND t.id=l.target_id AND t.client_id=l.client_id WHERE t.tenant_id=$1 AND t.client_id=$2 AND t.id=ANY($3::uuid[])
		UNION ALL SELECT l.target_id,'hypothesis',l.hypothesis_id FROM clinical_target_hypotheses l JOIN clinical_targets t ON t.tenant_id=l.tenant_id AND t.id=l.target_id AND t.client_id=l.client_id WHERE t.tenant_id=$1 AND t.client_id=$2 AND t.id=ANY($3::uuid[])
		UNION ALL SELECT l.target_id,'event',l.event_id FROM clinical_target_events l JOIN clinical_targets t ON t.tenant_id=l.tenant_id AND t.id=l.target_id AND t.client_id=l.client_id WHERE t.tenant_id=$1 AND t.client_id=$2 AND t.id=ANY($3::uuid[])
		ORDER BY 1,2,3`, t, c, ids)
	if err != nil {
		return nil, err
	}
	defer links.Close()
	for links.Next() {
		var owner, linked uuid.UUID
		var kind string
		if err := links.Scan(&owner, &kind, &linked); err != nil {
			return nil, err
		}
		if i, ok := byID[owner]; ok {
			switch kind {
			case "evidence":
				out[i].EvidenceIDs = append(out[i].EvidenceIDs, linked)
			case "hypothesis":
				out[i].HypothesisIDs = append(out[i].HypothesisIDs, linked)
			case "event":
				out[i].EventIDs = append(out[i].EventIDs, linked)
			}
		}
	}
	return out, links.Err()
}
func (r *ClinicalLongitudinalRepository) ListGoals(ctx context.Context, t, c uuid.UUID) ([]longitudinal.Goal, error) {
	return r.listGoals(ctx, t, c, 0, 0, nil)
}
func (r *ClinicalLongitudinalRepository) ListGoalsPage(ctx context.Context, t, c uuid.UUID, limit, offset int) ([]longitudinal.Goal, error) {
	return r.listGoals(ctx, t, c, limit, offset, nil)
}
func (r *ClinicalLongitudinalRepository) listGoals(ctx context.Context, t, c uuid.UUID, limit, offset int, processIDs []uuid.UUID) ([]longitudinal.Goal, error) {
	query := goalSelect + ` WHERE tenant_id=$1 AND client_id=$2`
	args := []any{t, c}
	if processIDs != nil {
		query += ` AND process_id=ANY($3::uuid[])`
		args = append(args, processIDs)
	}
	query += ` ORDER BY process_id,CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,created_at,id`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := []longitudinal.Goal{}
	for rows.Next() {
		x, e := scanGoal(rows)
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
	byGoal := make(map[uuid.UUID]int, len(out))
	goalIDs := make([]uuid.UUID, 0, len(out))
	for i := range out {
		byGoal[out[i].ID] = i
		goalIDs = append(goalIDs, out[i].ID)
	}
	targetRows, err := r.pool.Query(ctx, `SELECT l.goal_id,l.target_id FROM clinical_goal_targets l JOIN clinical_goals g ON g.tenant_id=l.tenant_id AND g.id=l.goal_id AND g.client_id=l.client_id WHERE g.tenant_id=$1 AND g.client_id=$2 AND g.id=ANY($3::uuid[]) ORDER BY l.goal_id,l.target_id`, t, c, goalIDs)
	if err != nil {
		return nil, err
	}
	for targetRows.Next() {
		var goalID, targetID uuid.UUID
		if err := targetRows.Scan(&goalID, &targetID); err != nil {
			targetRows.Close()
			return nil, err
		}
		if i, ok := byGoal[goalID]; ok {
			out[i].TargetIDs = append(out[i].TargetIDs, targetID)
		}
	}
	err = targetRows.Err()
	targetRows.Close()
	if err != nil {
		return nil, err
	}
	indicatorRows, err := r.pool.Query(ctx, indicatorSelect+` WHERE tenant_id=$1 AND client_id=$2 AND goal_id=ANY($3::uuid[]) ORDER BY goal_id,created_at,id`, t, c, goalIDs)
	if err != nil {
		return nil, err
	}
	indicatorIndex := make(map[uuid.UUID][2]int)
	indicatorIDs := []uuid.UUID{}
	for indicatorRows.Next() {
		indicator, err := scanIndicator(indicatorRows)
		if err != nil {
			indicatorRows.Close()
			return nil, err
		}
		if i, ok := byGoal[indicator.GoalID]; ok {
			indicatorIndex[indicator.ID] = [2]int{i, len(out[i].Indicators)}
			indicatorIDs = append(indicatorIDs, indicator.ID)
			out[i].Indicators = append(out[i].Indicators, indicator)
		}
	}
	err = indicatorRows.Err()
	indicatorRows.Close()
	if err != nil {
		return nil, err
	}
	if len(indicatorIDs) == 0 {
		return out, nil
	}
	linkRows, err := r.pool.Query(ctx, `
		SELECT l.indicator_id,l.evidence_id,'evidence',l.relation_type FROM clinical_indicator_evidence l JOIN clinical_goal_indicators i ON i.tenant_id=l.tenant_id AND i.id=l.indicator_id AND i.client_id=l.client_id WHERE i.tenant_id=$1 AND i.client_id=$2 AND i.id=ANY($3::uuid[])
		UNION ALL SELECT l.indicator_id,l.event_id,'event',l.relation_type FROM clinical_indicator_events l JOIN clinical_goal_indicators i ON i.tenant_id=l.tenant_id AND i.id=l.indicator_id AND i.client_id=l.client_id WHERE i.tenant_id=$1 AND i.client_id=$2 AND i.id=ANY($3::uuid[])
		ORDER BY 1,3,2`, t, c, indicatorIDs)
	if err != nil {
		return nil, err
	}
	defer linkRows.Close()
	for linkRows.Next() {
		var indicatorID uuid.UUID
		var link longitudinal.GoalIndicatorLink
		if err := linkRows.Scan(&indicatorID, &link.SourceID, &link.SourceType, &link.RelationType); err != nil {
			return nil, err
		}
		if pos, ok := indicatorIndex[indicatorID]; ok {
			out[pos[0]].Indicators[pos[1]].Links = append(out[pos[0]].Indicators[pos[1]].Links, link)
		}
	}
	return out, linkRows.Err()
}
func (r *ClinicalLongitudinalRepository) listRationales(ctx context.Context, t, c, process uuid.UUID) ([]longitudinal.TherapeuticRationale, error) {
	return r.listRationalesFiltered(ctx, t, c, &process, nil, nil)
}
func (r *ClinicalLongitudinalRepository) listRationalesByClient(ctx context.Context, t, c uuid.UUID) ([]longitudinal.TherapeuticRationale, error) {
	return r.listRationalesFiltered(ctx, t, c, nil, nil, nil)
}
func (r *ClinicalLongitudinalRepository) listRationalesByProcessIDs(ctx context.Context, t, c uuid.UUID, processIDs []uuid.UUID) ([]longitudinal.TherapeuticRationale, error) {
	return r.listRationalesFiltered(ctx, t, c, nil, nil, processIDs)
}
func (r *ClinicalLongitudinalRepository) listRationalesByIDs(ctx context.Context, t, c uuid.UUID, ids []uuid.UUID) ([]longitudinal.TherapeuticRationale, error) {
	if len(ids) == 0 {
		return []longitudinal.TherapeuticRationale{}, nil
	}
	return r.listRationalesFiltered(ctx, t, c, nil, ids, nil)
}
func (r *ClinicalLongitudinalRepository) listRationalesFiltered(ctx context.Context, t, c uuid.UUID, process *uuid.UUID, ids, processIDs []uuid.UUID) ([]longitudinal.TherapeuticRationale, error) {
	query := rationaleSelect + ` WHERE r.tenant_id=$1 AND r.client_id=$2`
	args := []any{t, c}
	if process != nil {
		query += fmt.Sprintf(` AND r.process_id=$%d`, len(args)+1)
		args = append(args, *process)
	}
	if ids != nil {
		query += fmt.Sprintf(` AND r.id=ANY($%d::uuid[])`, len(args)+1)
		args = append(args, ids)
	}
	if processIDs != nil {
		query += fmt.Sprintf(` AND r.process_id=ANY($%d::uuid[])`, len(args)+1)
		args = append(args, processIDs)
	}
	query += ` ORDER BY r.process_id,r.target_id,r.goal_id,r.approach_slug,r.approach_version,r.id`
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := []longitudinal.TherapeuticRationale{}
	for rows.Next() {
		x, err := scanRationale(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(out) == 0 {
		return out, err
	}
	byID := make(map[uuid.UUID]int, len(out))
	selectedIDs := make([]uuid.UUID, 0, len(out))
	for i := range out {
		byID[out[i].ID] = i
		selectedIDs = append(selectedIDs, out[i].ID)
	}
	links, err := r.pool.Query(ctx, `
		SELECT l.rationale_id,'evidence',l.evidence_id FROM therapeutic_rationale_evidence l JOIN therapeutic_rationales r ON r.tenant_id=l.tenant_id AND r.id=l.rationale_id AND r.client_id=l.client_id WHERE r.tenant_id=$1 AND r.client_id=$2 AND r.id=ANY($3::uuid[])
		UNION ALL SELECT l.rationale_id,'hypothesis',l.hypothesis_id FROM therapeutic_rationale_hypotheses l JOIN therapeutic_rationales r ON r.tenant_id=l.tenant_id AND r.id=l.rationale_id AND r.client_id=l.client_id WHERE r.tenant_id=$1 AND r.client_id=$2 AND r.id=ANY($3::uuid[])
		ORDER BY 1,2,3`, t, c, selectedIDs)
	if err != nil {
		return nil, err
	}
	defer links.Close()
	for links.Next() {
		var owner, linked uuid.UUID
		var kind string
		if err := links.Scan(&owner, &kind, &linked); err != nil {
			return nil, err
		}
		if i, ok := byID[owner]; ok {
			if kind == "evidence" {
				out[i].EvidenceIDs = append(out[i].EvidenceIDs, linked)
			} else {
				out[i].HypothesisIDs = append(out[i].HypothesisIDs, linked)
			}
		}
	}
	return out, links.Err()
}
func (r *ClinicalLongitudinalRepository) loadRationaleLinks(ctx context.Context, t uuid.UUID, x *longitudinal.TherapeuticRationale) error {
	for _, s := range []struct {
		q   string
		dst *[]uuid.UUID
	}{{`SELECT evidence_id FROM therapeutic_rationale_evidence WHERE tenant_id=$1 AND rationale_id=$2 ORDER BY evidence_id`, &x.EvidenceIDs}, {`SELECT hypothesis_id FROM therapeutic_rationale_hypotheses WHERE tenant_id=$1 AND rationale_id=$2 ORDER BY hypothesis_id`, &x.HypothesisIDs}} {
		rows, err := r.pool.Query(ctx, s.q, t, x.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			*s.dst = append(*s.dst, id)
		}
		rows.Close()
	}
	return nil
}
func (r *ClinicalLongitudinalRepository) ListGIRAs(ctx context.Context, t, c uuid.UUID) ([]longitudinal.GIRA, error) {
	return r.listGIRAs(ctx, t, c, 0, 0, nil)
}
func (r *ClinicalLongitudinalRepository) ListGIRAsPage(ctx context.Context, t, c uuid.UUID, limit, offset int) ([]longitudinal.GIRA, error) {
	return r.listGIRAs(ctx, t, c, limit, offset, nil)
}
func (r *ClinicalLongitudinalRepository) listGIRAs(ctx context.Context, t, c uuid.UUID, limit, offset int, processIDs []uuid.UUID) ([]longitudinal.GIRA, error) {
	query := giraSelect + ` WHERE tenant_id=$1 AND client_id=$2`
	args := []any{t, c}
	if processIDs != nil {
		query += ` AND process_id=ANY($3::uuid[])`
		args = append(args, processIDs)
	}
	query += ` ORDER BY process_id,gira_version DESC,id`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := []longitudinal.GIRA{}
	for rows.Next() {
		x, e := scanGIRA(rows)
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
	giraIDs := make([]uuid.UUID, 0, len(out))
	for i := range out {
		byID[out[i].ID] = i
		giraIDs = append(giraIDs, out[i].ID)
	}
	linkRows, err := r.pool.Query(ctx, `
		SELECT l.gira_id,'target',l.target_id FROM gira_targets l JOIN giras g ON g.tenant_id=l.tenant_id AND g.id=l.gira_id AND g.client_id=l.client_id WHERE g.tenant_id=$1 AND g.client_id=$2 AND g.id=ANY($3::uuid[])
		UNION ALL SELECT l.gira_id,'goal',l.goal_id FROM gira_goals l JOIN giras g ON g.tenant_id=l.tenant_id AND g.id=l.gira_id AND g.client_id=l.client_id WHERE g.tenant_id=$1 AND g.client_id=$2 AND g.id=ANY($3::uuid[])
		ORDER BY 1,2,3`, t, c, giraIDs)
	if err != nil {
		return nil, err
	}
	for linkRows.Next() {
		var giraID, linked uuid.UUID
		var kind string
		if err := linkRows.Scan(&giraID, &kind, &linked); err != nil {
			linkRows.Close()
			return nil, err
		}
		if i, ok := byID[giraID]; ok {
			if kind == "target" {
				out[i].TargetIDs = append(out[i].TargetIDs, linked)
			} else {
				out[i].GoalIDs = append(out[i].GoalIDs, linked)
			}
		}
	}
	err = linkRows.Err()
	linkRows.Close()
	if err != nil {
		return nil, err
	}
	rationaleRows, err := r.pool.Query(ctx, `SELECT l.gira_id,l.rationale_id FROM gira_rationales l JOIN giras g ON g.tenant_id=l.tenant_id AND g.id=l.gira_id AND g.client_id=l.client_id WHERE g.tenant_id=$1 AND g.client_id=$2 AND g.id=ANY($3::uuid[])`, t, c, giraIDs)
	if err != nil {
		return nil, err
	}
	rationaleOwners := make(map[uuid.UUID][]uuid.UUID)
	rationaleIDs := []uuid.UUID{}
	for rationaleRows.Next() {
		var giraID, rationaleID uuid.UUID
		if err := rationaleRows.Scan(&giraID, &rationaleID); err != nil {
			rationaleRows.Close()
			return nil, err
		}
		if _, ok := byID[giraID]; ok {
			if _, seen := rationaleOwners[rationaleID]; !seen {
				rationaleIDs = append(rationaleIDs, rationaleID)
			}
			rationaleOwners[rationaleID] = append(rationaleOwners[rationaleID], giraID)
		}
	}
	err = rationaleRows.Err()
	rationaleRows.Close()
	if err != nil {
		return nil, err
	}
	rationales, err := r.listRationalesByIDs(ctx, t, c, rationaleIDs)
	if err != nil {
		return nil, err
	}
	for _, rationale := range rationales {
		for _, giraID := range rationaleOwners[rationale.ID] {
			out[byID[giraID]].Rationales = append(out[byID[giraID]].Rationales, rationale)
		}
	}
	for i := range out {
		sort.Slice(out[i].Rationales, func(a, b int) bool {
			left, right := out[i].Rationales[a], out[i].Rationales[b]
			if left.ApproachSlug != right.ApproachSlug {
				return left.ApproachSlug < right.ApproachSlug
			}
			if left.ApproachVersion != right.ApproachVersion {
				return left.ApproachVersion < right.ApproachVersion
			}
			return left.ID.String() < right.ID.String()
		})
	}
	phaseRows, err := r.pool.Query(ctx, phaseSelect+` WHERE tenant_id=$1 AND client_id=$2 AND gira_id=ANY($3::uuid[]) ORDER BY gira_id,position,id`, t, c, giraIDs)
	if err != nil {
		return nil, err
	}
	phaseIndex := make(map[uuid.UUID][2]int)
	phaseIDs := []uuid.UUID{}
	for phaseRows.Next() {
		phase, err := scanPhase(phaseRows)
		if err != nil {
			phaseRows.Close()
			return nil, err
		}
		if i, ok := byID[phase.GIRAID]; ok {
			phaseIndex[phase.ID] = [2]int{i, len(out[i].Phases)}
			phaseIDs = append(phaseIDs, phase.ID)
			out[i].Phases = append(out[i].Phases, phase)
		}
	}
	err = phaseRows.Err()
	phaseRows.Close()
	if err != nil {
		return nil, err
	}
	if len(phaseIDs) == 0 {
		return out, nil
	}
	phaseLinks, err := r.pool.Query(ctx, `
		SELECT l.phase_id,'goal',l.goal_id FROM gira_phase_goals l JOIN gira_phases p ON p.tenant_id=l.tenant_id AND p.id=l.phase_id AND p.client_id=l.client_id WHERE p.tenant_id=$1 AND p.client_id=$2 AND p.id=ANY($3::uuid[])
		UNION ALL SELECT l.phase_id,'rationale',l.rationale_id FROM gira_phase_rationales l JOIN gira_phases p ON p.tenant_id=l.tenant_id AND p.id=l.phase_id AND p.client_id=l.client_id WHERE p.tenant_id=$1 AND p.client_id=$2 AND p.id=ANY($3::uuid[])
		UNION ALL SELECT l.phase_id,'indicator',l.indicator_id FROM gira_phase_indicators l JOIN gira_phases p ON p.tenant_id=l.tenant_id AND p.id=l.phase_id AND p.client_id=l.client_id WHERE p.tenant_id=$1 AND p.client_id=$2 AND p.id=ANY($3::uuid[])
		ORDER BY 1,2,3`, t, c, phaseIDs)
	if err != nil {
		return nil, err
	}
	defer phaseLinks.Close()
	for phaseLinks.Next() {
		var phaseID, linked uuid.UUID
		var kind string
		if err := phaseLinks.Scan(&phaseID, &kind, &linked); err != nil {
			return nil, err
		}
		if pos, ok := phaseIndex[phaseID]; ok {
			phase := &out[pos[0]].Phases[pos[1]]
			switch kind {
			case "goal":
				phase.GoalIDs = append(phase.GoalIDs, linked)
			case "rationale":
				phase.RationaleIDs = append(phase.RationaleIDs, linked)
			case "indicator":
				phase.IndicatorIDs = append(phase.IndicatorIDs, linked)
			}
		}
	}
	return out, phaseLinks.Err()
}
func (r *ClinicalLongitudinalRepository) GetGIRA(ctx context.Context, t, id uuid.UUID) (longitudinal.GIRA, error) {
	x, err := scanGIRA(r.pool.QueryRow(ctx, giraSelect+` WHERE tenant_id=$1 AND id=$2`, t, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return x, domainerrors.ErrNotFound
	}
	if err != nil {
		return x, err
	}
	err = r.loadGIRA(ctx, t, &x)
	return x, err
}
func (r *ClinicalLongitudinalRepository) loadGIRA(ctx context.Context, t uuid.UUID, x *longitudinal.GIRA) error {
	for _, s := range []struct {
		q   string
		dst *[]uuid.UUID
	}{{`SELECT target_id FROM gira_targets WHERE tenant_id=$1 AND gira_id=$2 ORDER BY target_id`, &x.TargetIDs}, {`SELECT goal_id FROM gira_goals WHERE tenant_id=$1 AND gira_id=$2 ORDER BY goal_id`, &x.GoalIDs}} {
		rows, err := r.pool.Query(ctx, s.q, t, x.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			*s.dst = append(*s.dst, id)
		}
		rows.Close()
	}
	rows, err := r.pool.Query(ctx, rationaleSelect+` JOIN gira_rationales l ON l.tenant_id=r.tenant_id AND l.rationale_id=r.id WHERE l.tenant_id=$1 AND l.gira_id=$2 ORDER BY r.approach_slug,r.approach_version,r.id`, t, x.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		v, e := scanRationale(rows)
		if e != nil {
			rows.Close()
			return e
		}
		if e = r.loadRationaleLinks(ctx, t, &v); e != nil {
			rows.Close()
			return e
		}
		x.Rationales = append(x.Rationales, v)
	}
	rows.Close()
	rows, err = r.pool.Query(ctx, phaseSelect+` WHERE tenant_id=$1 AND gira_id=$2 ORDER BY position,id`, t, x.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		p, e := scanPhase(rows)
		if e != nil {
			return e
		}
		if e := r.loadPhaseLinks(ctx, t, &p); e != nil {
			return e
		}
		x.Phases = append(x.Phases, p)
	}
	return rows.Err()
}
func (r *ClinicalLongitudinalRepository) loadPhaseLinks(ctx context.Context, t uuid.UUID, x *longitudinal.GIRAPhase) error {
	for _, s := range []struct {
		q   string
		dst *[]uuid.UUID
	}{{`SELECT goal_id FROM gira_phase_goals WHERE tenant_id=$1 AND phase_id=$2 ORDER BY goal_id`, &x.GoalIDs}, {`SELECT rationale_id FROM gira_phase_rationales WHERE tenant_id=$1 AND phase_id=$2 ORDER BY rationale_id`, &x.RationaleIDs}, {`SELECT indicator_id FROM gira_phase_indicators WHERE tenant_id=$1 AND phase_id=$2 ORDER BY indicator_id`, &x.IndicatorIDs}} {
		rows, err := r.pool.Query(ctx, s.q, t, x.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			*s.dst = append(*s.dst, id)
		}
		rows.Close()
	}
	return nil
}
func (r *ClinicalLongitudinalRepository) processTherapeuticStrategy(ctx context.Context, t, c, p uuid.UUID, currentOnly bool) (*longitudinal.TherapeuticStrategy, error) {
	targets, err := r.ListTargets(ctx, t, c)
	if err != nil {
		return nil, err
	}
	goals, err := r.ListGoals(ctx, t, c)
	if err != nil {
		return nil, err
	}
	rationales, err := r.listRationales(ctx, t, c, p)
	if err != nil {
		return nil, err
	}
	giras, err := r.ListGIRAs(ctx, t, c)
	if err != nil {
		return nil, err
	}
	out := &longitudinal.TherapeuticStrategy{Targets: []longitudinal.Target{}, Goals: []longitudinal.Goal{}, Rationales: []longitudinal.TherapeuticRationale{}, GIRAs: []longitudinal.GIRA{}}
	for _, x := range targets {
		if x.ProcessID == p && (!currentOnly || x.ApprovalStatus == "approved") {
			out.Targets = append(out.Targets, x)
		}
	}
	for _, x := range goals {
		if x.ProcessID == p && (!currentOnly || x.ApprovalStatus == "approved") {
			out.Goals = append(out.Goals, x)
		}
	}
	for _, x := range rationales {
		if !currentOnly || x.ApprovalStatus == "approved" {
			out.Rationales = append(out.Rationales, x)
		}
	}
	for _, x := range giras {
		if x.ProcessID == p && (!currentOnly || x.ApprovalStatus == "approved") {
			out.GIRAs = append(out.GIRAs, x)
		}
	}
	return out, nil
}

func (r *ClinicalLongitudinalRepository) ListApproaches(ctx context.Context) ([]longitudinal.ApproachDefinition, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,slug,version,name,description,target_domains,core_mechanisms,intervention_families,progress_signals,limitations,cautions,status,created_at,updated_at FROM therapeutic_approach_definitions ORDER BY slug,version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.ApproachDefinition{}
	for rows.Next() {
		var x longitudinal.ApproachDefinition
		if err := rows.Scan(&x.ID, &x.Slug, &x.Version, &x.Name, &x.Description, &x.TargetDomains, &x.CoreMechanisms, &x.InterventionFamilies, &x.ProgressSignals, &x.Limitations, &x.Cautions, &x.Status, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) ListTechniques(ctx context.Context) ([]longitudinal.TechniqueDefinition, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,slug,version,name,approach_slug,approach_version,description,target_domains,mechanism,indications,cautions,limits,expected_signals,status,created_at,updated_at FROM therapeutic_technique_definitions ORDER BY approach_slug,slug,version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []longitudinal.TechniqueDefinition{}
	for rows.Next() {
		var x longitudinal.TechniqueDefinition
		if err := rows.Scan(&x.ID, &x.Slug, &x.Version, &x.Name, &x.ApproachSlug, &x.ApproachVersion, &x.Description, &x.TargetDomains, &x.Mechanism, &x.Indications, &x.Cautions, &x.Limits, &x.ExpectedSignals, &x.Status, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ClinicalLongitudinalRepository) GetStrategyHistory(ctx context.Context, t, c uuid.UUID, entityType string, id uuid.UUID) (longitudinal.StrategyHistory, error) {
	allowed := map[string]bool{"target": true, "goal": true, "goal_indicator": true, "therapeutic_rationale": true, "gira": true, "gira_phase": true}
	if !allowed[entityType] {
		return longitudinal.StrategyHistory{}, domainerrors.NewValidation("invalid strategy entity type")
	}
	items, err := r.entityHistoryTransitions(ctx, t, c, entityType, id)
	if err != nil {
		return longitudinal.StrategyHistory{}, err
	}
	if len(items) == 0 {
		return longitudinal.StrategyHistory{}, domainerrors.ErrNotFound
	}
	return longitudinal.StrategyHistory{EntityType: entityType, EntityID: id, Transitions: items}, nil
}
