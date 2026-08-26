package transcriber

import (
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
	"time"

	transcription "sessionflow/apps/api/internal/usecase/transcription"
)

type Provider struct {
	baseURL string
	client  *http.Client
}

func NewProvider(baseURL string, timeout time.Duration) (*Provider, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || !isLoopback(parsed) {
		return nil, errors.New("transcriber URL must use an explicit loopback host")
	}
	return &Provider{baseURL: strings.TrimRight(parsed.String(), "/"), client: &http.Client{Timeout: timeout}}, nil
}

func (p *Provider) Health(ctx context.Context) (transcription.Status, error) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/health", nil)
	response, err := p.client.Do(request)
	if err != nil {
		return transcription.Status{}, fmt.Errorf("%w: %v", transcription.ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return transcription.Status{}, transcription.ErrUnavailable
	}
	var status transcription.Status
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		return transcription.Status{}, transcription.ErrUnavailable
	}
	return status, nil
}

func (p *Provider) Transcribe(ctx context.Context, audio []byte, format string) (transcription.Result, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/transcribe", bytes.NewReader(audio))
	if err != nil {
		return transcription.Result{}, err
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("X-Audio-Format", format)
	response, err := p.client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return transcription.Result{}, err
		}
		return transcription.Result{}, fmt.Errorf("%w: %v", transcription.ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusConflict {
		return transcription.Result{}, transcription.ErrBusy
	}
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return transcription.Result{}, transcription.ErrUnavailable
	}
	var result transcription.Result
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return transcription.Result{}, transcription.ErrUnavailable
	}
	return result, nil
}

func isLoopback(parsed *url.URL) bool {
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
