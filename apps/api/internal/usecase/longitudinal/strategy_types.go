package longitudinal

import (
	"time"

	"github.com/google/uuid"
)

type ApproachDefinition struct {
	ID                   uuid.UUID `json:"id"`
	Slug                 string    `json:"slug"`
	Version              int       `json:"version"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	TargetDomains        []string  `json:"target_domains"`
	CoreMechanisms       []string  `json:"core_mechanisms"`
	InterventionFamilies []string  `json:"intervention_families"`
	ProgressSignals      []string  `json:"progress_signals"`
	Limitations          []string  `json:"limitations"`
	Cautions             []string  `json:"cautions"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type TechniqueDefinition struct {
	ID              uuid.UUID `json:"id"`
	Slug            string    `json:"slug"`
	Version         int       `json:"version"`
	Name            string    `json:"name"`
	ApproachSlug    string    `json:"approach_slug"`
	ApproachVersion int       `json:"approach_version"`
	Description     string    `json:"description"`
	TargetDomains   []string  `json:"target_domains"`
	Mechanism       string    `json:"mechanism"`
	Indications     []string  `json:"indications"`
	Cautions        []string  `json:"cautions"`
	Limits          []string  `json:"limits"`
	ExpectedSignals []string  `json:"expected_signals"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Target struct {
	ID                 uuid.UUID   `json:"id"`
	TenantID           uuid.UUID   `json:"tenant_id"`
	ClientID           uuid.UUID   `json:"client_id"`
	ProcessID          uuid.UUID   `json:"process_id"`
	Title              string      `json:"title"`
	Description        string      `json:"description"`
	TargetType         string      `json:"target_type"`
	ApprovalStatus     string      `json:"approval_status"`
	ClinicalStatus     string      `json:"clinical_status"`
	Version            int         `json:"version"`
	CreatedByUserID    *uuid.UUID  `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID *uuid.UUID  `json:"created_from_ai_run_id,omitempty"`
	ApprovedByUserID   *uuid.UUID  `json:"approved_by_user_id,omitempty"`
	ApprovedAt         *time.Time  `json:"approved_at,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	EvidenceIDs        []uuid.UUID `json:"evidence_ids"`
	HypothesisIDs      []uuid.UUID `json:"hypothesis_ids"`
	EventIDs           []uuid.UUID `json:"event_ids"`
}

type GoalIndicatorLink struct {
	SourceID     uuid.UUID `json:"source_id"`
	SourceType   string    `json:"source_type"`
	RelationType string    `json:"relation_type"`
}

type GoalIndicator struct {
	ID                 uuid.UUID           `json:"id"`
	TenantID           uuid.UUID           `json:"tenant_id"`
	ClientID           uuid.UUID           `json:"client_id"`
	GoalID             uuid.UUID           `json:"goal_id"`
	Description        string              `json:"description"`
	IndicatorType      string              `json:"indicator_type"`
	MeasurementMethod  *string             `json:"measurement_method,omitempty"`
	Baseline           *string             `json:"baseline,omitempty"`
	TargetValue        *string             `json:"target_value,omitempty"`
	Status             string              `json:"status"`
	Version            int                 `json:"version"`
	CreatedByUserID    *uuid.UUID          `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID *uuid.UUID          `json:"created_from_ai_run_id,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
	Links              []GoalIndicatorLink `json:"links"`
}

type Goal struct {
	ID                 uuid.UUID       `json:"id"`
	TenantID           uuid.UUID       `json:"tenant_id"`
	ClientID           uuid.UUID       `json:"client_id"`
	ProcessID          uuid.UUID       `json:"process_id"`
	Title              string          `json:"title"`
	Description        string          `json:"description"`
	GoalType           string          `json:"goal_type"`
	Priority           string          `json:"priority"`
	ApprovalStatus     string          `json:"approval_status"`
	ClinicalStatus     string          `json:"clinical_status"`
	Version            int             `json:"version"`
	CreatedByUserID    *uuid.UUID      `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID *uuid.UUID      `json:"created_from_ai_run_id,omitempty"`
	ApprovedByUserID   *uuid.UUID      `json:"approved_by_user_id,omitempty"`
	ApprovedAt         *time.Time      `json:"approved_at,omitempty"`
	ActivatedAt        *time.Time      `json:"activated_at,omitempty"`
	AchievedAt         *time.Time      `json:"achieved_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	TargetIDs          []uuid.UUID     `json:"target_ids"`
	Indicators         []GoalIndicator `json:"indicators"`
}

type TherapeuticRationale struct {
	ID                 uuid.UUID   `json:"id"`
	TenantID           uuid.UUID   `json:"tenant_id"`
	ClientID           uuid.UUID   `json:"client_id"`
	ProcessID          uuid.UUID   `json:"process_id"`
	TargetID           uuid.UUID   `json:"target_id"`
	GoalID             uuid.UUID   `json:"goal_id"`
	ApproachSlug       string      `json:"approach_slug"`
	ApproachVersion    int         `json:"approach_version"`
	TechniqueSlug      *string     `json:"technique_slug,omitempty"`
	TechniqueVersion   *int        `json:"technique_version,omitempty"`
	Rationale          string      `json:"rationale"`
	ExpectedEffect     string      `json:"expected_effect"`
	GroundingStatus    string      `json:"grounding_status"`
	ApprovalStatus     string      `json:"approval_status"`
	Version            int         `json:"version"`
	CreatedByUserID    *uuid.UUID  `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID *uuid.UUID  `json:"created_from_ai_run_id,omitempty"`
	ApprovedByUserID   *uuid.UUID  `json:"approved_by_user_id,omitempty"`
	ApprovedAt         *time.Time  `json:"approved_at,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	EvidenceIDs        []uuid.UUID `json:"evidence_ids"`
	HypothesisIDs      []uuid.UUID `json:"hypothesis_ids"`
}

type GIRAPhase struct {
	ID                 uuid.UUID   `json:"id"`
	TenantID           uuid.UUID   `json:"tenant_id"`
	ClientID           uuid.UUID   `json:"client_id"`
	GIRAID             uuid.UUID   `json:"gira_id"`
	Position           int         `json:"position"`
	Title              string      `json:"title"`
	Description        string      `json:"description"`
	ClinicalStatus     string      `json:"clinical_status"`
	EntryCriteria      *string     `json:"entry_criteria,omitempty"`
	ExitCriteria       *string     `json:"exit_criteria,omitempty"`
	Version            int         `json:"version"`
	CreatedByUserID    *uuid.UUID  `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID *uuid.UUID  `json:"created_from_ai_run_id,omitempty"`
	ActivatedAt        *time.Time  `json:"activated_at,omitempty"`
	CompletedAt        *time.Time  `json:"completed_at,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	GoalIDs            []uuid.UUID `json:"goal_ids"`
	RationaleIDs       []uuid.UUID `json:"rationale_ids"`
	IndicatorIDs       []uuid.UUID `json:"indicator_ids"`
}

type GIRA struct {
	ID                 uuid.UUID              `json:"id"`
	TenantID           uuid.UUID              `json:"tenant_id"`
	ClientID           uuid.UUID              `json:"client_id"`
	ProcessID          uuid.UUID              `json:"process_id"`
	GIRAVersion        int                    `json:"gira_version"`
	Title              string                 `json:"title"`
	Summary            string                 `json:"summary"`
	ApprovalStatus     string                 `json:"approval_status"`
	ClinicalStatus     string                 `json:"clinical_status"`
	EntityVersion      int                    `json:"entity_version"`
	CreatedByUserID    *uuid.UUID             `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID *uuid.UUID             `json:"created_from_ai_run_id,omitempty"`
	ApprovedByUserID   *uuid.UUID             `json:"approved_by_user_id,omitempty"`
	ApprovedAt         *time.Time             `json:"approved_at,omitempty"`
	ActivatedAt        *time.Time             `json:"activated_at,omitempty"`
	CompletedAt        *time.Time             `json:"completed_at,omitempty"`
	SupersedesGIRAID   *uuid.UUID             `json:"supersedes_gira_id,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	TargetIDs          []uuid.UUID            `json:"target_ids"`
	GoalIDs            []uuid.UUID            `json:"goal_ids"`
	Rationales         []TherapeuticRationale `json:"rationales"`
	Phases             []GIRAPhase            `json:"phases"`
}

type TherapeuticStrategy struct {
	Targets    []Target               `json:"targets"`
	Goals      []Goal                 `json:"goals"`
	Rationales []TherapeuticRationale `json:"rationales"`
	GIRAs      []GIRA                 `json:"giras"`
}

type StrategyHistory struct {
	EntityType  string              `json:"entity_type"`
	EntityID    uuid.UUID           `json:"entity_id"`
	Transitions []HistoryTransition `json:"transitions"`
}

type CreateTargetProposal struct {
	ProcessID     uuid.UUID   `json:"process_id"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	TargetType    string      `json:"target_type"`
	EvidenceIDs   []uuid.UUID `json:"evidence_ids"`
	HypothesisIDs []uuid.UUID `json:"hypothesis_ids"`
	EventIDs      []uuid.UUID `json:"event_ids"`
}
type UpdateTargetProposal struct {
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	TargetType    string      `json:"target_type"`
	EvidenceIDs   []uuid.UUID `json:"evidence_ids"`
	HypothesisIDs []uuid.UUID `json:"hypothesis_ids"`
	EventIDs      []uuid.UUID `json:"event_ids"`
}
type TransitionTargetProposal struct {
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
}
type CreateGoalProposal struct {
	ProcessID   uuid.UUID   `json:"process_id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	GoalType    string      `json:"goal_type"`
	Priority    string      `json:"priority"`
	TargetIDs   []uuid.UUID `json:"target_ids"`
}
type UpdateGoalProposal struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	GoalType    string      `json:"goal_type"`
	Priority    string      `json:"priority"`
	TargetIDs   []uuid.UUID `json:"target_ids"`
}
type TransitionGoalProposal struct {
	EvidenceIDs  []uuid.UUID `json:"evidence_ids"`
	IndicatorIDs []uuid.UUID `json:"indicator_ids"`
}
type CreateGoalIndicatorProposal struct {
	GoalID            uuid.UUID `json:"goal_id"`
	Description       string    `json:"description"`
	IndicatorType     string    `json:"indicator_type"`
	MeasurementMethod *string   `json:"measurement_method"`
	Baseline          *string   `json:"baseline"`
	TargetValue       *string   `json:"target_value"`
}
type UpdateGoalIndicatorProposal struct {
	Description       string  `json:"description"`
	IndicatorType     string  `json:"indicator_type"`
	MeasurementMethod *string `json:"measurement_method"`
	Baseline          *string `json:"baseline"`
	TargetValue       *string `json:"target_value"`
	Status            string  `json:"status"`
}
type LinkIndicatorSourceProposal struct {
	IndicatorID  uuid.UUID   `json:"indicator_id"`
	SourceID     uuid.UUID   `json:"source_id"`
	RelationType string      `json:"relation_type"`
	EvidenceIDs  []uuid.UUID `json:"evidence_ids"`
}
type CreateTherapeuticRationaleProposal struct {
	ProcessID        uuid.UUID   `json:"process_id"`
	TargetID         uuid.UUID   `json:"target_id"`
	GoalID           uuid.UUID   `json:"goal_id"`
	ApproachSlug     string      `json:"approach_slug"`
	ApproachVersion  int         `json:"approach_version"`
	TechniqueSlug    *string     `json:"technique_slug"`
	TechniqueVersion *int        `json:"technique_version"`
	Rationale        string      `json:"rationale"`
	ExpectedEffect   string      `json:"expected_effect"`
	GroundingStatus  string      `json:"grounding_status"`
	EvidenceIDs      []uuid.UUID `json:"evidence_ids"`
	HypothesisIDs    []uuid.UUID `json:"hypothesis_ids"`
}
type UpdateTherapeuticRationaleProposal = CreateTherapeuticRationaleProposal
type CreateGIRAProposal struct {
	ProcessID        uuid.UUID   `json:"process_id"`
	GIRAVersion      int         `json:"gira_version"`
	Title            string      `json:"title"`
	Summary          string      `json:"summary"`
	SupersedesGIRAID *uuid.UUID  `json:"supersedes_gira_id"`
	TargetIDs        []uuid.UUID `json:"target_ids"`
	GoalIDs          []uuid.UUID `json:"goal_ids"`
	RationaleIDs     []uuid.UUID `json:"rationale_ids"`
}
type TransitionGIRAProposal struct {
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
}
type CreateGIRAPhaseProposal struct {
	GIRAID        uuid.UUID   `json:"gira_id"`
	Position      int         `json:"position"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	EntryCriteria *string     `json:"entry_criteria"`
	ExitCriteria  *string     `json:"exit_criteria"`
	GoalIDs       []uuid.UUID `json:"goal_ids"`
	RationaleIDs  []uuid.UUID `json:"rationale_ids"`
	IndicatorIDs  []uuid.UUID `json:"indicator_ids"`
}
type UpdateGIRAPhaseProposal struct {
	Position      int         `json:"position"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	EntryCriteria *string     `json:"entry_criteria"`
	ExitCriteria  *string     `json:"exit_criteria"`
	GoalIDs       []uuid.UUID `json:"goal_ids"`
	RationaleIDs  []uuid.UUID `json:"rationale_ids"`
	IndicatorIDs  []uuid.UUID `json:"indicator_ids"`
}
type TransitionGIRAPhaseProposal struct {
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
}
