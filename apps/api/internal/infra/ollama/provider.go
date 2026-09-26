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
	GIRAContextTokens     int
	GIRATemperature       float64
	GIRATimeout           time.Duration
	GIRAMaxOutputTokens   int
	GIRAThink             bool
	GIRATopP              float64
	GIRATopK              int
	GIRASeed              int
}

// GIRAFailureDiagnostics contains only operational metadata and normalized
// validation messages. It intentionally excludes prompts and generated text.
type GIRAFailureDiagnostics struct {
	Classification  string
	FirstFailure    string
	FinalFailure    string
	InputBytes      int
	OutputBytes     int
	RepairAttempted bool
	Metrics         clinicalanalysis.GenerationMetrics
}

type GIRAOutputError struct {
	Diagnostics GIRAFailureDiagnostics
	Err         error
}

func (e *GIRAOutputError) Error() string { return e.Err.Error() }
func (e *GIRAOutputError) Unwrap() error { return e.Err }

func classifyGIRAFailure(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(message, "deadline exceeded"):
		return "TIMEOUT"
	case strings.Contains(message, "invalid semantic gira proposal"), strings.Contains(message, "json"):
		return "SCHEMA_MISMATCH"
	case strings.Contains(message, "unknown") && strings.Contains(message, "ref"):
		return "INVALID_DEPENDENCY"
	case strings.Contains(message, "technique"):
		return "INVALID_TECHNIQUE_VERSION"
	case strings.Contains(message, "approach"):
		return "INVALID_APPROACH_VERSION"
	case strings.Contains(message, "ground"), strings.Contains(message, "evidence"):
		return "CLINICAL_GROUNDING_FAILURE"
	case strings.Contains(message, "unsupported"):
		return "UNSUPPORTED_MECHANISM"
	default:
		return "OTHER"
	}
}

func (p *Provider) GenerateSessionReport(ctx context.Context, systemPrompt string, input []byte, schema map[string]any) (sessionreport.ProviderOutput, error) {
	if !p.acquire() {
		return sessionreport.ProviderOutput{}, clinicalanalysis.ErrProviderBusy
	}
	defer p.release()
	generationSchema := sessionReportGenerationSchema(schema)
	raw, metrics, err := p.generate(ctx, []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: string(input)}}, generationSchema, true, nil)
	if err != nil {
		return sessionreport.ProviderOutput{}, err
	}
	if _, validationErr := sessionreport.DecodeAndValidate(raw); validationErr == nil {
		return sessionreport.ProviderOutput{JSON: raw, Duration: metrics.Total}, nil
	}
	repairPrompt := "Repara el siguiente contenido para que sea exclusivamente JSON válido según el esquema. No agregues explicación ni datos nuevos:\n" + string(raw)
	repaired, repairMetrics, err := p.generate(ctx, []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: repairPrompt}}, generationSchema, true, nil)
	if err != nil {
		return sessionreport.ProviderOutput{}, err
	}
	if _, validationErr := sessionreport.DecodeAndValidate(repaired); validationErr != nil {
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
	if config.GIRAContextTokens <= 0 {
		config.GIRAContextTokens = config.ReviewContextTokens
	}
	if config.GIRATimeout <= 0 {
		config.GIRATimeout = config.ReviewTimeout
	}
	if config.GIRAMaxOutputTokens <= 0 {
		config.GIRAMaxOutputTokens = 1536
	}
	if config.GIRATopP <= 0 {
		config.GIRATopP = 0.8
	}
	if config.GIRATopK <= 0 {
		config.GIRATopK = 20
	}
	if config.GIRASeed == 0 {
		config.GIRASeed = 42
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
	longitudinalSchema := longitudinal.JSONSchemaForReport(input.SessionReport)
	raw, metrics, err := p.generate(backgroundCtx, messages, longitudinalSchema, true, onProgress)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, err
	}
	validate := func(candidate []byte) error {
		result, validationErr := longitudinal.DecodeInterpreterResult(candidate)
		if validationErr != nil {
			return validationErr
		}
		if validationErr := longitudinal.ValidateInterpreterSemantics(input.SessionReport, result); validationErr != nil {
			return validationErr
		}
		return longitudinal.ValidateInterpreterReferences(input, result)
	}
	if err = validate(raw); err == nil {
		return clinicalanalysis.ProviderOutput{JSON: raw, Metrics: metrics}, nil
	}
	candidate := raw
	combined := metrics
	for attempt := 0; attempt < 2; attempt++ {
		repairPrompt := "Repara el siguiente contenido para que sea exclusivamente JSON válido según el esquema y coherente con los UUID y versiones de la entrada original. No agregues explicación ni datos nuevos. Conserva sin cambios las operaciones válidas. Si una operación depende de un target UUID que la entrada propone crear explícitamente, agrega o restaura la operación create correspondiente antes de su primera dependencia. Si referencia una entidad ausente que la entrada no propone crear, elimina solamente esa operación y expresa la insuficiencia en uncertainties; nunca inventes otra referencia ni elimines otras operaciones válidas. Error de validación: " + err.Error() + "\nEntrada original:\n" + string(userJSON) + "\nContenido a reparar:\n" + string(candidate)
		repaired, repairMetrics, repairErr := p.generate(backgroundCtx, []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: repairPrompt}}, longitudinalSchema, true, onProgress)
		if repairErr != nil {
			return clinicalanalysis.ProviderOutput{}, repairErr
		}
		combined.Total += repairMetrics.Total
		combined.RepairDuration += repairMetrics.Total
		combined.EvalCount += repairMetrics.EvalCount
		combined.Repaired = true
		candidate = repaired
		if err = validate(candidate); err == nil {
			return clinicalanalysis.ProviderOutput{JSON: candidate, Metrics: combined}, nil
		}
	}
	return clinicalanalysis.ProviderOutput{}, fmt.Errorf("%w: %v", clinicalanalysis.ErrInvalidModelOutput, err)
}

func (p *Provider) BuildGIRA(ctx context.Context, systemPrompt string, request longitudinal.GIRAProviderRequest, onProgress func(clinicalanalysis.GenerationProgress)) (clinicalanalysis.ProviderOutput, error) {
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
	userJSON, err := json.Marshal(request.Context)
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, fmt.Errorf("encode GIRA input: %w", err)
	}
	messages := []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: string(userJSON)}}
	schema := request.Schema
	raw, metrics, err := p.generateGIRA(backgroundCtx, messages, schema, onProgress)
	metrics.PrimaryDuration = metrics.Total
	if err != nil {
		return clinicalanalysis.ProviderOutput{}, &GIRAOutputError{Diagnostics: GIRAFailureDiagnostics{Classification: classifyGIRAFailure(err), InputBytes: len(userJSON), Metrics: metrics}, Err: err}
	}
	validate := request.ValidateCandidate
	if err = validate(raw); err == nil {
		return clinicalanalysis.ProviderOutput{JSON: raw, Metrics: metrics}, nil
	}
	firstFailure := err.Error()
	candidate, combined := raw, metrics
	combined.RepairReason = classifyGIRAFailure(err)
	for attempt := 0; attempt < 1; attempt++ {
		repairPrompt := "Corrige solamente los campos/refs que causan este error y devuelve el objeto JSON completo según el schema. Conserva los elementos válidos. Usa exclusivamente refs y versiones de INPUT; si falta grounding, elimina la propuesta insegura y agrega insufficient_evidence. ERROR_VALIDACION: " + err.Error() + "\nINPUT:\n" + string(userJSON) + "\nPROPUESTA:\n" + string(candidate)
		repaired, repairMetrics, repairErr := p.generateGIRA(backgroundCtx, []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: repairPrompt}}, schema, onProgress)
		if repairErr != nil {
			return clinicalanalysis.ProviderOutput{}, &GIRAOutputError{Diagnostics: GIRAFailureDiagnostics{Classification: classifyGIRAFailure(repairErr), FirstFailure: firstFailure, FinalFailure: repairErr.Error(), InputBytes: len(userJSON), OutputBytes: len(candidate), RepairAttempted: true, Metrics: combined}, Err: repairErr}
		}
		combined.Total += repairMetrics.Total
		combined.RepairDuration += repairMetrics.Total
		combined.EvalCount += repairMetrics.EvalCount
		combined.Repaired = true
		candidate = repaired
		if err = validate(candidate); err == nil {
			return clinicalanalysis.ProviderOutput{JSON: candidate, Metrics: combined}, nil
		}
	}
	finalErr := fmt.Errorf("%w: %v", clinicalanalysis.ErrInvalidModelOutput, err)
	return clinicalanalysis.ProviderOutput{}, &GIRAOutputError{Diagnostics: GIRAFailureDiagnostics{Classification: classifyGIRAFailure(err), FirstFailure: firstFailure, FinalFailure: err.Error(), InputBytes: len(userJSON), OutputBytes: len(candidate), RepairAttempted: true, Metrics: combined}, Err: finalErr}
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
	return p.generateWithProfile(ctx, messages, schema, generationProfile{contextTokens: contextTokens, temperature: temperature, maxOutputTokens: maxOutputTokens, timeout: timeout, think: think}, onProgress)
}

type generationProfile struct {
	contextTokens, maxOutputTokens int
	temperature, topP              float64
	topK, seed                     int
	timeout                        time.Duration
	think                          bool
}

func (p *Provider) generateGIRA(ctx context.Context, messages []chatMessage, schema map[string]any, onProgress func(clinicalanalysis.GenerationProgress)) ([]byte, clinicalanalysis.GenerationMetrics, error) {
	return p.generateWithProfile(ctx, messages, schema, generationProfile{contextTokens: p.config.GIRAContextTokens, temperature: p.config.GIRATemperature, maxOutputTokens: p.config.GIRAMaxOutputTokens, timeout: p.config.GIRATimeout, think: p.config.GIRAThink, topP: p.config.GIRATopP, topK: p.config.GIRATopK, seed: p.config.GIRASeed}, onProgress)
}

func (p *Provider) generateWithProfile(ctx context.Context, messages []chatMessage, schema map[string]any, profile generationProfile, onProgress func(clinicalanalysis.GenerationProgress)) ([]byte, clinicalanalysis.GenerationMetrics, error) {
	options := map[string]any{
		"temperature": profile.temperature,
		"num_ctx":     profile.contextTokens,
		"num_predict": profile.maxOutputTokens,
	}
	if profile.topP > 0 {
		options["top_p"] = profile.topP
	}
	if profile.topK > 0 {
		options["top_k"] = profile.topK
	}
	if profile.seed != 0 {
		options["seed"] = profile.seed
	}
	request := map[string]any{
		"model":      p.config.Model,
		"messages":   messages,
		"stream":     true,
		"think":      profile.think,
		"format":     schema,
		"keep_alive": p.config.KeepAlive,
		"options":    options,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, clinicalanalysis.GenerationMetrics{}, err
	}
	if profile.timeout <= 0 {
		profile.timeout = p.config.Timeout
	}
	timedCtx, cancel := context.WithTimeout(ctx, profile.timeout)
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
