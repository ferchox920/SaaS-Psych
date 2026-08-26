package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
	longitudinal "sessionflow/apps/api/internal/usecase/longitudinal"
	sessionreport "sessionflow/apps/api/internal/usecase/sessionreport"
)

type Config struct {
	BaseURL               string
	Model                 string
	ContextTokens         int
	Temperature           float64
	Timeout               time.Duration
	KeepAlive             string
	MaxOutputTokens       int
	ReviewContextTokens   int
	ReviewTemperature     float64
	ReviewTimeout         time.Duration
	ReviewMaxOutputTokens int
	ReviewThink           bool
}

func (p *Provider) GenerateSessionReport(ctx context.Context, systemPrompt string, input []byte, schema map[string]any) (sessionreport.ProviderOutput, error) {
	if !p.acquire() {
		return sessionreport.ProviderOutput{}, clinicalanalysis.ErrProviderBusy
	}
	defer p.release()
	raw, metrics, err := p.generate(ctx, []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: string(input)}}, schema, true, nil)
	if err != nil {
		return sessionreport.ProviderOutput{}, err
	}
	if json.Valid(raw) {
		return sessionreport.ProviderOutput{JSON: raw, Duration: metrics.Total}, nil
	}
	repairPrompt := "Repara el siguiente contenido para que sea exclusivamente JSON válido según el esquema. No agregues explicación ni datos nuevos:\n" + string(raw)
	repaired, repairMetrics, err := p.generate(ctx, []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: repairPrompt}}, schema, true, nil)
	if err != nil {
		return sessionreport.ProviderOutput{}, err
	}
	if !json.Valid(repaired) {
		return sessionreport.ProviderOutput{}, clinicalanalysis.ErrInvalidModelOutput
	}
	return sessionreport.ProviderOutput{JSON: repaired, Duration: metrics.Total + repairMetrics.Total, Repaired: true}, nil
}

type Provider struct {
	baseURL      *url.URL
	config       Config
	client       *http.Client
	gate         chan struct{}
	busy         atomic.Bool
	reviewMu     sync.Mutex
	reviewCancel context.CancelFunc
}

func NewProvider(config Config) (*Provider, error) {
	parsed, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse Ollama base URL: %w", err)
	}
	if !isLoopbackURL(parsed) {
		return nil, errors.New("Ollama base URL must use an explicit loopback host")
	}
	if config.Timeout <= 0 {
		config.Timeout = 45 * time.Second
	}
	if config.ReviewContextTokens <= 0 {
		config.ReviewContextTokens = 8192
	}
	if config.ReviewTemperature == 0 {
		config.ReviewTemperature = 0.15
	}
	if config.ReviewTimeout <= 0 {
		config.ReviewTimeout = 180 * time.Second
	}
	if config.ReviewMaxOutputTokens <= 0 {
		config.ReviewMaxOutputTokens = 1024
	}
	return &Provider{
		baseURL: parsed,
		config:  config,
		client:  &http.Client{},
		gate:    make(chan struct{}, 1),
	}, nil
}

func isLoopbackURL(parsed *url.URL) bool {
	if parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (p *Provider) Health(ctx context.Context) (clinicalanalysis.ProviderStatus, error) {
	models, err := p.listModels(ctx)
	if err != nil {
		return clinicalanalysis.ProviderStatus{
			Available:       false,
			Busy:            p.busy.Load(),
			ConfiguredModel: p.config.Model,
			Message:         "Ollama local no está disponible",
		}, fmt.Errorf("%w: %v", clinicalanalysis.ErrProviderUnavailable, err)
	}
	return clinicalanalysis.ProviderStatus{
		Available:       true,
		Busy:            p.busy.Load(),
		ConfiguredModel: p.config.Model,
		Models:          models,
	}, nil
}

func (p *Provider) ListModels(ctx context.Context) ([]clinicalanalysis.ModelInfo, error) {
	return p.listModels(ctx)
}

func (p *Provider) listModels(ctx context.Context) ([]clinicalanalysis.ModelInfo, error) {
	var tags struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}
	if err := p.doJSON(ctx, http.MethodGet, "/api/tags", nil, &tags); err != nil {
		return nil, err
	}
	loaded := make(map[string]bool)
	var running struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := p.doJSON(ctx, http.MethodGet, "/api/ps", nil, &running); err == nil {
		for _, model := range running.Models {
			loaded[model.Name] = true
		}
	}
	models := make([]clinicalanalysis.ModelInfo, 0, len(tags.Models))
	for _, model := range tags.Models {
		models = append(models, clinicalanalysis.ModelInfo{
			Name:       model.Name,
			Size:       model.Size,
			Loaded:     loaded[model.Name],
			Configured: model.Name == p.config.Model,
		})
	}
	return models, nil
}

func (p *Provider) WarmModel(ctx context.Context) error {
	if !p.acquire() {
		return clinicalanalysis.ErrProviderBusy
	}
	defer p.release()
	request := map[string]any{
		"model":      p.config.Model,
		"prompt":     "",
		"stream":     false,
		"keep_alive": p.config.KeepAlive,
		"options":    map[string]any{"num_predict": 1, "num_ctx": p.config.ContextTokens},
	}
	var response map[string]any
	return p.doJSONWithTimeout(ctx, http.MethodPost, "/api/generate", request, &response)
}

func (p *Provider) UnloadModel(ctx context.Context) error {
	if !p.acquire() {
		return clinicalanalysis.ErrProviderBusy
	}
	defer p.release()
	request := map[string]any{"model": p.config.Model, "prompt": "", "stream": false, "keep_alive": 0}
	var response map[string]any
	return p.doJSONWithTimeout(ctx, http.MethodPost, "/api/generate", request, &response)
}

func (p *Provider) AnalyzeLive(ctx context.Context, systemPrompt string, input clinicalanalysis.LiveRequest, onProgress func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	p.cancelActiveReview()
	if !p.acquireLive(ctx) {
		return clinicalanalysis.ProviderOutput{}, clinicalanalysis.ErrProviderBusy
	}
	defer p.release()

	userJSON, err := json.Marshal(input)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, fmt.Errorf("encode local analysis input: %w", err)
	}
	messages := []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: string(userJSON)}}
	raw, metrics, err := p.generate(ctx, messages, clinicalanalysis.LiveJSONSchema(), false, onProgress)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	if validLiveStructuredOutput(raw) {
		return clinicalanalysis.ProviderOutput{JSON: raw, Metrics: metrics}, nil
	}

	repairPrompt := "Repara el siguiente contenido para que sea exclusivamente JSON válido según el esquema. No agregues explicación ni datos nuevos:\n" + string(raw)
	repaired, repairMetrics, err := p.generate(ctx, []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: repairPrompt}}, clinicalanalysis.LiveJSONSchema(), false, onProgress)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	if !validLiveStructuredOutput(repaired) {
		return clinicalanalysis.ProviderOutput{}, clinicalanalysis.ErrInvalidModelOutput
	}
	repairMetrics.Repaired = true
	repairMetrics.Total += metrics.Total
	if metrics.FirstToken > 0 {
		repairMetrics.FirstToken = metrics.FirstToken
	}
	return clinicalanalysis.ProviderOutput{JSON: repaired, Metrics: repairMetrics}, nil
}

func (p *Provider) ReviewSession(ctx context.Context, systemPrompt string, input clinicalanalysis.ReviewRequest, onProgress func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	if !p.acquire() {
		return clinicalanalysis.ProviderOutput{}, clinicalanalysis.ErrProviderBusy
	}
	reviewCtx, cancel := context.WithCancel(ctx)
	p.reviewMu.Lock()
	p.reviewCancel = cancel
	p.reviewMu.Unlock()
	defer func() {
		cancel()
		p.reviewMu.Lock()
		p.reviewCancel = nil
		p.reviewMu.Unlock()
		p.release()
	}()
	userJSON, err := json.Marshal(input)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, fmt.Errorf("encode local review input: %w", err)
	}
	messages := []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: string(userJSON)}}
	raw, metrics, err := p.generate(reviewCtx, messages, clinicalanalysis.ReviewJSONSchema(), true, onProgress)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	if validReviewStructuredOutput(raw) {
		return clinicalanalysis.ProviderOutput{JSON: raw, Metrics: metrics}, nil
	}
	repairPrompt := "Repara el siguiente contenido para que sea exclusivamente JSON válido según el esquema. No agregues explicación ni datos nuevos:\n" + string(raw)
	repaired, repairMetrics, err := p.generate(reviewCtx, []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: repairPrompt}}, clinicalanalysis.ReviewJSONSchema(), true, onProgress)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	if !validReviewStructuredOutput(repaired) {
		return clinicalanalysis.ProviderOutput{}, clinicalanalysis.ErrInvalidModelOutput
	}
	repairMetrics.Repaired = true
	repairMetrics.Total += metrics.Total
	if metrics.FirstToken > 0 {
		repairMetrics.FirstToken = metrics.FirstToken
	}
	return clinicalanalysis.ProviderOutput{JSON: repaired, Metrics: repairMetrics}, nil
}

func (p *Provider) InterpretLongitudinal(ctx context.Context, systemPrompt string, input longitudinal.InterpreterInput, onProgress func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
	if !p.acquire() {
		return clinicalanalysis.ProviderOutput{}, clinicalanalysis.ErrProviderBusy
	}
	backgroundCtx, cancel := context.WithCancel(ctx)
	p.reviewMu.Lock()
	p.reviewCancel = cancel
	p.reviewMu.Unlock()
	defer func() {
		cancel()
		p.reviewMu.Lock()
		p.reviewCancel = nil
		p.reviewMu.Unlock()
		p.release()
	}()
	userJSON, err := json.Marshal(input)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, fmt.Errorf("encode longitudinal input: %w", err)
	}
	messages := []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: string(userJSON)}}
	raw, metrics, err := p.generate(backgroundCtx, messages, longitudinal.JSONSchemaV1(), true, onProgress)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	if _, err = longitudinal.DecodeInterpreterResult(raw); err == nil {
		return clinicalanalysis.ProviderOutput{JSON: raw, Metrics: metrics}, nil
	}
	repairPrompt := "Repara el siguiente contenido para que sea exclusivamente JSON válido según el esquema. No agregues explicación ni datos nuevos:\n" + string(raw)
	repaired, repairMetrics, err := p.generate(backgroundCtx, []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: repairPrompt}}, longitudinal.JSONSchemaV1(), true, onProgress)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	if _, err = longitudinal.DecodeInterpreterResult(repaired); err != nil {
		return clinicalanalysis.ProviderOutput{}, clinicalanalysis.ErrInvalidModelOutput
	}
	repairMetrics.Repaired = true
	repairMetrics.Total += metrics.Total
	if metrics.FirstToken > 0 {
		repairMetrics.FirstToken = metrics.FirstToken
	}
	return clinicalanalysis.ProviderOutput{JSON: repaired, Metrics: repairMetrics}, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done         bool  `json:"done"`
	EvalCount    int   `json:"eval_count"`
	EvalDuration int64 `json:"eval_duration"`
}

func (p *Provider) generate(ctx context.Context, messages []chatMessage, schema map[string]any, review bool, onProgress func(clinicalanalysis.GenerationProgress)) ([]byte, clinicalanalysis.GenerationMetrics, error) {
	contextTokens, temperature, maxOutputTokens, timeout, think := p.config.ContextTokens, p.config.Temperature, p.config.MaxOutputTokens, p.config.Timeout, false
	if review {
		contextTokens, temperature, maxOutputTokens, timeout, think = p.config.ReviewContextTokens, p.config.ReviewTemperature, p.config.ReviewMaxOutputTokens, p.config.ReviewTimeout, p.config.ReviewThink
	}
	request := map[string]any{
		"model":      p.config.Model,
		"messages":   messages,
		"stream":     true,
		"think":      think,
		"format":     schema,
		"keep_alive": p.config.KeepAlive,
		"options": map[string]any{
			"temperature": temperature,
			"num_ctx":     contextTokens,
			"num_predict": maxOutputTokens,
		},
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, clinicalanalysis.GenerationMetrics{}, err
	}
	if timeout <= 0 {
		timeout = p.config.Timeout
	}
	timedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(timedCtx, http.MethodPost, p.endpoint("/api/chat"), bytes.NewReader(body))
	if err != nil {
		return nil, clinicalanalysis.GenerationMetrics{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := p.client.Do(httpRequest)
	if err != nil {
		return nil, clinicalanalysis.GenerationMetrics{}, normalizeRequestError(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, clinicalanalysis.GenerationMetrics{}, providerHTTPError(response)
	}

	var output strings.Builder
	metrics := clinicalanalysis.GenerationMetrics{}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		var chunk streamResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			return nil, metrics, fmt.Errorf("decode Ollama stream: %w", err)
		}
		if chunk.Message.Content != "" {
			if metrics.FirstToken == 0 {
				metrics.FirstToken = time.Since(started)
			}
			output.WriteString(chunk.Message.Content)
			if onProgress != nil {
				onProgress(clinicalanalysis.GenerationProgress{GeneratedCharacters: output.Len(), Elapsed: time.Since(started)})
			}
		}
		if chunk.Done {
			metrics.EvalCount = chunk.EvalCount
			if chunk.EvalDuration > 0 {
				metrics.EvalRate = float64(chunk.EvalCount) / (float64(chunk.EvalDuration) / float64(time.Second))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, metrics, normalizeRequestError(err)
	}
	metrics.Total = time.Since(started)
	if output.Len() == 0 {
		return nil, metrics, clinicalanalysis.ErrInvalidModelOutput
	}
	return []byte(output.String()), metrics, nil
}

func (p *Provider) doJSONWithTimeout(ctx context.Context, method, path string, requestBody, output any) error {
	timedCtx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()
	return p.doJSON(timedCtx, method, path, requestBody, output)
}

func (p *Provider) doJSON(ctx context.Context, method, path string, requestBody, output any) error {
	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, p.endpoint(path), body)
	if err != nil {
		return err
	}
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := p.client.Do(request)
	if err != nil {
		return normalizeRequestError(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return providerHTTPError(response)
	}
	return json.NewDecoder(response.Body).Decode(output)
}

func providerHTTPError(response *http.Response) error {
	limited, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := strings.ToLower(string(limited))
	if response.StatusCode == http.StatusNotFound || strings.Contains(message, "model") && strings.Contains(message, "not found") {
		return fmt.Errorf("%w: configured model is not installed", clinicalanalysis.ErrProviderUnavailable)
	}
	if strings.Contains(message, "memory") || strings.Contains(message, "cuda") {
		return fmt.Errorf("%w: insufficient local memory", clinicalanalysis.ErrProviderUnavailable)
	}
	return fmt.Errorf("%w: Ollama returned HTTP %d", clinicalanalysis.ErrProviderUnavailable, response.StatusCode)
}

func normalizeRequestError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %v", clinicalanalysis.ErrProviderUnavailable, err)
}

func validLiveStructuredOutput(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var result clinicalanalysis.Result
	if err := decoder.Decode(&result); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func validReviewStructuredOutput(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var result clinicalanalysis.ReviewResult
	if err := decoder.Decode(&result); err != nil {
		return false
	}
	return result.Mode == "review" && decoder.Decode(&struct{}{}) == io.EOF
}

func (p *Provider) endpoint(path string) string {
	return strings.TrimRight(p.baseURL.String(), "/") + path
}

func (p *Provider) acquire() bool {
	select {
	case p.gate <- struct{}{}:
		p.busy.Store(true)
		return true
	default:
		return false
	}
}

func (p *Provider) cancelActiveReview() {
	p.reviewMu.Lock()
	cancel := p.reviewCancel
	p.reviewMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (p *Provider) acquireLive(ctx context.Context) bool {
	deadline := time.NewTimer(750 * time.Millisecond)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if p.acquire() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
		}
	}
}

func (p *Provider) release() {
	p.busy.Store(false)
	<-p.gate
}
