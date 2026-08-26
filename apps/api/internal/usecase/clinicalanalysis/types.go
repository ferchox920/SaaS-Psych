package clinicalanalysis

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const PromptVersion = "clinical-live-v1"
const ReviewPromptVersion = "clinical-review-v1"

type EpistemicLevel string

const (
	EpistemicFact       EpistemicLevel = "fact"
	EpistemicInference  EpistemicLevel = "inference"
	EpistemicHypothesis EpistemicLevel = "hypothesis"
)

type TrafficLight string

const (
	TrafficGreen  TrafficLight = "green"
	TrafficYellow TrafficLight = "yellow"
	TrafficRed    TrafficLight = "red"
)

type NowAction string

const (
	NowListen         NowAction = "listen"
	NowClarify        NowAction = "clarify"
	NowReflect        NowAction = "reflect"
	NowExplore        NowAction = "explore"
	NowConfront       NowAction = "confront"
	NowRestructure    NowAction = "restructure"
	NowResignify      NowAction = "resignify"
	NowValues         NowAction = "values"
	NowRegulate       NowAction = "regulate"
	NowNoIntervention NowAction = "no_intervention"
)

type Evidence struct {
	SourceID string         `json:"source_id"`
	Kind     EpistemicLevel `json:"kind"`
	Summary  string         `json:"summary"`
}

type Hypothesis struct {
	Text           string         `json:"text"`
	EpistemicLevel EpistemicLevel `json:"epistemic_level"`
	TrafficLight   TrafficLight   `json:"traffic_light"`
}

type Risk struct {
	Detected                bool    `json:"detected"`
	Category                *string `json:"category"`
	RequiresHumanAssessment bool    `json:"requires_human_assessment"`
}

type Result struct {
	Mode                   string     `json:"mode"`
	Node                   string     `json:"node"`
	Hypothesis             Hypothesis `json:"hypothesis"`
	Evidence               []Evidence `json:"evidence"`
	Now                    NowAction  `json:"now"`
	Caution                string     `json:"caution"`
	SuggestedInterventions []string   `json:"suggested_interventions"`
	TherapistMeta          *string    `json:"therapist_meta"`
	Risk                   Risk       `json:"risk"`
}

type ContextItem struct {
	SourceID string         `json:"source_id"`
	Kind     EpistemicLevel `json:"kind"`
	Summary  string         `json:"summary"`
}

type LiveRequest struct {
	Fragment             string        `json:"fragment"`
	ApprovedSummary      string        `json:"approved_summary,omitempty"`
	RelevantContext      []ContextItem `json:"relevant_context,omitempty"`
	PreviousIntervention string        `json:"previous_intervention,omitempty"`
	PreviousAction       NowAction     `json:"previous_action,omitempty"`
	PatientResponse      string        `json:"patient_response,omitempty"`
}

type ReviewRequest struct {
	SessionText     string        `json:"session_text"`
	ApprovedSummary string        `json:"approved_summary,omitempty"`
	RelevantContext []ContextItem `json:"relevant_context,omitempty"`
}

type ReviewResult struct {
	Mode                     string     `json:"mode"`
	EmergingFormulation      string     `json:"emerging_formulation"`
	EffectiveInterventions   []string   `json:"effective_interventions"`
	WeakOrRiskyInterventions []string   `json:"weak_or_risky_interventions"`
	HypothesisCalibration    string     `json:"hypothesis_calibration"`
	PatientResponses         string     `json:"patient_responses"`
	Alliance                 string     `json:"alliance"`
	TherapistPatterns        []string   `json:"therapist_patterns"`
	NextFocus                []string   `json:"next_focus"`
	Evidence                 []Evidence `json:"evidence"`
	Caution                  string     `json:"caution"`
	Risk                     Risk       `json:"risk"`
}

type ReviewOutput struct {
	Result        ReviewResult      `json:"result"`
	Metrics       GenerationMetrics `json:"metrics"`
	PromptVersion string            `json:"prompt_version"`
	SuggestionID  *uuid.UUID        `json:"suggestion_id,omitempty"`
	RiskProtocol  *string           `json:"risk_protocol,omitempty"`
	AIRunID       *uuid.UUID        `json:"ai_run_id,omitempty"`
}

type ModelInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size,omitempty"`
	Loaded     bool   `json:"loaded"`
	Configured bool   `json:"configured"`
}

type ProviderStatus struct {
	Available       bool        `json:"available"`
	Busy            bool        `json:"busy"`
	ConfiguredModel string      `json:"configured_model"`
	Models          []ModelInfo `json:"models,omitempty"`
	Message         string      `json:"message,omitempty"`
}

type GenerationProgress struct {
	GeneratedCharacters int           `json:"generated_characters"`
	Elapsed             time.Duration `json:"elapsed"`
}

type GenerationMetrics struct {
	FirstToken time.Duration `json:"first_token"`
	Total      time.Duration `json:"total"`
	EvalCount  int           `json:"eval_count"`
	EvalRate   float64       `json:"eval_tokens_per_second"`
	Repaired   bool          `json:"repaired"`
}

type ProviderOutput struct {
	JSON    []byte
	Metrics GenerationMetrics
}

type ClinicalInferenceProvider interface {
	Health(ctx context.Context) (ProviderStatus, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
	WarmModel(ctx context.Context) error
	AnalyzeLive(ctx context.Context, systemPrompt string, input LiveRequest, onProgress func(GenerationProgress)) (ProviderOutput, error)
	ReviewSession(ctx context.Context, systemPrompt string, input ReviewRequest, onProgress func(GenerationProgress)) (ProviderOutput, error)
	UnloadModel(ctx context.Context) error
}
