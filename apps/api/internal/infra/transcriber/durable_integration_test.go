package transcriber

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestFasterWhisperDurableSyntheticIntegration(t *testing.T) {
	if os.Getenv("RUN_FASTER_WHISPER_INTEGRATION") != "1" {
		t.Skip("set RUN_FASTER_WHISPER_INTEGRATION=1; synthetic TTS audio only")
	}
	path := os.Getenv("SYNTHETIC_TRANSCRIPTION_AUDIO")
	if path == "" {
		t.Fatal("synthetic audio fixture required")
	}
	audio, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("cannot read synthetic audio")
	}
	base := os.Getenv("SYNTHETIC_TRANSCRIBER_URL")
	if base == "" {
		base = "http://127.0.0.1:8092"
	}
	p, err := NewProvider(base, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/health", nil)
	res, err := p.client.Do(req)
	if err != nil {
		t.Fatal("synthetic sidecar unavailable")
	}
	defer res.Body.Close()
	var health struct {
		Hash     string `json:"configuration_hash"`
		Contract string `json:"contract"`
	}
	if err = json.NewDecoder(res.Body).Decode(&health); err != nil || health.Contract != "local-transcription-v1" {
		t.Fatal("sidecar contract unavailable")
	}
	start := time.Now()
	out, err := p.TranscribeDurable(ctx, audio, "wav", uuid.New(), health.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Segments) == 0 || out.Text == "" || out.Engine != "faster-whisper" {
		t.Fatal("missing durable transcript")
	}
	t.Logf("synthetic acceptance: segments=%d engine=%s model=%s duration=%s", len(out.Segments), out.Engine, out.Model, time.Since(start))
}
