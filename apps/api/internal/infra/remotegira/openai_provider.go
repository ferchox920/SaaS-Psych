package remotegira

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"

	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
)

type PricingConfig struct {
	Version                  string
	InputUSDPerMillion       float64
	CachedInputUSDPerMillion float64
	OutputUSDPerMillion      float64
}

type OpenAIConfig struct {
	Model             string
	MaxOutputTokens   int
	ProviderRegion    string
	DataControlMode   string
	DataControlStatus string
	Pricing           PricingConfig
}

type OpenAIGIRABuilderProvider struct {
	transport RemoteLLMTransport
	config    OpenAIConfig
}

func NewOpenAIGIRABuilderProvider(transport RemoteLLMTransport, config OpenAIConfig) (*OpenAIGIRABuilderProvider, error) {
	if transport == nil {
		return nil, errors.New("OpenAI GIRA transport is required")
	}
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("OpenAI GIRA model is required")
	}
	if config.MaxOutputTokens <= 0 {
		return nil, errors.New("OpenAI GIRA max output tokens must be positive")
	}
	return &OpenAIGIRABuilderProvider{transport: transport, config: config}, nil
}

type openAIRequest struct {
	Model           string     `json:"model"`
	Instructions    string     `json:"instructions"`
	Input           string     `json:"input"`
	Text            openAIText `json:"text"`
	MaxOutputTokens int        `json:"max_output_tokens"`
	Store           bool       `json:"store"`
}

type openAIText struct {
	Format openAIFormat `json:"format"`
}

type openAIFormat struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type openAIResponse struct {
	ID         string `json:"id"`
	Model      string `json:"model"`
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		InputDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"input_tokens_details"`
		OutputDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
}

func (p *OpenAIGIRABuilderProvider) BuildGIRA(ctx context.Context, systemPrompt string, request longitudinal.GIRAProviderRequest, onProgress func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	contextJSON, err := json.Marshal(request.Context)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, &Error{Code: "request_serialization_failed", cause: err}
	}
	primary, err := p.generate(ctx, systemPrompt, string(contextJSON), request.Schema)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	primary.Metrics.PrimaryDuration = primary.Metrics.Total
	if onProgress != nil {
		onProgress(clinicalanalysis.GenerationProgress{GeneratedCharacters: len(primary.JSON), Elapsed: primary.Metrics.Total})
	}
	if err = request.ValidateCandidate(primary.JSON); err == nil {
		return primary, nil
	}
	validationClass := "invalid_semantic_output"
	if !json.Valid(primary.JSON) {
		validationClass = "invalid_json"
	}
	repairInput := "INPUT:\n" + string(contextJSON) + "\nPROPUESTA:\n" + string(primary.JSON) + "\nERROR_CLASS:\n" + validationClass + "\nCorrige solamente el JSON para cumplir el schema y las referencias del INPUT. No inventes evidencia. Devuelve el objeto completo."
	repaired, repairErr := p.generate(ctx, systemPrompt, repairInput, request.Schema)
	if repairErr != nil {
		return clinicalanalysis.ProviderOutput{}, repairErr
	}
	repaired.Metrics.Total += primary.Metrics.Total
	repaired.Metrics.PrimaryDuration = primary.Metrics.PrimaryDuration
	repaired.Metrics.RepairDuration = repaired.Metrics.Total - primary.Metrics.Total
	repaired.Metrics.RepairReason = validationClass
	repaired.Metrics.Repaired = true
	repaired.Metadata.InputTokens += primary.Metadata.InputTokens
	repaired.Metadata.OutputTokens += primary.Metadata.OutputTokens
	repaired.Metadata.ReasoningTokens += primary.Metadata.ReasoningTokens
	repaired.Metadata.CacheTokens += primary.Metadata.CacheTokens
	repaired.Metadata.RepairAttempts = 1
	repaired.Metadata.EstimatedCostMicros = p.estimatedCost(repaired.Metadata)
	if err = request.ValidateCandidate(repaired.JSON); err != nil {
		return clinicalanalysis.ProviderOutput{}, &Error{Code: "invalid_model_output"}
	}
	if onProgress != nil {
		onProgress(clinicalanalysis.GenerationProgress{GeneratedCharacters: len(repaired.JSON), Elapsed: repaired.Metrics.Total})
	}
	return repaired, nil
}

func (p *OpenAIGIRABuilderProvider) generate(ctx context.Context, systemPrompt, input string, schema map[string]any) (clinicalanalysis.ProviderOutput, error) {
	body, err := json.Marshal(openAIRequest{Model: p.config.Model, Instructions: systemPrompt, Input: input, Text: openAIText{Format: openAIFormat{Type: "json_schema", Name: "clinical_gira_semantic_v1", Strict: true, Schema: schema}}, MaxOutputTokens: p.config.MaxOutputTokens, Store: false})
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, &Error{Code: "request_serialization_failed", cause: err}
	}
	response, err := p.transport.Do(ctx, TransportRequest{Path: "/v1/responses", Body: body})
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	var decoded openAIResponse
	if err = json.Unmarshal(response.Body, &decoded); err != nil {
		return clinicalanalysis.ProviderOutput{}, &Error{Code: "invalid_provider_response"}
	}
	output := decoded.OutputText
	if output == "" {
		for _, item := range decoded.Output {
			for _, content := range item.Content {
				if content.Type == "output_text" && content.Text != "" {
					output += content.Text
				}
			}
		}
	}
	if strings.TrimSpace(output) == "" {
		return clinicalanalysis.ProviderOutput{}, &Error{Code: "empty_provider_output"}
	}
	metadata := clinicalanalysis.ProviderMetadata{RemoteRequestID: firstNonEmpty(response.RemoteRequestID, decoded.ID), ProviderRegion: p.config.ProviderRegion, ModelSnapshot: decoded.Model, InputTokens: decoded.Usage.InputTokens, OutputTokens: decoded.Usage.OutputTokens, ReasoningTokens: decoded.Usage.OutputDetails.ReasoningTokens, CacheTokens: decoded.Usage.InputDetails.CachedTokens, PriceConfigVersion: p.config.Pricing.Version}
	metadata.EstimatedCostMicros = p.estimatedCost(metadata)
	return clinicalanalysis.ProviderOutput{JSON: []byte(output), Metrics: clinicalanalysis.GenerationMetrics{Total: response.Duration}, Metadata: metadata}, nil
}

func (p *OpenAIGIRABuilderProvider) estimatedCost(metadata clinicalanalysis.ProviderMetadata) int64 {
	pricing := p.config.Pricing
	uncached := metadata.InputTokens - metadata.CacheTokens
	if uncached < 0 {
		uncached = 0
	}
	costMicros := float64(uncached)*pricing.InputUSDPerMillion + float64(metadata.CacheTokens)*pricing.CachedInputUSDPerMillion + float64(metadata.OutputTokens)*pricing.OutputUSDPerMillion
	return int64(math.Round(costMicros))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func SafeErrorMetadata(err error) map[string]any {
	metadata := map[string]any{"normalized_error_code": ErrorCode(err)}
	var remoteErr *Error
	if errors.As(err, &remoteErr) && remoteErr.RemoteRequestID != "" {
		metadata["remote_request_id"] = remoteErr.RemoteRequestID
	}
	return metadata
}

var _ longitudinal.GIRABuilderProvider = (*OpenAIGIRABuilderProvider)(nil)
