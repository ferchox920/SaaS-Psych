package longitudinal

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type strategyEntity struct {
	kind      string
	version   int
	processID uuid.UUID
	parentID  uuid.UUID
}

// ValidateGIRABuilderResult enforces the strategy boundary against the exact
// approved snapshot and registry versions sent to the model. It also simulates
// ordered create dependencies, so invented and forward UUID references fail
// before a diff or successful AI run can be persisted.
func ValidateGIRABuilderResult(input GIRABuilderInput, result InterpreterResult) error {
	entities := map[uuid.UUID]strategyEntity{}
	goalTargets := map[uuid.UUID]map[uuid.UUID]bool{}
	giraGoals := map[uuid.UUID]map[uuid.UUID]bool{}
	giraRationales := map[uuid.UUID]map[uuid.UUID]bool{}
	evidence := map[uuid.UUID]bool{}
	events := map[uuid.UUID]bool{}
	hypotheses := map[uuid.UUID]bool{}
	targetApproaches := map[uuid.UUID]string{}
	for _, e := range input.ApprovedEvidence {
		if e.Status == "active" {
			evidence[e.ID] = true
		}
	}
	for _, e := range input.ApprovedEvents {
		if e.ApprovalStatus == "approved" {
			events[e.ID] = true
		}
	}
	for _, h := range input.ApprovedHypotheses {
		if h.ApprovalStatus == "approved" {
			hypotheses[h.ID] = true
		}
	}
	for _, t := range input.CurrentStrategy.Targets {
		if t.ApprovalStatus == "approved" {
			entities[t.ID] = strategyEntity{"target", t.Version, t.ProcessID, uuid.Nil}
		}
	}
	for _, g := range input.CurrentStrategy.Goals {
		if g.ApprovalStatus == "approved" {
			entities[g.ID] = strategyEntity{"goal", g.Version, g.ProcessID, uuid.Nil}
		}
		goalTargets[g.ID] = uuidSet(g.TargetIDs)
		for _, i := range g.Indicators {
			entities[i.ID] = strategyEntity{"indicator", i.Version, g.ProcessID, g.ID}
		}
	}
	for _, r := range input.CurrentStrategy.Rationales {
		if r.ApprovalStatus == "approved" {
			entities[r.ID] = strategyEntity{"rationale", r.Version, r.ProcessID, r.GoalID}
			targetApproaches[r.TargetID] = fmt.Sprintf("%s/%d", r.ApproachSlug, r.ApproachVersion)
		}
	}
	for _, g := range input.CurrentStrategy.GIRAs {
		if g.ApprovalStatus == "approved" {
			entities[g.ID] = strategyEntity{"gira", g.EntityVersion, g.ProcessID, uuid.Nil}
		}
		giraGoals[g.ID] = uuidSet(g.GoalIDs)
		rationaleIDs := make([]uuid.UUID, 0, len(g.Rationales))
		for _, r := range g.Rationales {
			rationaleIDs = append(rationaleIDs, r.ID)
		}
		giraRationales[g.ID] = uuidSet(rationaleIDs)
		for _, p := range g.Phases {
			entities[p.ID] = strategyEntity{"phase", p.Version, g.ProcessID, g.ID}
		}
	}
	approaches := map[string]bool{}
	techniques := map[string]string{}
	for _, a := range input.ApproachRegistry {
		approaches[fmt.Sprintf("%s/%d", a.Slug, a.Version)] = true
	}
	for _, t := range input.TechniqueRegistry {
		techniques[fmt.Sprintf("%s/%d", t.Slug, t.Version)] = fmt.Sprintf("%s/%d", t.ApproachSlug, t.ApproachVersion)
	}

	validGrounding := func(es, hs, evs []uuid.UUID) bool {
		if len(es)+len(hs)+len(evs) == 0 {
			return false
		}
		for _, id := range es {
			if !evidence[id] {
				return false
			}
		}
		for _, id := range hs {
			if !hypotheses[id] {
				return false
			}
		}
		for _, id := range evs {
			if !events[id] {
				return false
			}
		}
		return true
	}
	require := func(id uuid.UUID, kind string) (strategyEntity, error) {
		e, ok := entities[id]
		if !ok || e.kind != kind {
			return e, domainerrors.NewValidation("GIRA builder references an absent or wrong-kind entity")
		}
		return e, nil
	}
	selectionKeys := map[string]string{}
	createdGoals := map[uuid.UUID]bool{}
	goalsWithCreatedIndicator := map[uuid.UUID]bool{}
	for _, current := range input.CurrentStrategy.Rationales {
		selectionKeys[fmt.Sprintf("%s/%s/%s/%d", current.TargetID, current.GoalID, current.ApproachSlug, current.ApproachVersion)] = normalizeText(current.Rationale)
	}
	for _, op := range result.Operations {
		if op.OperationType == "achieve_goal" || op.OperationType == "complete_gira" || op.OperationType == "complete_gira_phase" {
			return domainerrors.NewValidation("GIRA builder cannot complete goals, GIRAs or phases")
		}
		if !containsString(giraBuilderOperations, op.OperationType) {
			return domainerrors.NewValidation("operation is outside the GIRA builder contract")
		}
		id := *op.TargetEntityID
		if strings.HasPrefix(op.OperationType, "create_") {
			if _, exists := entities[id]; exists {
				return domainerrors.NewValidation("create operation target UUID already exists")
			}
		} else {
			e, ok := entities[id]
			if !ok || op.ExpectedEntityVersion == nil || e.version != *op.ExpectedEntityVersion {
				return domainerrors.NewValidation("stale or absent strategy entity version")
			}
		}
		switch op.OperationType {
		case "create_target":
			var p CreateTargetProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if p.ProcessID != input.SelectedProcess.ID || !validGrounding(p.EvidenceIDs, p.HypothesisIDs, p.EventIDs) {
				return domainerrors.NewValidation("target requires approved grounding in the selected process")
			}
			entities[id] = strategyEntity{"target", 1, p.ProcessID, uuid.Nil}
		case "update_target":
			var p UpdateTargetProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			e, err := require(id, "target")
			if err != nil {
				return err
			}
			if e.processID != input.SelectedProcess.ID || !validGrounding(p.EvidenceIDs, p.HypothesisIDs, p.EventIDs) {
				return domainerrors.NewValidation("updated target requires approved grounding")
			}
			e.version++
			entities[id] = e
		case "resolve_target", "retire_target":
			e, err := require(id, "target")
			if err != nil {
				return err
			}
			e.version++
			entities[id] = e
		case "create_goal":
			var p CreateGoalProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if p.ProcessID != input.SelectedProcess.ID || len(p.TargetIDs) == 0 {
				return domainerrors.NewValidation("goal requires a target in the selected process")
			}
			for _, targetID := range p.TargetIDs {
				e, err := require(targetID, "target")
				if err != nil || e.processID != p.ProcessID {
					return domainerrors.NewValidation("goal target is invalid")
				}
			}
			entities[id] = strategyEntity{"goal", 1, p.ProcessID, uuid.Nil}
			goalTargets[id] = uuidSet(p.TargetIDs)
			createdGoals[id] = true
		case "update_goal":
			var p UpdateGoalProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			e, err := require(id, "goal")
			if err != nil {
				return err
			}
			if len(p.TargetIDs) == 0 {
				return domainerrors.NewValidation("goal requires target")
			}
			for _, targetID := range p.TargetIDs {
				target, x := require(targetID, "target")
				if x != nil || target.processID != e.processID {
					return domainerrors.NewValidation("goal target is invalid")
				}
			}
			e.version++
			entities[id] = e
			goalTargets[id] = uuidSet(p.TargetIDs)
		case "activate_goal", "pause_goal", "abandon_goal":
			e, err := require(id, "goal")
			if err != nil {
				return err
			}
			e.version++
			entities[id] = e
		case "create_goal_indicator":
			var p CreateGoalIndicatorProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			g, err := require(p.GoalID, "goal")
			if err != nil {
				return err
			}
			entities[id] = strategyEntity{"indicator", 1, g.processID, p.GoalID}
			goalsWithCreatedIndicator[p.GoalID] = true
		case "update_goal_indicator":
			e, err := require(id, "indicator")
			if err != nil {
				return err
			}
			e.version++
			entities[id] = e
		case "link_indicator_evidence", "link_indicator_event":
			var p LinkIndicatorSourceProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if p.IndicatorID != id {
				return domainerrors.NewValidation("indicator link target mismatch")
			}
			e, err := require(id, "indicator")
			if err != nil {
				return err
			}
			if op.OperationType == "link_indicator_evidence" && !evidence[p.SourceID] {
				return domainerrors.NewValidation("indicator evidence is not approved")
			}
			if op.OperationType == "link_indicator_event" && !events[p.SourceID] {
				return domainerrors.NewValidation("indicator event is not approved")
			}
			e.version++
			entities[id] = e
		case "create_therapeutic_rationale", "update_therapeutic_rationale":
			var p CreateTherapeuticRationaleProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if p.ProcessID != input.SelectedProcess.ID || p.GroundingStatus != "grounded" || len(p.EvidenceIDs)+len(p.HypothesisIDs) == 0 {
				return domainerrors.NewValidation("confident rationale requires approved grounding")
			}
			target, err := require(p.TargetID, "target")
			if err != nil {
				return err
			}
			goal, err := require(p.GoalID, "goal")
			if err != nil {
				return err
			}
			if target.processID != p.ProcessID || goal.processID != p.ProcessID {
				return domainerrors.NewValidation("rationale chain crosses process")
			}
			if !goalTargets[p.GoalID][p.TargetID] {
				return domainerrors.NewValidation("rationale target must be linked to its goal")
			}
			for _, x := range p.EvidenceIDs {
				if !evidence[x] {
					return domainerrors.NewValidation("rationale evidence is invalid")
				}
			}
			for _, x := range p.HypothesisIDs {
				if !hypotheses[x] {
					return domainerrors.NewValidation("rationale hypothesis is invalid")
				}
			}
			approachKey := fmt.Sprintf("%s/%d", p.ApproachSlug, p.ApproachVersion)
			if !approaches[approachKey] {
				return domainerrors.NewValidation("unknown approach version")
			}
			if p.TechniqueSlug != nil {
				key := fmt.Sprintf("%s/%d", *p.TechniqueSlug, *p.TechniqueVersion)
				if techniques[key] != approachKey {
					return domainerrors.NewValidation("technique is incompatible with approach version")
				}
			}
			if prior, exists := targetApproaches[p.TargetID]; exists && prior != approachKey {
				return domainerrors.NewValidation("multiple approaches require distinct targets and clinical functions")
			}
			targetApproaches[p.TargetID] = approachKey
			selection := fmt.Sprintf("%s/%s/%s", p.TargetID, p.GoalID, approachKey)
			if prior, exists := selectionKeys[selection]; exists && prior == normalizeText(p.Rationale) && op.OperationType == "create_therapeutic_rationale" {
				return domainerrors.NewValidation("duplicate approach selection requires a distinct clinical function")
			}
			selectionKeys[selection] = normalizeText(p.Rationale)
			if op.OperationType == "create_therapeutic_rationale" {
				entities[id] = strategyEntity{"rationale", 1, p.ProcessID, p.GoalID}
			} else {
				e, err := require(id, "rationale")
				if err != nil {
					return err
				}
				e.version++
				entities[id] = e
			}
		case "create_gira":
			var p CreateGIRAProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			if p.ProcessID != input.SelectedProcess.ID || len(p.TargetIDs) == 0 || len(p.GoalIDs) == 0 || len(p.RationaleIDs) == 0 {
				return domainerrors.NewValidation("GIRA requires target, goal and rationale")
			}
			for _, x := range p.TargetIDs {
				e, err := require(x, "target")
				if err != nil || e.processID != p.ProcessID {
					return domainerrors.NewValidation("GIRA target is invalid")
				}
			}
			for _, x := range p.GoalIDs {
				e, err := require(x, "goal")
				if err != nil || e.processID != p.ProcessID {
					return domainerrors.NewValidation("GIRA goal is invalid")
				}
			}
			for _, x := range p.RationaleIDs {
				e, err := require(x, "rationale")
				if err != nil || e.processID != p.ProcessID {
					return domainerrors.NewValidation("GIRA rationale is invalid")
				}
			}
			if len(input.CurrentStrategy.GIRAs) == 0 && (p.GIRAVersion != 1 || p.SupersedesGIRAID != nil) {
				return domainerrors.NewValidation("first GIRA must be version 1")
			}
			if len(input.CurrentStrategy.GIRAs) > 0 {
				max := 0
				validPrevious := false
				for _, current := range input.CurrentStrategy.GIRAs {
					if current.GIRAVersion > max {
						max = current.GIRAVersion
					}
					if p.SupersedesGIRAID != nil && current.ID == *p.SupersedesGIRAID {
						validPrevious = true
					}
				}
				if p.GIRAVersion != max+1 || !validPrevious {
					return domainerrors.NewValidation("new GIRA must preserve and supersede the prior version")
				}
			}
			entities[id] = strategyEntity{"gira", 1, p.ProcessID, uuid.Nil}
			giraGoals[id] = uuidSet(p.GoalIDs)
			giraRationales[id] = uuidSet(p.RationaleIDs)
		case "supersede_gira", "activate_gira", "pause_gira":
			e, err := require(id, "gira")
			if err != nil {
				return err
			}
			e.version++
			entities[id] = e
		case "create_gira_phase":
			var p CreateGIRAPhaseProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			g, err := require(p.GIRAID, "gira")
			if err != nil {
				return err
			}
			for _, x := range p.GoalIDs {
				e, err := require(x, "goal")
				if err != nil || e.processID != g.processID || !giraGoals[p.GIRAID][x] {
					return domainerrors.NewValidation("phase goal is invalid")
				}
			}
			for _, x := range p.RationaleIDs {
				e, err := require(x, "rationale")
				if err != nil || e.processID != g.processID || !giraRationales[p.GIRAID][x] {
					return domainerrors.NewValidation("phase rationale is invalid")
				}
			}
			for _, x := range p.IndicatorIDs {
				e, err := require(x, "indicator")
				if err != nil || e.processID != g.processID || !containsUUID(p.GoalIDs, e.parentID) {
					return domainerrors.NewValidation("phase indicator is invalid")
				}
			}
			entities[id] = strategyEntity{"phase", 1, g.processID, p.GIRAID}
		case "update_gira_phase":
			var p UpdateGIRAPhaseProposal
			if err := decodeStrict(op.Proposal, &p); err != nil {
				return err
			}
			e, err := require(id, "phase")
			if err != nil {
				return err
			}
			for _, x := range p.GoalIDs {
				goal, xerr := require(x, "goal")
				if xerr != nil || goal.processID != e.processID || !giraGoals[e.parentID][x] {
					return domainerrors.NewValidation("phase goal is invalid")
				}
			}
			for _, x := range p.RationaleIDs {
				rationale, xerr := require(x, "rationale")
				if xerr != nil || rationale.processID != e.processID || !giraRationales[e.parentID][x] {
					return domainerrors.NewValidation("phase rationale is invalid")
				}
			}
			for _, x := range p.IndicatorIDs {
				indicator, xerr := require(x, "indicator")
				if xerr != nil || indicator.processID != e.processID || !containsUUID(p.GoalIDs, indicator.parentID) {
					return domainerrors.NewValidation("phase indicator is invalid")
				}
			}
			e.version++
			entities[id] = e
		case "activate_gira_phase", "pause_gira_phase":
			e, err := require(id, "phase")
			if err != nil {
				return err
			}
			e.version++
			entities[id] = e
		}
	}
	for goalID := range createdGoals {
		if !goalsWithCreatedIndicator[goalID] {
			return domainerrors.NewValidation("new goal must be operationalized with at least one indicator")
		}
	}
	return nil
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func uuidSet(items []uuid.UUID) map[uuid.UUID]bool {
	out := make(map[uuid.UUID]bool, len(items))
	for _, id := range items {
		out[id] = true
	}
	return out
}

func containsUUID(items []uuid.UUID, value uuid.UUID) bool {
	for _, id := range items {
		if id == value {
			return true
		}
	}
	return false
}
