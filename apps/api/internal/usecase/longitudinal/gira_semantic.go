package longitudinal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

// GIRAModelInput is the compact, AI-facing contract. It deliberately omits
// database UUID choreography, audit fields, timestamps and nested domain
// objects. Existing entities are addressed by deterministic opaque refs.
type GIRAModelInput struct {
	SchemaVersion       string                  `json:"schema_version"`
	Process             GIRAProcessContext      `json:"process"`
	Evidence            []GIRAEvidenceContext   `json:"evidence"`
	Events              []GIRAEventContext      `json:"events"`
	Hypotheses          []GIRAHypothesisContext `json:"hypotheses"`
	CurrentStrategy     GIRAStrategyContext     `json:"current_strategy"`
	ApproachCandidates  []GIRAApproachContext   `json:"approach_candidates"`
	TechniqueCandidates []GIRATechniqueContext  `json:"technique_candidates"`
}

type GIRAProcessContext struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	ClinicalStatus string `json:"clinical_status"`
	Version        int    `json:"version"`
}
type GIRAEvidenceContext struct {
	Ref           string `json:"ref"`
	EpistemicType string `json:"epistemic_type"`
	Statement     string `json:"statement"`
	Version       int    `json:"version"`
}
type GIRAEventContext struct {
	Ref         string `json:"ref"`
	EventType   string `json:"event_type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     int    `json:"version"`
}
type GIRAHypothesisContext struct {
	Ref             string `json:"ref"`
	Statement       string `json:"statement"`
	ConfidenceLevel string `json:"confidence_level"`
	ClinicalStatus  string `json:"clinical_status"`
	Version         int    `json:"version"`
}
type GIRATargetContext struct {
	Ref            string `json:"ref"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	TargetType     string `json:"target_type"`
	ClinicalStatus string `json:"clinical_status"`
	Version        int    `json:"version"`
}
type GIRAGoalContext struct {
	Ref            string   `json:"ref"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	GoalType       string   `json:"goal_type"`
	Priority       string   `json:"priority"`
	ClinicalStatus string   `json:"clinical_status"`
	Version        int      `json:"version"`
	TargetRefs     []string `json:"target_refs"`
}
type GIRAIndicatorContext struct {
	Ref           string `json:"ref"`
	GoalRef       string `json:"goal_ref"`
	Description   string `json:"description"`
	IndicatorType string `json:"indicator_type"`
	Status        string `json:"status"`
	Version       int    `json:"version"`
}
type GIRARationaleContext struct {
	Ref              string  `json:"ref"`
	TargetRef        string  `json:"target_ref"`
	GoalRef          string  `json:"goal_ref"`
	ApproachSlug     string  `json:"approach_slug"`
	ApproachVersion  int     `json:"approach_version"`
	TechniqueSlug    *string `json:"technique_slug"`
	TechniqueVersion *int    `json:"technique_version"`
	Rationale        string  `json:"rationale"`
	ExpectedEffect   string  `json:"expected_effect"`
	Version          int     `json:"version"`
}
type GIRAGIRAContext struct {
	Ref            string   `json:"ref"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	ClinicalStatus string   `json:"clinical_status"`
	GIRAVersion    int      `json:"gira_version"`
	EntityVersion  int      `json:"entity_version"`
	TargetRefs     []string `json:"target_refs"`
	GoalRefs       []string `json:"goal_refs"`
	RationaleRefs  []string `json:"rationale_refs"`
}
type GIRAStrategyContext struct {
	Targets    []GIRATargetContext    `json:"targets"`
	Goals      []GIRAGoalContext      `json:"goals"`
	Indicators []GIRAIndicatorContext `json:"indicators"`
	Rationales []GIRARationaleContext `json:"rationales"`
	GIRAs      []GIRAGIRAContext      `json:"giras"`
}
type GIRAApproachContext struct {
	Slug           string   `json:"slug"`
	Version        int      `json:"version"`
	Name           string   `json:"name"`
	TargetDomains  []string `json:"target_domains"`
	CoreMechanisms []string `json:"core_mechanisms"`
	Limitations    []string `json:"limitations"`
	Cautions       []string `json:"cautions"`
}
type GIRATechniqueContext struct {
	Slug            string   `json:"slug"`
	Version         int      `json:"version"`
	Name            string   `json:"name"`
	ApproachSlug    string   `json:"approach_slug"`
	ApproachVersion int      `json:"approach_version"`
	TargetDomains   []string `json:"target_domains"`
	Mechanism       string   `json:"mechanism"`
	Cautions        []string `json:"cautions"`
	Limits          []string `json:"limits"`
	ExpectedSignals []string `json:"expected_signals"`
}

type giraRefIndex struct {
	evidence, events, hypotheses map[string]uuid.UUID
	targets, goals, indicators   map[string]uuid.UUID
	rationales, giras, phases    map[string]uuid.UUID
	versions                     map[string]int
}

type GIRAGenerationRequest struct {
	Snapshot      GIRABuilderInput
	ModelInput    GIRAModelInput
	RemoteContext RemoteGIRAContext
	Privacy       GIRAPrivacySummary
	refs          giraRefIndex
}

const (
	giraEvidenceLimit   = 24
	giraEventLimit      = 16
	giraHypothesisLimit = 16
	giraApproachLimit   = 12
	giraTechniqueLimit  = 32
)

// BoundGIRAContext applies deterministic, relationship-first limits before
// aliases and AI-run sources are built. It never reaches into other processes.
func BoundGIRAContext(snapshot GIRABuilderInput) GIRABuilderInput {
	directEvidence, directEvents := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, event := range snapshot.SelectedProcess.Events {
		directEvents[event.ID] = true
		for _, evidence := range event.Evidence {
			directEvidence[evidence.ID] = true
		}
	}
	for _, hypothesis := range snapshot.SelectedProcess.Hypotheses {
		for _, evidence := range hypothesis.SupportingEvidence {
			directEvidence[evidence.ID] = true
		}
		for _, evidence := range hypothesis.ContradictingEvidence {
			directEvidence[evidence.ID] = true
		}
	}
	for _, target := range snapshot.CurrentStrategy.Targets {
		for _, id := range target.EvidenceIDs {
			directEvidence[id] = true
		}
		for _, id := range target.EventIDs {
			directEvents[id] = true
		}
	}
	for _, rationale := range snapshot.CurrentStrategy.Rationales {
		for _, id := range rationale.EvidenceIDs {
			directEvidence[id] = true
		}
	}
	for _, goal := range snapshot.CurrentStrategy.Goals {
		for _, indicator := range goal.Indicators {
			for _, link := range indicator.Links {
				if link.SourceType == "evidence" {
					directEvidence[link.SourceID] = true
				}
				if link.SourceType == "event" {
					directEvents[link.SourceID] = true
				}
			}
		}
	}
	sort.SliceStable(snapshot.ApprovedEvidence, func(i, j int) bool {
		a, b := snapshot.ApprovedEvidence[i], snapshot.ApprovedEvidence[j]
		if directEvidence[a.ID] != directEvidence[b.ID] {
			return directEvidence[a.ID]
		}
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt)
		}
		return a.ID.String() < b.ID.String()
	})
	sort.SliceStable(snapshot.ApprovedEvents, func(i, j int) bool {
		a, b := snapshot.ApprovedEvents[i], snapshot.ApprovedEvents[j]
		if directEvents[a.ID] != directEvents[b.ID] {
			return directEvents[a.ID]
		}
		if !a.ObservedAt.Equal(b.ObservedAt) {
			return a.ObservedAt.After(b.ObservedAt)
		}
		return a.ID.String() < b.ID.String()
	})
	sort.SliceStable(snapshot.ApprovedHypotheses, func(i, j int) bool {
		a, b := snapshot.ApprovedHypotheses[i], snapshot.ApprovedHypotheses[j]
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt)
		}
		return a.ID.String() < b.ID.String()
	})
	if len(snapshot.ApprovedEvidence) > giraEvidenceLimit {
		snapshot.ApprovedEvidence = snapshot.ApprovedEvidence[:giraEvidenceLimit]
	}
	if len(snapshot.ApprovedEvents) > giraEventLimit {
		snapshot.ApprovedEvents = snapshot.ApprovedEvents[:giraEventLimit]
	}
	if len(snapshot.ApprovedHypotheses) > giraHypothesisLimit {
		snapshot.ApprovedHypotheses = snapshot.ApprovedHypotheses[:giraHypothesisLimit]
	}
	return snapshot
}

// MeasureGIRAContext returns only byte counts with a closed set of component
// names; it is safe for metrics and diagnostics because it contains no content.
func MeasureGIRAContext(input RemoteGIRAContext) map[string]int {
	parts := map[string]any{
		"system_prompt": GIRASystemPromptV1(), "process": input.Process,
		"evidence": input.Evidence, "events": input.Events, "hypotheses": input.Hypotheses,
		"strategy": input.CurrentStrategy, "approaches": input.ApproachCandidates,
		"techniques": input.TechniqueCandidates, "schema": GIRASemanticJSONSchema(),
	}
	out, total := map[string]int{}, 0
	for name, value := range parts {
		raw, _ := json.Marshal(value)
		out[name], total = len(raw), total+len(raw)
	}
	out["total"] = total
	return out
}

type GIRASemanticProposal struct {
	Targets        []GIRASemanticTarget        `json:"targets"`
	Goals          []GIRASemanticGoal          `json:"goals"`
	Indicators     []GIRASemanticIndicator     `json:"indicators"`
	Rationales     []GIRASemanticRationale     `json:"rationales"`
	GIRA           *GIRASemanticGIRA           `json:"gira"`
	Phases         []GIRASemanticPhase         `json:"phases"`
	IndicatorLinks []GIRASemanticIndicatorLink `json:"indicator_links"`
	Uncertainties  []GIRASemanticUncertainty   `json:"uncertainties"`
}
type GIRASemanticTarget struct {
	Ref            string   `json:"ref"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	TargetType     string   `json:"target_type"`
	EvidenceRefs   []string `json:"evidence_refs"`
	HypothesisRefs []string `json:"hypothesis_refs"`
	EventRefs      []string `json:"event_refs"`
}
type GIRASemanticGoal struct {
	Ref         string   `json:"ref"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	GoalType    string   `json:"goal_type"`
	Priority    string   `json:"priority"`
	TargetRefs  []string `json:"target_refs"`
}
type GIRASemanticIndicator struct {
	Ref               string  `json:"ref"`
	GoalRef           string  `json:"goal_ref"`
	Description       string  `json:"description"`
	IndicatorType     string  `json:"indicator_type"`
	MeasurementMethod *string `json:"measurement_method"`
	Baseline          *string `json:"baseline"`
	TargetValue       *string `json:"target_value"`
}
type GIRASemanticRationale struct {
	Ref              string   `json:"ref"`
	TargetRef        string   `json:"target_ref"`
	GoalRef          string   `json:"goal_ref"`
	ApproachSlug     string   `json:"approach_slug"`
	ApproachVersion  int      `json:"approach_version"`
	TechniqueSlug    *string  `json:"technique_slug"`
	TechniqueVersion *int     `json:"technique_version"`
	Rationale        string   `json:"rationale"`
	ExpectedEffect   string   `json:"expected_effect"`
	EvidenceRefs     []string `json:"evidence_refs"`
	HypothesisRefs   []string `json:"hypothesis_refs"`
}
type GIRASemanticGIRA struct {
	Ref               string   `json:"ref"`
	Title             string   `json:"title"`
	Summary           string   `json:"summary"`
	SupersedesGIRARef *string  `json:"supersedes_gira_ref"`
	TargetRefs        []string `json:"target_refs"`
	GoalRefs          []string `json:"goal_refs"`
	RationaleRefs     []string `json:"rationale_refs"`
}
type GIRASemanticPhase struct {
	Ref           string   `json:"ref"`
	GIRARef       string   `json:"gira_ref"`
	Position      int      `json:"position"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	EntryCriteria *string  `json:"entry_criteria"`
	ExitCriteria  *string  `json:"exit_criteria"`
	GoalRefs      []string `json:"goal_refs"`
	RationaleRefs []string `json:"rationale_refs"`
	IndicatorRefs []string `json:"indicator_refs"`
}
type GIRASemanticIndicatorLink struct {
	IndicatorRef string   `json:"indicator_ref"`
	SourceType   string   `json:"source_type"`
	SourceRef    string   `json:"source_ref"`
	RelationType string   `json:"relation_type"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type GIRASemanticUncertainty struct {
	Type         string   `json:"type"`
	Question     string   `json:"question"`
	EvidenceRefs []string `json:"evidence_refs"`
}

func BuildGIRAGenerationRequest(snapshot GIRABuilderInput) GIRAGenerationRequest {
	snapshot = BoundGIRAContext(snapshot)
	canonicalizeGIRAStrategy(&snapshot.CurrentStrategy)
	refs := giraRefIndex{evidence: map[string]uuid.UUID{}, events: map[string]uuid.UUID{}, hypotheses: map[string]uuid.UUID{}, targets: map[string]uuid.UUID{}, goals: map[string]uuid.UUID{}, indicators: map[string]uuid.UUID{}, rationales: map[string]uuid.UUID{}, giras: map[string]uuid.UUID{}, phases: map[string]uuid.UUID{}, versions: map[string]int{}}
	model := GIRAModelInput{SchemaVersion: GIRAPromptVersion, Process: GIRAProcessContext{Title: snapshot.SelectedProcess.Title, Description: snapshot.SelectedProcess.Description, ClinicalStatus: snapshot.SelectedProcess.ClinicalStatus, Version: snapshot.SelectedProcess.Version}, Evidence: []GIRAEvidenceContext{}, Events: []GIRAEventContext{}, Hypotheses: []GIRAHypothesisContext{}, CurrentStrategy: GIRAStrategyContext{Targets: []GIRATargetContext{}, Goals: []GIRAGoalContext{}, Indicators: []GIRAIndicatorContext{}, Rationales: []GIRARationaleContext{}, GIRAs: []GIRAGIRAContext{}}, ApproachCandidates: []GIRAApproachContext{}, TechniqueCandidates: []GIRATechniqueContext{}}
	for i, e := range snapshot.ApprovedEvidence {
		ref := fmt.Sprintf("evidence_%d", i+1)
		refs.evidence[ref] = e.ID
		model.Evidence = append(model.Evidence, GIRAEvidenceContext{Ref: ref, EpistemicType: e.EpistemicType, Statement: e.Statement, Version: e.Version})
	}
	for i, e := range snapshot.ApprovedEvents {
		ref := fmt.Sprintf("event_%d", i+1)
		refs.events[ref] = e.ID
		model.Events = append(model.Events, GIRAEventContext{Ref: ref, EventType: e.EventType, Title: e.Title, Description: e.Description, Version: e.Version})
	}
	for i, h := range snapshot.ApprovedHypotheses {
		ref := fmt.Sprintf("hypothesis_%d", i+1)
		refs.hypotheses[ref] = h.ID
		model.Hypotheses = append(model.Hypotheses, GIRAHypothesisContext{Ref: ref, Statement: h.Statement, ConfidenceLevel: h.ConfidenceLevel, ClinicalStatus: h.ClinicalStatus, Version: h.Version})
	}
	for i, t := range snapshot.CurrentStrategy.Targets {
		ref := fmt.Sprintf("existing_target_%d", i+1)
		refs.targets[ref] = t.ID
		refs.versions[ref] = t.Version
		model.CurrentStrategy.Targets = append(model.CurrentStrategy.Targets, GIRATargetContext{Ref: ref, Title: t.Title, Description: t.Description, TargetType: t.TargetType, ClinicalStatus: t.ClinicalStatus, Version: t.Version})
	}
	for i, g := range snapshot.CurrentStrategy.Goals {
		ref := fmt.Sprintf("existing_goal_%d", i+1)
		refs.goals[ref] = g.ID
		refs.versions[ref] = g.Version
		targetRefs := refsForIDs(refs.targets, g.TargetIDs)
		model.CurrentStrategy.Goals = append(model.CurrentStrategy.Goals, GIRAGoalContext{Ref: ref, Title: g.Title, Description: g.Description, GoalType: g.GoalType, Priority: g.Priority, ClinicalStatus: g.ClinicalStatus, Version: g.Version, TargetRefs: targetRefs})
		for j, indicator := range g.Indicators {
			iref := fmt.Sprintf("existing_indicator_%d_%d", i+1, j+1)
			refs.indicators[iref] = indicator.ID
			refs.versions[iref] = indicator.Version
			model.CurrentStrategy.Indicators = append(model.CurrentStrategy.Indicators, GIRAIndicatorContext{Ref: iref, GoalRef: ref, Description: indicator.Description, IndicatorType: indicator.IndicatorType, Status: indicator.Status, Version: indicator.Version})
		}
	}
	for i, r := range snapshot.CurrentStrategy.Rationales {
		ref := fmt.Sprintf("existing_rationale_%d", i+1)
		refs.rationales[ref] = r.ID
		refs.versions[ref] = r.Version
		model.CurrentStrategy.Rationales = append(model.CurrentStrategy.Rationales, GIRARationaleContext{Ref: ref, TargetRef: refForID(refs.targets, r.TargetID), GoalRef: refForID(refs.goals, r.GoalID), ApproachSlug: r.ApproachSlug, ApproachVersion: r.ApproachVersion, TechniqueSlug: r.TechniqueSlug, TechniqueVersion: r.TechniqueVersion, Rationale: r.Rationale, ExpectedEffect: r.ExpectedEffect, Version: r.Version})
	}
	for i, g := range snapshot.CurrentStrategy.GIRAs {
		ref := fmt.Sprintf("existing_gira_%d", i+1)
		refs.giras[ref] = g.ID
		refs.versions[ref] = g.EntityVersion
		rationaleIDs := make([]uuid.UUID, 0, len(g.Rationales))
		for _, r := range g.Rationales {
			rationaleIDs = append(rationaleIDs, r.ID)
		}
		model.CurrentStrategy.GIRAs = append(model.CurrentStrategy.GIRAs, GIRAGIRAContext{Ref: ref, Title: g.Title, Summary: g.Summary, ClinicalStatus: g.ClinicalStatus, GIRAVersion: g.GIRAVersion, EntityVersion: g.EntityVersion, TargetRefs: refsForIDs(refs.targets, g.TargetIDs), GoalRefs: refsForIDs(refs.goals, g.GoalIDs), RationaleRefs: refsForIDs(refs.rationales, rationaleIDs)})
		for j, p := range g.Phases {
			pref := fmt.Sprintf("existing_phase_%d_%d", i+1, j+1)
			refs.phases[pref] = p.ID
			refs.versions[pref] = p.Version
		}
	}
	approaches, techniques := selectRegistryCandidates(snapshot)
	for _, a := range approaches {
		model.ApproachCandidates = append(model.ApproachCandidates, GIRAApproachContext{Slug: a.Slug, Version: a.Version, Name: a.Name, TargetDomains: a.TargetDomains, CoreMechanisms: a.CoreMechanisms, Limitations: a.Limitations, Cautions: a.Cautions})
	}
	for _, t := range techniques {
		model.TechniqueCandidates = append(model.TechniqueCandidates, GIRATechniqueContext{Slug: t.Slug, Version: t.Version, Name: t.Name, ApproachSlug: t.ApproachSlug, ApproachVersion: t.ApproachVersion, TargetDomains: t.TargetDomains, Mechanism: t.Mechanism, Cautions: t.Cautions, Limits: t.Limits, ExpectedSignals: t.ExpectedSignals})
	}
	remote, privacy := BuildRemoteGIRAContext(model, snapshot, refs)
	return GIRAGenerationRequest{Snapshot: snapshot, ModelInput: model, RemoteContext: remote, Privacy: privacy, refs: refs}
}

func canonicalizeGIRAStrategy(strategy *TherapeuticStrategy) {
	sort.SliceStable(strategy.Targets, func(i, j int) bool { return strategy.Targets[i].ID.String() < strategy.Targets[j].ID.String() })
	sort.SliceStable(strategy.Goals, func(i, j int) bool { return strategy.Goals[i].ID.String() < strategy.Goals[j].ID.String() })
	for i := range strategy.Goals {
		sort.SliceStable(strategy.Goals[i].Indicators, func(a, b int) bool {
			return strategy.Goals[i].Indicators[a].ID.String() < strategy.Goals[i].Indicators[b].ID.String()
		})
	}
	sort.SliceStable(strategy.Rationales, func(i, j int) bool { return strategy.Rationales[i].ID.String() < strategy.Rationales[j].ID.String() })
	sort.SliceStable(strategy.GIRAs, func(i, j int) bool {
		if strategy.GIRAs[i].GIRAVersion != strategy.GIRAs[j].GIRAVersion {
			return strategy.GIRAs[i].GIRAVersion < strategy.GIRAs[j].GIRAVersion
		}
		return strategy.GIRAs[i].ID.String() < strategy.GIRAs[j].ID.String()
	})
	for i := range strategy.GIRAs {
		sort.SliceStable(strategy.GIRAs[i].Phases, func(a, b int) bool {
			if strategy.GIRAs[i].Phases[a].Position != strategy.GIRAs[i].Phases[b].Position {
				return strategy.GIRAs[i].Phases[a].Position < strategy.GIRAs[i].Phases[b].Position
			}
			return strategy.GIRAs[i].Phases[a].ID.String() < strategy.GIRAs[i].Phases[b].ID.String()
		})
	}
}

func selectRegistryCandidates(snapshot GIRABuilderInput) ([]ApproachDefinition, []TechniqueDefinition) {
	active := map[string]bool{}
	used := map[string]bool{}
	domains := map[string]bool{}
	for _, t := range snapshot.CurrentStrategy.Targets {
		if t.TargetType != "" {
			domains[t.TargetType] = true
		}
	}
	for _, r := range snapshot.CurrentStrategy.Rationales {
		used[fmt.Sprintf("%s/%d", r.ApproachSlug, r.ApproachVersion)] = true
	}
	approaches := []ApproachDefinition{}
	for _, a := range snapshot.ApproachRegistry {
		key := fmt.Sprintf("%s/%d", a.Slug, a.Version)
		match := len(domains) == 0 || used[key]
		for _, d := range a.TargetDomains {
			match = match || domains[d]
		}
		if used[key] || ((a.Status == "" || a.Status == "active") && match) {
			approaches = append(approaches, a)
			active[key] = true
		}
	}
	sort.SliceStable(approaches, func(i, j int) bool {
		ki, kj := fmt.Sprintf("%s/%d", approaches[i].Slug, approaches[i].Version), fmt.Sprintf("%s/%d", approaches[j].Slug, approaches[j].Version)
		if used[ki] != used[kj] {
			return used[ki]
		}
		return ki < kj
	})
	if len(approaches) > giraApproachLimit {
		approaches = approaches[:giraApproachLimit]
	}
	active = map[string]bool{}
	for _, approach := range approaches {
		active[fmt.Sprintf("%s/%d", approach.Slug, approach.Version)] = true
	}
	techniques := []TechniqueDefinition{}
	for _, t := range snapshot.TechniqueRegistry {
		if active[fmt.Sprintf("%s/%d", t.ApproachSlug, t.ApproachVersion)] && (t.Status == "" || t.Status == "active" || used[fmt.Sprintf("%s/%d", t.ApproachSlug, t.ApproachVersion)]) {
			techniques = append(techniques, t)
		}
	}
	sort.SliceStable(techniques, func(i, j int) bool {
		ai, aj := fmt.Sprintf("%s/%d", techniques[i].ApproachSlug, techniques[i].ApproachVersion), fmt.Sprintf("%s/%d", techniques[j].ApproachSlug, techniques[j].ApproachVersion)
		if used[ai] != used[aj] {
			return used[ai]
		}
		ki, kj := ai+"/"+techniques[i].Slug+fmt.Sprint(techniques[i].Version), aj+"/"+techniques[j].Slug+fmt.Sprint(techniques[j].Version)
		return ki < kj
	})
	if len(techniques) > giraTechniqueLimit {
		techniques = techniques[:giraTechniqueLimit]
	}
	return approaches, techniques
}

func DecodeGIRASemanticProposal(raw []byte) (GIRASemanticProposal, error) {
	var out GIRASemanticProposal
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return out, domainerrors.NewValidation("invalid semantic GIRA proposal: " + err.Error())
	}
	for _, required := range []string{"targets", "goals", "indicators", "rationales", "gira", "phases", "indicator_links", "uncertainties"} {
		value, ok := envelope[required]
		if !ok || (required != "gira" && string(value) == "null") {
			return out, domainerrors.NewValidation("invalid semantic GIRA proposal: missing required field " + required)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, domainerrors.NewValidation("invalid semantic GIRA proposal: " + err.Error())
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return out, domainerrors.NewValidation("multiple JSON values")
		}
		return out, domainerrors.NewValidation("invalid trailing JSON: " + err.Error())
	}
	return out, nil
}

var semanticRefPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func CompileGIRASemanticProposal(req GIRAGenerationRequest, proposal GIRASemanticProposal, newID func() uuid.UUID) (InterpreterResult, error) {
	if newID == nil {
		newID = uuid.New
	}
	refs := req.refs
	refs.versions = make(map[string]int, len(req.refs.versions))
	for ref, version := range req.refs.versions {
		refs.versions[ref] = version
	}
	copyMap := func(in map[string]uuid.UUID) map[string]uuid.UUID {
		out := map[string]uuid.UUID{}
		for k, v := range in {
			out[k] = v
		}
		return out
	}
	targets, goals, indicators, rationales, giras := copyMap(refs.targets), copyMap(refs.goals), copyMap(refs.indicators), copyMap(refs.rationales), copyMap(refs.giras)
	seenNew := map[string]bool{}
	reserve := func(ref string, set map[string]uuid.UUID) (uuid.UUID, error) {
		if !semanticRefPattern.MatchString(ref) || strings.HasPrefix(ref, "existing_") || seenNew[ref] {
			return uuid.Nil, domainerrors.NewValidation("invalid or duplicate temporary ref: " + ref)
		}
		seenNew[ref] = true
		id := newID()
		set[ref] = id
		return id, nil
	}
	resolve := func(ref string, set map[string]uuid.UUID, kind string) (uuid.UUID, error) {
		id, ok := set[ref]
		if !ok {
			return uuid.Nil, domainerrors.NewValidation("unknown " + kind + " ref: " + ref)
		}
		return id, nil
	}
	resolveMany := func(items []string, set map[string]uuid.UUID, kind string) ([]uuid.UUID, error) {
		out := make([]uuid.UUID, 0, len(items))
		seen := map[uuid.UUID]bool{}
		for _, ref := range items {
			id, err := resolve(ref, set, kind)
			if err != nil {
				return nil, err
			}
			if seen[id] {
				return nil, domainerrors.NewValidation("duplicate " + kind + " ref")
			}
			seen[id] = true
			out = append(out, id)
		}
		return out, nil
	}
	ops := []ProposedOperation{}
	appendOp := func(ref, kind string, id uuid.UUID, expected *int, payload any) {
		raw, _ := json.Marshal(payload)
		ops = append(ops, ProposedOperation{ID: ref, OperationType: kind, TargetEntityID: &id, ExpectedEntityVersion: expected, Proposal: raw})
	}
	// The model expresses dependencies with refs, but never controls persistence
	// order. Sorting inside each topological layer makes compilation repeatable.
	sort.Slice(proposal.Targets, func(i, j int) bool { return proposal.Targets[i].Ref < proposal.Targets[j].Ref })
	sort.Slice(proposal.Goals, func(i, j int) bool { return proposal.Goals[i].Ref < proposal.Goals[j].Ref })
	sort.Slice(proposal.Indicators, func(i, j int) bool { return proposal.Indicators[i].Ref < proposal.Indicators[j].Ref })
	sort.Slice(proposal.Rationales, func(i, j int) bool { return proposal.Rationales[i].Ref < proposal.Rationales[j].Ref })
	sort.Slice(proposal.Phases, func(i, j int) bool {
		if proposal.Phases[i].Position != proposal.Phases[j].Position {
			return proposal.Phases[i].Position < proposal.Phases[j].Position
		}
		return proposal.Phases[i].Ref < proposal.Phases[j].Ref
	})
	sort.Slice(proposal.IndicatorLinks, func(i, j int) bool {
		if proposal.IndicatorLinks[i].IndicatorRef != proposal.IndicatorLinks[j].IndicatorRef {
			return proposal.IndicatorLinks[i].IndicatorRef < proposal.IndicatorLinks[j].IndicatorRef
		}
		return proposal.IndicatorLinks[i].SourceRef < proposal.IndicatorLinks[j].SourceRef
	})
	for _, x := range proposal.Targets {
		id, err := reserve(x.Ref, targets)
		if err != nil {
			return InterpreterResult{}, err
		}
		es, err := resolveMany(x.EvidenceRefs, refs.evidence, "evidence")
		if err != nil {
			return InterpreterResult{}, err
		}
		hs, err := resolveMany(x.HypothesisRefs, refs.hypotheses, "hypothesis")
		if err != nil {
			return InterpreterResult{}, err
		}
		evs, err := resolveMany(x.EventRefs, refs.events, "event")
		if err != nil {
			return InterpreterResult{}, err
		}
		appendOp(x.Ref, "create_target", id, nil, CreateTargetProposal{ProcessID: req.Snapshot.SelectedProcess.ID, Title: x.Title, Description: x.Description, TargetType: x.TargetType, EvidenceIDs: es, HypothesisIDs: hs, EventIDs: evs})
	}
	for _, x := range proposal.Goals {
		id, err := reserve(x.Ref, goals)
		if err != nil {
			return InterpreterResult{}, err
		}
		ts, err := resolveMany(x.TargetRefs, targets, "target")
		if err != nil {
			return InterpreterResult{}, err
		}
		appendOp(x.Ref, "create_goal", id, nil, CreateGoalProposal{ProcessID: req.Snapshot.SelectedProcess.ID, Title: x.Title, Description: x.Description, GoalType: x.GoalType, Priority: x.Priority, TargetIDs: ts})
	}
	for _, x := range proposal.Indicators {
		id, err := reserve(x.Ref, indicators)
		if err != nil {
			return InterpreterResult{}, err
		}
		goal, err := resolve(x.GoalRef, goals, "goal")
		if err != nil {
			return InterpreterResult{}, err
		}
		appendOp(x.Ref, "create_goal_indicator", id, nil, CreateGoalIndicatorProposal{GoalID: goal, Description: x.Description, IndicatorType: x.IndicatorType, MeasurementMethod: x.MeasurementMethod, Baseline: x.Baseline, TargetValue: x.TargetValue})
	}
	for _, x := range proposal.Rationales {
		id, err := reserve(x.Ref, rationales)
		if err != nil {
			return InterpreterResult{}, err
		}
		target, err := resolve(x.TargetRef, targets, "target")
		if err != nil {
			return InterpreterResult{}, err
		}
		goal, err := resolve(x.GoalRef, goals, "goal")
		if err != nil {
			return InterpreterResult{}, err
		}
		es, err := resolveMany(x.EvidenceRefs, refs.evidence, "evidence")
		if err != nil {
			return InterpreterResult{}, err
		}
		hs, err := resolveMany(x.HypothesisRefs, refs.hypotheses, "hypothesis")
		if err != nil {
			return InterpreterResult{}, err
		}
		appendOp(x.Ref, "create_therapeutic_rationale", id, nil, CreateTherapeuticRationaleProposal{ProcessID: req.Snapshot.SelectedProcess.ID, TargetID: target, GoalID: goal, ApproachSlug: x.ApproachSlug, ApproachVersion: x.ApproachVersion, TechniqueSlug: x.TechniqueSlug, TechniqueVersion: x.TechniqueVersion, Rationale: x.Rationale, ExpectedEffect: x.ExpectedEffect, GroundingStatus: "grounded", EvidenceIDs: es, HypothesisIDs: hs})
	}
	if proposal.GIRA != nil {
		x := proposal.GIRA
		id, err := reserve(x.Ref, giras)
		if err != nil {
			return InterpreterResult{}, err
		}
		ts, err := resolveMany(x.TargetRefs, targets, "target")
		if err != nil {
			return InterpreterResult{}, err
		}
		gs, err := resolveMany(x.GoalRefs, goals, "goal")
		if err != nil {
			return InterpreterResult{}, err
		}
		rs, err := resolveMany(x.RationaleRefs, rationales, "rationale")
		if err != nil {
			return InterpreterResult{}, err
		}
		version := 1
		var supersedes *uuid.UUID
		if x.SupersedesGIRARef != nil {
			old, err := resolve(*x.SupersedesGIRARef, refs.giras, "existing GIRA")
			if err != nil {
				return InterpreterResult{}, err
			}
			supersedes = &old
			for _, g := range req.Snapshot.CurrentStrategy.GIRAs {
				if g.ID == old {
					version = g.GIRAVersion + 1
				}
			}
		} else if len(req.Snapshot.CurrentStrategy.GIRAs) > 0 {
			return InterpreterResult{}, domainerrors.NewValidation("new GIRA must select the existing version it supersedes")
		}
		appendOp(x.Ref, "create_gira", id, nil, CreateGIRAProposal{ProcessID: req.Snapshot.SelectedProcess.ID, GIRAVersion: version, Title: x.Title, Summary: x.Summary, SupersedesGIRAID: supersedes, TargetIDs: ts, GoalIDs: gs, RationaleIDs: rs})
	}
	for _, x := range proposal.Phases {
		id, err := reserve(x.Ref, map[string]uuid.UUID{})
		if err != nil {
			return InterpreterResult{}, err
		}
		gira, err := resolve(x.GIRARef, giras, "GIRA")
		if err != nil {
			return InterpreterResult{}, err
		}
		gs, err := resolveMany(x.GoalRefs, goals, "goal")
		if err != nil {
			return InterpreterResult{}, err
		}
		rs, err := resolveMany(x.RationaleRefs, rationales, "rationale")
		if err != nil {
			return InterpreterResult{}, err
		}
		is, err := resolveMany(x.IndicatorRefs, indicators, "indicator")
		if err != nil {
			return InterpreterResult{}, err
		}
		appendOp(x.Ref, "create_gira_phase", id, nil, CreateGIRAPhaseProposal{GIRAID: gira, Position: x.Position, Title: x.Title, Description: x.Description, EntryCriteria: x.EntryCriteria, ExitCriteria: x.ExitCriteria, GoalIDs: gs, RationaleIDs: rs, IndicatorIDs: is})
	}
	for i, x := range proposal.IndicatorLinks {
		indicator, err := resolve(x.IndicatorRef, indicators, "indicator")
		if err != nil {
			return InterpreterResult{}, err
		}
		sourceSet, kind := refs.evidence, "link_indicator_evidence"
		if x.SourceType == "event" {
			sourceSet, kind = refs.events, "link_indicator_event"
		} else if x.SourceType != "evidence" {
			return InterpreterResult{}, domainerrors.NewValidation("invalid indicator source type")
		}
		source, err := resolve(x.SourceRef, sourceSet, x.SourceType)
		if err != nil {
			return InterpreterResult{}, err
		}
		evidenceIDs, err := resolveMany(x.EvidenceRefs, refs.evidence, "evidence")
		if err != nil {
			return InterpreterResult{}, err
		}
		version := refs.versions[x.IndicatorRef]
		if version == 0 {
			version = 1
		}
		appendOp(fmt.Sprintf("indicator_link_%d", i+1), kind, indicator, &version, LinkIndicatorSourceProposal{IndicatorID: indicator, SourceID: source, RelationType: x.RelationType, EvidenceIDs: evidenceIDs})
		refs.versions[x.IndicatorRef] = version + 1
	}
	uncertainties := make([]Uncertainty, 0, len(proposal.Uncertainties))
	for _, u := range proposal.Uncertainties {
		ids, err := resolveMany(u.EvidenceRefs, refs.evidence, "evidence")
		if err != nil {
			return InterpreterResult{}, err
		}
		uncertainties = append(uncertainties, Uncertainty{Type: u.Type, Question: u.Question, EvidenceIDs: ids})
	}
	result := InterpreterResult{Operations: ops, Uncertainties: uncertainties}
	if err := ValidateGIRABuilderResult(req.Snapshot, result); err != nil {
		return InterpreterResult{}, err
	}
	return result, nil
}

func refsForIDs(refs map[string]uuid.UUID, ids []uuid.UUID) []string {
	out := []string{}
	for _, id := range ids {
		if ref := refForID(refs, id); ref != "" {
			out = append(out, ref)
		}
	}
	sort.Strings(out)
	return out
}
func refForID(refs map[string]uuid.UUID, id uuid.UUID) string {
	for ref, candidate := range refs {
		if candidate == id {
			return ref
		}
	}
	return ""
}
