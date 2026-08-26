package sessionreport

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	LegacySchemaVersion = "session-report-v1"
	SchemaVersion       = "session-report-v1.1"
)

type Fact struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
	Category  string `json:"category"`
}
type RelevantChange struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
type Intervention struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
}
type PatientResponse struct {
	ID           string `json:"id"`
	ResponseType string `json:"response_type"`
	Description  string `json:"description"`
}
type AffectiveNode struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}
type InferenceCandidate struct {
	ID           string   `json:"id"`
	Statement    string   `json:"statement"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type HypothesisCandidate struct {
	ID           string   `json:"id"`
	Statement    string   `json:"statement"`
	TrafficLight string   `json:"traffic_light"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type SafetySignal struct {
	ID                      string `json:"id"`
	Description             string `json:"description"`
	Category                string `json:"category"`
	RequiresHumanAssessment bool   `json:"requires_human_assessment"`
}
type OpenQuestion struct {
	ID       string `json:"id"`
	Question string `json:"question"`
}
type LongitudinalCandidate struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
	Rationale string `json:"rationale"`
}
type ReportV1 struct {
	SchemaVersion          string                  `json:"schema_version"`
	Summary                string                  `json:"summary"`
	Facts                  []Fact                  `json:"facts"`
	RelevantChanges        []RelevantChange        `json:"relevant_changes"`
	Interventions          []Intervention          `json:"interventions"`
	PatientResponses       []PatientResponse       `json:"patient_responses"`
	AffectiveNodes         []AffectiveNode         `json:"affective_nodes"`
	InferenceCandidates    []InferenceCandidate    `json:"inference_candidates"`
	HypothesisCandidates   []HypothesisCandidate   `json:"hypothesis_candidates"`
	SafetySignals          []SafetySignal          `json:"safety_signals"`
	OpenQuestions          []OpenQuestion          `json:"open_questions"`
	LongitudinalCandidates []LongitudinalCandidate `json:"longitudinal_candidates"`
}
type Report struct {
	ID                uuid.UUID       `json:"id"`
	TenantID          uuid.UUID       `json:"tenant_id"`
	ClinicalSessionID uuid.UUID       `json:"clinical_session_id"`
	Version           int             `json:"version"`
	Revision          int             `json:"revision"`
	SchemaVersion     string          `json:"schema_version"`
	Status            string          `json:"status"`
	ReportJSON        json.RawMessage `json:"report_json"`
	CreatedByUserID   uuid.UUID       `json:"created_by_user_id"`
	SourceAIRunID     *uuid.UUID      `json:"source_ai_run_id,omitempty"`
	ApprovedByUserID  *uuid.UUID      `json:"approved_by_user_id,omitempty"`
	ApprovedAt        *time.Time      `json:"approved_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}
type SessionDetails struct {
	ID, TenantID, ClientID, TherapistUserID uuid.UUID
	AppointmentID                           *uuid.UUID
	Status                                  string
}
type ProviderOutput struct {
	JSON     []byte
	Duration time.Duration
	Repaired bool
}
