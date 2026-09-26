package longitudinal

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	PromptName    = "clinical-longitudinal-interpreter"
	PromptVersion = "clinical-longitudinal-interpreter-v1"
)

type Evidence struct {
	ID                 uuid.UUID  `json:"id"`
	TenantID           uuid.UUID  `json:"tenant_id"`
	ClientID           uuid.UUID  `json:"client_id"`
	SourceType         string     `json:"source_type"`
	SourceID           uuid.UUID  `json:"source_id"`
	SourceVersion      int        `json:"source_version"`
	SourceItemID       string     `json:"source_item_id"`
	EpistemicType      string     `json:"epistemic_type"`
	Statement          string     `json:"statement"`
	Status             string     `json:"status"`
	Version            int        `json:"version"`
	CreatedByUserID    *uuid.UUID `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID *uuid.UUID `json:"created_from_ai_run_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type Event struct {
	ID                 uuid.UUID  `json:"id"`
	TenantID           uuid.UUID  `json:"tenant_id"`
	ClientID           uuid.UUID  `json:"client_id"`
	EventType          string     `json:"event_type"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	OccurredAt         *time.Time `json:"occurred_at,omitempty"`
	ObservedAt         time.Time  `json:"observed_at"`
	ApprovalStatus     string     `json:"approval_status"`
	Version            int        `json:"version"`
	CreatedByUserID    *uuid.UUID `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID *uuid.UUID `json:"created_from_ai_run_id,omitempty"`
	ApprovedByUserID   *uuid.UUID `json:"approved_by_user_id,omitempty"`
	ApprovedAt         *time.Time `json:"approved_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Evidence           []Evidence `json:"evidence"`
}

type Process struct {
	ID                  uuid.UUID            `json:"id"`
	TenantID            uuid.UUID            `json:"tenant_id"`
	ClientID            uuid.UUID            `json:"client_id"`
	Title               string               `json:"title"`
	Description         string               `json:"description"`
	ApprovalStatus      string               `json:"approval_status"`
	ClinicalStatus      string               `json:"clinical_status"`
	Version             int                  `json:"version"`
	CreatedByUserID     *uuid.UUID           `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID  *uuid.UUID           `json:"created_from_ai_run_id,omitempty"`
	ApprovedByUserID    *uuid.UUID           `json:"approved_by_user_id,omitempty"`
	ApprovedAt          *time.Time           `json:"approved_at,omitempty"`
	OpenedAt            *time.Time           `json:"opened_at,omitempty"`
	ClosedAt            *time.Time           `json:"closed_at,omitempty"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
	Events              []Event              `json:"events"`
	Hypotheses          []Hypothesis         `json:"hypotheses"`
	TherapeuticStrategy *TherapeuticStrategy `json:"therapeutic_strategy,omitempty"`
}

type Hypothesis struct {
	ID                    uuid.UUID  `json:"id"`
	TenantID              uuid.UUID  `json:"tenant_id"`
	ClientID              uuid.UUID  `json:"client_id"`
	ProcessID             *uuid.UUID `json:"process_id,omitempty"`
	Statement             string     `json:"statement"`
	HypothesisType        *string    `json:"hypothesis_type,omitempty"`
	ApprovalStatus        string     `json:"approval_status"`
	ClinicalStatus        string     `json:"clinical_status"`
	ConfidenceLevel       string     `json:"confidence_level"`
	Version               int        `json:"version"`
	CreatedByUserID       *uuid.UUID `json:"created_by_user_id,omitempty"`
	CreatedFromAIRunID    *uuid.UUID `json:"created_from_ai_run_id,omitempty"`
	ApprovedByUserID      *uuid.UUID `json:"approved_by_user_id,omitempty"`
	ApprovedAt            *time.Time `json:"approved_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	SupportingEvidence    []Evidence `json:"supporting_evidence"`
	ContradictingEvidence []Evidence `json:"contradicting_evidence"`
}

type Uncertainty struct {
	Type        string      `json:"type"`
	Question    string      `json:"question"`
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
}

type Operation struct {
	ID                    uuid.UUID       `json:"id"`
	TenantID              uuid.UUID       `json:"tenant_id"`
	ClientID              uuid.UUID       `json:"client_id"`
	DiffID                uuid.UUID       `json:"diff_id"`
	Sequence              int             `json:"sequence"`
	OperationType         string          `json:"operation_type"`
	TargetEntityID        uuid.UUID       `json:"target_entity_id"`
	ExpectedEntityVersion *int            `json:"expected_entity_version,omitempty"`
	OriginalProposal      json.RawMessage `json:"original_proposal"`
	HumanModification     json.RawMessage `json:"human_modification,omitempty"`
	ReviewStatus          string          `json:"review_status"`
	ReviewedByUserID      *uuid.UUID      `json:"reviewed_by_user_id,omitempty"`
	ReviewedAt            *time.Time      `json:"reviewed_at,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
}

type Diff struct {
	ID                       uuid.UUID     `json:"id"`
	TenantID                 uuid.UUID     `json:"tenant_id"`
	ClientID                 uuid.UUID     `json:"client_id"`
	ClinicalSessionID        *uuid.UUID    `json:"clinical_session_id,omitempty"`
	SourceSessionReportID    *uuid.UUID    `json:"source_session_report_id,omitempty"`
	SourceAIRunID            *uuid.UUID    `json:"source_ai_run_id,omitempty"`
	SourceExternalProposalID *uuid.UUID    `json:"source_external_proposal_id,omitempty"`
	Status                   string        `json:"status"`
	BaseStateVersion         int64         `json:"base_state_version"`
	Revision                 int           `json:"revision"`
	IsStale                  bool          `json:"is_stale"`
	Uncertainties            []Uncertainty `json:"uncertainties"`
	CreatedByUserID          *uuid.UUID    `json:"created_by_user_id,omitempty"`
	ReviewedByUserID         *uuid.UUID    `json:"reviewed_by_user_id,omitempty"`
	ReviewedAt               *time.Time    `json:"reviewed_at,omitempty"`
	MergedByUserID           *uuid.UUID    `json:"merged_by_user_id,omitempty"`
	MergedAt                 *time.Time    `json:"merged_at,omitempty"`
	MergedStateVersion       *int64        `json:"merged_state_version,omitempty"`
	CreatedAt                time.Time     `json:"created_at"`
	UpdatedAt                time.Time     `json:"updated_at"`
	Operations               []Operation   `json:"operations"`
}

type State struct {
	ClientID             uuid.UUID    `json:"client_id"`
	StateVersion         int64        `json:"state_version"`
	Processes            []Process    `json:"processes"`
	UnassignedHypotheses []Hypothesis `json:"unassigned_hypotheses"`
	RecentEvents         []Event      `json:"recent_events"`
	ActiveEvidence       []Evidence   `json:"active_evidence"`
	OpenProposals        []Diff       `json:"open_proposals"`
}

type HistoryTransition struct {
	ID                uuid.UUID       `json:"id"`
	DiffID            uuid.UUID       `json:"diff_id"`
	OperationID       uuid.UUID       `json:"operation_id"`
	EntityType        string          `json:"entity_type"`
	EntityID          uuid.UUID       `json:"entity_id"`
	Action            string          `json:"action"`
	FromVersion       *int            `json:"from_version,omitempty"`
	ToVersion         int             `json:"to_version"`
	FromStatus        *string         `json:"from_status,omitempty"`
	ToStatus          *string         `json:"to_status,omitempty"`
	ActorUserID       uuid.UUID       `json:"actor_user_id"`
	MergedByUserID    *uuid.UUID      `json:"merged_by_user_id,omitempty"`
	MergedAt          *time.Time      `json:"merged_at,omitempty"`
	OriginalProposal  json.RawMessage `json:"original_proposal"`
	HumanModification json.RawMessage `json:"human_modification,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

type ProcessHistory struct {
	Process     Process             `json:"process"`
	Transitions []HistoryTransition `json:"transitions"`
}

type HypothesisHistory struct {
	Hypothesis  Hypothesis          `json:"hypothesis"`
	Transitions []HistoryTransition `json:"transitions"`
}

type CreateEvidenceProposal struct {
	SourceItemID  string `json:"source_item_id"`
	EpistemicType string `json:"epistemic_type"`
}
type CreateEventProposal struct {
	EventType                string      `json:"event_type"`
	Title                    string      `json:"title"`
	Description              string      `json:"description"`
	OccurredAt               *time.Time  `json:"occurred_at"`
	ObservedAt               time.Time   `json:"observed_at"`
	EvidenceIDs              []uuid.UUID `json:"evidence_ids"`
	InterventionSourceItemID *string     `json:"intervention_source_item_id,omitempty"`
}
type CreateProcessProposal struct {
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	ClinicalStatus string      `json:"clinical_status"`
	EvidenceIDs    []uuid.UUID `json:"evidence_ids"`
}
type UpdateProcessProposal struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
}
type TransitionProcessProposal struct {
	EvidenceIDs  []uuid.UUID `json:"evidence_ids"`
	ReopenStatus string      `json:"reopen_status,omitempty"`
}
type LinkEventProcessProposal struct {
	EventID     uuid.UUID   `json:"event_id"`
	ProcessID   uuid.UUID   `json:"process_id"`
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
}
type CreateHypothesisProposal struct {
	ProcessID                *uuid.UUID  `json:"process_id"`
	Statement                string      `json:"statement"`
	HypothesisType           *string     `json:"hypothesis_type"`
	ConfidenceLevel          string      `json:"confidence_level"`
	SupportingEvidenceIDs    []uuid.UUID `json:"supporting_evidence_ids"`
	ContradictingEvidenceIDs []uuid.UUID `json:"contradicting_evidence_ids"`
}
type UpdateHypothesisProposal struct {
	ProcessID      *uuid.UUID  `json:"process_id"`
	Statement      string      `json:"statement"`
	HypothesisType *string     `json:"hypothesis_type"`
	EvidenceIDs    []uuid.UUID `json:"evidence_ids"`
}
type LinkHypothesisEvidenceProposal struct {
	HypothesisID uuid.UUID   `json:"hypothesis_id"`
	EvidenceID   uuid.UUID   `json:"evidence_id"`
	EvidenceIDs  []uuid.UUID `json:"evidence_ids"`
}
type TransitionHypothesisProposal struct {
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
}
type InvalidateEvidenceProposal struct {
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
}
type ApproveEventProposal struct {
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
}

type ProposedOperation struct {
	ID                    string          `json:"id"`
	OperationType         string          `json:"operation_type"`
	TargetEntityID        *uuid.UUID      `json:"target_entity_id"`
	ExpectedEntityVersion *int            `json:"expected_entity_version"`
	Proposal              json.RawMessage `json:"proposal"`
}
type InterpreterResult struct {
	Operations    []ProposedOperation `json:"operations"`
	Uncertainties []Uncertainty       `json:"uncertainties"`
}

type SessionAnalysis struct {
	SessionID     uuid.UUID
	ClientID      uuid.UUID
	AppointmentID *uuid.UUID
	Status        string
	ReportID      uuid.UUID
	ReportVersion int
	ReportJSON    json.RawMessage
}
type CreateDiffInput struct {
	ExternalProposalID                                      uuid.UUID
	TenantID, ClientID, SessionID, ReportID, RunID, ActorID uuid.UUID
	BaseStateVersion                                        int64
	Operations                                              []Operation
	Uncertainties                                           []Uncertainty
	OutputHash                                              string
	RunMetadata                                             map[string]any
}
type DecisionInput struct {
	TenantID, DiffID, OperationID, ActorID uuid.UUID
	ExpectedDiffRevision                   int
	Decision                               string
	Modification                           json.RawMessage
}
type MergeInput struct {
	TenantID, DiffID, ActorID uuid.UUID
	ExpectedDiffRevision      int
}

type AnalysisOutput struct {
	Diff   Diff `json:"diff"`
	Reused bool `json:"reused"`
}
