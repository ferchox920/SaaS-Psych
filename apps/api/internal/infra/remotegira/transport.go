package remotegira

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultMaxResponseBytes int64 = 2 * 1024 * 1024

type TransportRequest struct {
	Path string
	Body []byte
}

type TransportResponse struct {
	StatusCode      int
	Body            []byte
	RemoteRequestID string
	Duration        time.Duration
}

type RemoteLLMTransport interface {
	Do(context.Context, TransportRequest) (TransportResponse, error)
}

type HTTPTransportConfig struct {
	BaseURL          string
	APIKey           string
	Timeout          time.Duration
	MaxResponseBytes int64
	Client           *http.Client
}

type HTTPTransport struct {
	baseURL          *url.URL
	apiKey           string
	timeout          time.Duration
	maxResponseBytes int64
	client           *http.Client
}

type Error struct {
	Code            string
	StatusCode      int
	RemoteRequestID string
	cause           error
}

func (e *Error) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("remote GIRA request failed (%s, HTTP %d)", e.Code, e.StatusCode)
	}
	return fmt.Sprintf("remote GIRA request failed (%s)", e.Code)
}

func (e *Error) Unwrap() error             { return e.cause }
func (e *Error) ProviderErrorCode() string { return e.Code }

func ErrorCode(err error) string {
	var remoteErr *Error
	if errors.As(err, &remoteErr) {
		return remoteErr.Code
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "provider_unavailable"
}

func NewHTTPTransport(config HTTPTransportConfig) (*HTTPTransport, error) {
	base, err := url.Parse(strings.TrimSpace(config.BaseURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, errors.New("remote GIRA base URL must be absolute")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, errors.New("remote GIRA API key is required")
	}
	if config.Timeout <= 0 {
		return nil, errors.New("remote GIRA timeout must be positive")
	}
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = defaultMaxResponseBytes
	}
	client := config.Client
	if client == nil {
		client = &http.Client{}
	}
	return &HTTPTransport{baseURL: base, apiKey: strings.TrimSpace(config.APIKey), timeout: config.Timeout, maxResponseBytes: config.MaxResponseBytes, client: client}, nil
}

func (t *HTTPTransport) Do(ctx context.Context, input TransportRequest) (TransportResponse, error) {
	endpoint := *t.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/" + strings.TrimLeft(input.Path, "/")
	timedCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(timedCtx, http.MethodPost, endpoint.String(), bytes.NewReader(input.Body))
	if err != nil {
		return TransportResponse{}, &Error{Code: "provider_unavailable", cause: err}
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	response, err := t.client.Do(req)
	if err != nil {
		code := "provider_unavailable"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(timedCtx.Err(), context.DeadlineExceeded) {
			code = "timeout"
		} else if errors.Is(err, context.Canceled) || errors.Is(timedCtx.Err(), context.Canceled) {
			code = "cancelled"
		}
		return TransportResponse{}, &Error{Code: code, cause: timedCtx.Err()}
	}
	defer response.Body.Close()
	requestID := strings.TrimSpace(response.Header.Get("x-request-id"))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		code := "provider_rejected"
		switch {
		case response.StatusCode == http.StatusTooManyRequests:
			code = "rate_limited"
		case response.StatusCode >= 500:
			code = "provider_unavailable"
		case response.StatusCode == http.StatusRequestTimeout:
			code = "timeout"
		}
		return TransportResponse{}, &Error{Code: code, StatusCode: response.StatusCode, RemoteRequestID: requestID}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, t.maxResponseBytes+1))
	if err != nil {
		return TransportResponse{}, &Error{Code: "provider_unavailable", RemoteRequestID: requestID, cause: err}
	}
	if int64(len(body)) > t.maxResponseBytes {
		return TransportResponse{}, &Error{Code: "response_too_large", RemoteRequestID: requestID}
	}
	return TransportResponse{StatusCode: response.StatusCode, Body: body, RemoteRequestID: requestID, Duration: time.Since(started)}, nil
}
