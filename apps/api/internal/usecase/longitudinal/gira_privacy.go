package longitudinal

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const GIRAPrivacyPolicyVersion = "gira-remote-minimization-v1"

// RemoteGIRAContext is the complete allow-list for data allowed to cross the
// local GIRA privacy boundary. Identifiers, source provenance, audit data,
// reports and transcripts are deliberately not representable in this DTO.
type RemoteGIRAContext struct {
	SchemaVersion       string                        `json:"schema_version"`
	Process             RemoteGIRAProcessContext      `json:"process"`
	Evidence            []RemoteGIRAEvidenceContext   `json:"evidence"`
	Events              []RemoteGIRAEventContext      `json:"events"`
	Hypotheses          []RemoteGIRAHypothesisContext `json:"hypotheses"`
	CurrentStrategy     RemoteGIRAStrategyContext     `json:"current_strategy"`
	ApproachCandidates  []RemoteGIRAApproachContext   `json:"approach_candidates"`
	TechniqueCandidates []RemoteGIRATechniqueContext  `json:"technique_candidates"`
}

type RemoteGIRAProcessContext struct {
	Ref            string `json:"ref"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	ClinicalStatus string `json:"clinical_status"`
	Version        int    `json:"version"`
}

type RemoteGIRAEvidenceContext struct {
	Ref           string `json:"ref"`
	EpistemicType string `json:"epistemic_type"`
	Statement     string `json:"statement"`
	Version       int    `json:"version"`
}

type RemoteGIRAEventContext struct {
	Ref          string   `json:"ref"`
	EventType    string   `json:"event_type"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	EvidenceRefs []string `json:"evidence_refs"`
	Version      int      `json:"version"`
}

type RemoteGIRAHypothesisContext struct {
	Ref                       string   `json:"ref"`
	Statement                 string   `json:"statement"`
	ConfidenceLevel           string   `json:"confidence_level"`
	ClinicalStatus            string   `json:"clinical_status"`
	SupportingEvidenceRefs    []string `json:"supporting_evidence_refs"`
	ContradictingEvidenceRefs []string `json:"contradicting_evidence_refs"`
	Version                   int      `json:"version"`
}

type RemoteGIRATargetContext struct {
	Ref            string `json:"ref"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	TargetType     string `json:"target_type"`
	ClinicalStatus string `json:"clinical_status"`
	Version        int    `json:"version"`
}

type RemoteGIRAGoalContext struct {
	Ref            string   `json:"ref"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	GoalType       string   `json:"goal_type"`
	Priority       string   `json:"priority"`
	ClinicalStatus string   `json:"clinical_status"`
	Version        int      `json:"version"`
	TargetRefs     []string `json:"target_refs"`
}

type RemoteGIRAIndicatorContext struct {
	Ref           string `json:"ref"`
	GoalRef       string `json:"goal_ref"`
	Description   string `json:"description"`
	IndicatorType string `json:"indicator_type"`
	Status        string `json:"status"`
	Version       int    `json:"version"`
}

type RemoteGIRARationaleContext struct {
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

type RemoteGIRAGIRAContext struct {
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

type RemoteGIRAStrategyContext struct {
	Targets    []RemoteGIRATargetContext    `json:"targets"`
	Goals      []RemoteGIRAGoalContext      `json:"goals"`
	Indicators []RemoteGIRAIndicatorContext `json:"indicators"`
	Rationales []RemoteGIRARationaleContext `json:"rationales"`
	GIRAs      []RemoteGIRAGIRAContext      `json:"giras"`
}

type RemoteGIRAApproachContext struct {
	Slug           string   `json:"slug"`
	Version        int      `json:"version"`
	TargetDomains  []string `json:"target_domains"`
	CoreMechanisms []string `json:"core_mechanisms"`
	Limitations    []string `json:"limitations"`
	Cautions       []string `json:"cautions"`
}

type RemoteGIRATechniqueContext struct {
	Slug            string   `json:"slug"`
	Version         int      `json:"version"`
	ApproachSlug    string   `json:"approach_slug"`
	ApproachVersion int      `json:"approach_version"`
	TargetDomains   []string `json:"target_domains"`
	Mechanism       string   `json:"mechanism"`
	Cautions        []string `json:"cautions"`
	Limits          []string `json:"limits"`
	ExpectedSignals []string `json:"expected_signals"`
}

// GIRAPrivacySummary is safe for audit/metrics: it only contains counts,
// bytes, a policy version and closed-set risk flags.
type GIRAPrivacySummary struct {
	PolicyVersion     string         `json:"minimization_policy_version"`
	ResidualRiskFlags []string       `json:"residual_risk_flags"`
	FieldCounts       map[string]int `json:"field_counts"`
	PayloadBytes      int            `json:"payload_bytes"`
}

type privacyRule struct {
	pattern     *regexp.Regexp
	replacement string
	riskFlag    string
}

// GIRAPrivacyMinimizer performs deterministic data minimization. It is not an
// anonymizer and does not claim to eliminate narrative re-identification risk.
type GIRAPrivacyMinimizer struct {
	rules []privacyRule
}

func NewGIRAPrivacyMinimizer() GIRAPrivacyMinimizer {
	return GIRAPrivacyMinimizer{rules: []privacyRule{
		{regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`), "[email]", "direct_identifier_removed"},
		{regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`), "[internal-id]", "internal_identifier_removed"},
		{regexp.MustCompile(`(?i)\bTEST-DNI-[A-Z0-9-]+\b|\b(?:DNI|documento|document|pasaporte|passport)\s*[:#-]?\s*[A-Z0-9][A-Z0-9.\-]{3,}\b`), "[document]", "direct_identifier_removed"},
		{regexp.MustCompile(`(?i)\b\d{4}-\d{2}-\d{2}(?:[T ]\d{2}:\d{2}(?::\d{2})?(?:Z|[+-]\d{2}:?\d{2})?)?\b|\b\d{1,2}[/-]\d{1,2}[/-]\d{2,4}\b`), "[date]", "exact_date_present"},
		{regexp.MustCompile(`(?i)(?:\+?\d[\d ()-]{7,}\d)`), "[phone]", "direct_identifier_removed"},
		{regexp.MustCompile(`(?i)\b(?:calle|avenida|av\.?|pasaje|street|road|ruta)\s+[[:alnum:]ÁÉÍÓÚÑáéíóúñ .'-]{2,50}\s+\d{1,6}\b`), "[location]", "exact_address_removed"},
		{regexp.MustCompile(`\b(?i:hospital|clínica|clinica|escuela|colegio|universidad|empresa|instituto|fundación|fundacion)\s+[A-ZÁÉÍÓÚÑ][[:alnum:]ÁÉÍÓÚÑáéíóúñ&'-]*(?:\s+[A-ZÁÉÍÓÚÑ][[:alnum:]ÁÉÍÓÚÑáéíóúñ&'-]*){0,3}`), "[organization]", "specific_organization_removed"},
		{regexp.MustCompile(`\b(?i:barrio|vecindario|ciudad de|localidad de|municipio de)\s+[A-ZÁÉÍÓÚÑ][[:alpha:]ÁÉÍÓÚÑáéíóúñ'-]*(?:\s+[A-ZÁÉÍÓÚÑ][[:alpha:]ÁÉÍÓÚÑáéíóúñ'-]*){0,2}`), "[location]", "rare_location_context"},
		{regexp.MustCompile(`(?i)\b\d{1,3}\s+(?:años|years old)\b`), "[age band]", "specific_age_context"},
		{regexp.MustCompile(`(?i)\b(?:neurocirujan[oa] pediátric[oa]|astronauta|domador(?:a)? de leones|embalsamador(?:a)?|controlador(?:a)? aéreo)\b`), "[profession]", "rare_profession_context"},
		{regexp.MustCompile(`\b((?i:paciente|supervisor(?:a)?|pareja|hij[oa]|madre|padre|herman[oa]|compañer[oa]|colega|terapeuta|psicólog[oa]|psiquiatra|doctor(?:a)?|médic[oa]|profesor(?:a)?|maestr[oa]))\s+[A-ZÁÉÍÓÚÑ][a-záéíóúñ]+(?:\s+[A-ZÁÉÍÓÚÑ][a-záéíóúñ]+)+`), "$1 [person]", "third_party_identifier_removed"},
		{regexp.MustCompile(`\b((?i:de|con|a))\s+[A-ZÁÉÍÓÚÑ][a-záéíóúñ]+\s+[A-ZÁÉÍÓÚÑ][a-záéíóúñ]+\b`), "$1 [person]", "direct_identifier_removed"},
		{regexp.MustCompile(`^[A-ZÁÉÍÓÚÑ][a-záéíóúñ]{2,}\s+[A-ZÁÉÍÓÚÑ][a-záéíóúñ]{2,}\s+`), "[person] ", "direct_identifier_removed"},
	}}
}

type privacyCollector struct {
	minimizer GIRAPrivacyMinimizer
	flags     map[string]struct{}
	counts    map[string]int
}

func newPrivacyCollector() *privacyCollector {
	return &privacyCollector{minimizer: NewGIRAPrivacyMinimizer(), flags: map[string]struct{}{}, counts: map[string]int{}}
}

func (c *privacyCollector) text(kind, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	c.counts[kind]++
	if len([]rune(value)) > 480 {
		c.flags["high_specificity_narrative"] = struct{}{}
	}
	for _, rule := range c.minimizer.rules {
		if rule.pattern.MatchString(value) {
			value = rule.pattern.ReplaceAllString(value, rule.replacement)
			c.flags[rule.riskFlag] = struct{}{}
		}
	}
	return strings.Join(strings.Fields(value), " ")
}

func (c *privacyCollector) texts(kind string, values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, c.text(kind, value))
	}
	return out
}

func (c *privacyCollector) summary(context RemoteGIRAContext) GIRAPrivacySummary {
	flags := make([]string, 0, len(c.flags))
	for flag := range c.flags {
		flags = append(flags, flag)
	}
	sort.Strings(flags)
	raw, _ := json.Marshal(context)
	return GIRAPrivacySummary{PolicyVersion: GIRAPrivacyPolicyVersion, ResidualRiskFlags: flags, FieldCounts: c.counts, PayloadBytes: len(raw)}
}

func evidenceObjectRefs(items []Evidence, refs map[string]uuid.UUID) []string {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return refsForIDs(refs, ids)
}

// BuildRemoteGIRAContext applies the explicit outbound allow-list and
// deterministic minimization. The alias-to-UUID index remains in the caller.
func BuildRemoteGIRAContext(model GIRAModelInput, snapshot GIRABuilderInput, refs giraRefIndex) (RemoteGIRAContext, GIRAPrivacySummary) {
	c := newPrivacyCollector()
	out := RemoteGIRAContext{
		SchemaVersion: model.SchemaVersion,
		Process:       RemoteGIRAProcessContext{Ref: "process_1", Title: c.text("process_title", model.Process.Title), Description: c.text("process_description", model.Process.Description), ClinicalStatus: model.Process.ClinicalStatus, Version: model.Process.Version},
		Evidence:      []RemoteGIRAEvidenceContext{}, Events: []RemoteGIRAEventContext{}, Hypotheses: []RemoteGIRAHypothesisContext{},
		CurrentStrategy:    RemoteGIRAStrategyContext{Targets: []RemoteGIRATargetContext{}, Goals: []RemoteGIRAGoalContext{}, Indicators: []RemoteGIRAIndicatorContext{}, Rationales: []RemoteGIRARationaleContext{}, GIRAs: []RemoteGIRAGIRAContext{}},
		ApproachCandidates: []RemoteGIRAApproachContext{}, TechniqueCandidates: []RemoteGIRATechniqueContext{},
	}
	for _, item := range model.Evidence {
		out.Evidence = append(out.Evidence, RemoteGIRAEvidenceContext{Ref: item.Ref, EpistemicType: item.EpistemicType, Statement: c.text("evidence_statement", item.Statement), Version: item.Version})
	}
	for i, item := range model.Events {
		evidenceRefs := []string{}
		if i < len(snapshot.ApprovedEvents) {
			evidenceRefs = evidenceObjectRefs(snapshot.ApprovedEvents[i].Evidence, refs.evidence)
		}
		out.Events = append(out.Events, RemoteGIRAEventContext{Ref: item.Ref, EventType: item.EventType, Title: c.text("event_title", item.Title), Description: c.text("event_description", item.Description), EvidenceRefs: evidenceRefs, Version: item.Version})
	}
	for i, item := range model.Hypotheses {
		supporting, contradicting := []string{}, []string{}
		if i < len(snapshot.ApprovedHypotheses) {
			supporting = evidenceObjectRefs(snapshot.ApprovedHypotheses[i].SupportingEvidence, refs.evidence)
			contradicting = evidenceObjectRefs(snapshot.ApprovedHypotheses[i].ContradictingEvidence, refs.evidence)
		}
		out.Hypotheses = append(out.Hypotheses, RemoteGIRAHypothesisContext{Ref: item.Ref, Statement: c.text("hypothesis_statement", item.Statement), ConfidenceLevel: item.ConfidenceLevel, ClinicalStatus: item.ClinicalStatus, SupportingEvidenceRefs: supporting, ContradictingEvidenceRefs: contradicting, Version: item.Version})
	}
	for _, item := range model.CurrentStrategy.Targets {
		out.CurrentStrategy.Targets = append(out.CurrentStrategy.Targets, RemoteGIRATargetContext{Ref: item.Ref, Title: c.text("target_title", item.Title), Description: c.text("target_description", item.Description), TargetType: item.TargetType, ClinicalStatus: item.ClinicalStatus, Version: item.Version})
	}
	for _, item := range model.CurrentStrategy.Goals {
		out.CurrentStrategy.Goals = append(out.CurrentStrategy.Goals, RemoteGIRAGoalContext{Ref: item.Ref, Title: c.text("goal_title", item.Title), Description: c.text("goal_description", item.Description), GoalType: item.GoalType, Priority: item.Priority, ClinicalStatus: item.ClinicalStatus, Version: item.Version, TargetRefs: append([]string(nil), item.TargetRefs...)})
	}
	for _, item := range model.CurrentStrategy.Indicators {
		out.CurrentStrategy.Indicators = append(out.CurrentStrategy.Indicators, RemoteGIRAIndicatorContext{Ref: item.Ref, GoalRef: item.GoalRef, Description: c.text("indicator_description", item.Description), IndicatorType: item.IndicatorType, Status: item.Status, Version: item.Version})
	}
	for _, item := range model.CurrentStrategy.Rationales {
		out.CurrentStrategy.Rationales = append(out.CurrentStrategy.Rationales, RemoteGIRARationaleContext{Ref: item.Ref, TargetRef: item.TargetRef, GoalRef: item.GoalRef, ApproachSlug: item.ApproachSlug, ApproachVersion: item.ApproachVersion, TechniqueSlug: item.TechniqueSlug, TechniqueVersion: item.TechniqueVersion, Rationale: c.text("rationale", item.Rationale), ExpectedEffect: c.text("expected_effect", item.ExpectedEffect), Version: item.Version})
	}
	for _, item := range model.CurrentStrategy.GIRAs {
		out.CurrentStrategy.GIRAs = append(out.CurrentStrategy.GIRAs, RemoteGIRAGIRAContext{Ref: item.Ref, Title: c.text("gira_title", item.Title), Summary: c.text("gira_summary", item.Summary), ClinicalStatus: item.ClinicalStatus, GIRAVersion: item.GIRAVersion, EntityVersion: item.EntityVersion, TargetRefs: append([]string(nil), item.TargetRefs...), GoalRefs: append([]string(nil), item.GoalRefs...), RationaleRefs: append([]string(nil), item.RationaleRefs...)})
	}
	for _, item := range model.ApproachCandidates {
		out.ApproachCandidates = append(out.ApproachCandidates, RemoteGIRAApproachContext{Slug: item.Slug, Version: item.Version, TargetDomains: c.texts("registry_target_domain", item.TargetDomains), CoreMechanisms: c.texts("registry_mechanism", item.CoreMechanisms), Limitations: c.texts("registry_limitation", item.Limitations), Cautions: c.texts("registry_caution", item.Cautions)})
	}
	for _, item := range model.TechniqueCandidates {
		out.TechniqueCandidates = append(out.TechniqueCandidates, RemoteGIRATechniqueContext{Slug: item.Slug, Version: item.Version, ApproachSlug: item.ApproachSlug, ApproachVersion: item.ApproachVersion, TargetDomains: c.texts("registry_target_domain", item.TargetDomains), Mechanism: c.text("registry_mechanism", item.Mechanism), Cautions: c.texts("registry_caution", item.Cautions), Limits: c.texts("registry_limitation", item.Limits), ExpectedSignals: c.texts("registry_signal", item.ExpectedSignals)})
	}
	return out, c.summary(out)
}
