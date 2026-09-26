package clinicalanalysis

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type CodedProviderError interface {
	error
	ProviderErrorCode() string
}

func NormalizedProviderErrorCode(err error) string {
	var coded CodedProviderError
	if errors.As(err, &coded) {
		return coded.ProviderErrorCode()
	}
	return ""
}

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
	FirstToken      time.Duration `json:"first_token"`
	PrimaryDuration time.Duration `json:"primary_duration"`
	Total           time.Duration `json:"total"`
	RepairDuration  time.Duration `json:"repair_duration"`
	RepairReason    string        `json:"repair_reason,omitempty"`
	EvalCount       int           `json:"eval_count"`
	EvalRate        float64       `json:"eval_tokens_per_second"`
	Repaired        bool          `json:"repaired"`
}

type ProviderOutput struct {
	JSON     []byte
	Metrics  GenerationMetrics
	Metadata ProviderMetadata
}

// ProviderMetadata is a provider-neutral, non-clinical provenance envelope.
// It must never contain prompts, model output or clinical narrative.
type ProviderMetadata struct {
	RemoteRequestID     string `json:"remote_request_id,omitempty"`
	ProviderRegion      string `json:"provider_region,omitempty"`
	ModelSnapshot       string `json:"model_snapshot,omitempty"`
	InputTokens         int    `json:"input_tokens,omitempty"`
	OutputTokens        int    `json:"output_tokens,omitempty"`
	ReasoningTokens     int    `json:"reasoning_tokens,omitempty"`
	CacheTokens         int    `json:"cache_tokens,omitempty"`
	RepairAttempts      int    `json:"repair_attempts"`
	PriceConfigVersion  string `json:"price_config_version,omitempty"`
	EstimatedCostMicros int64  `json:"estimated_cost_micros,omitempty"`
}

func (m ProviderMetadata) SafeMap() map[string]any {
	out := map[string]any{"repair_attempts": m.RepairAttempts}
	if m.RemoteRequestID != "" {
		out["remote_request_id"] = m.RemoteRequestID
	}
	if m.ProviderRegion != "" {
		out["provider_region"] = m.ProviderRegion
	}
	if m.ModelSnapshot != "" {
		out["model_snapshot"] = m.ModelSnapshot
	}
	if m.InputTokens > 0 {
		out["input_tokens"] = m.InputTokens
	}
	if m.OutputTokens > 0 {
		out["output_tokens"] = m.OutputTokens
	}
	if m.ReasoningTokens > 0 {
		out["reasoning_tokens"] = m.ReasoningTokens
	}
	if m.CacheTokens > 0 {
		out["cache_tokens"] = m.CacheTokens
	}
	if m.PriceConfigVersion != "" {
		out["price_config_version"] = m.PriceConfigVersion
	}
	if m.EstimatedCostMicros > 0 {
		out["estimated_cost_micros"] = m.EstimatedCostMicros
	}
	return out
}

type ClinicalInferenceProvider interface {
	Health(ctx context.Context) (ProviderStatus, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
	WarmModel(ctx context.Context) error
	AnalyzeLive(ctx context.Context, systemPrompt string, input LiveRequest, onProgress func(GenerationProgress)) (ProviderOutput, error)
	ReviewSession(ctx context.Context, systemPrompt string, input ReviewRequest, onProgress func(GenerationProgress)) (ProviderOutput, error)
	UnloadModel(ctx context.Context) error
}
