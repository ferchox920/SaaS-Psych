package transcriber

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewProviderRejectsRemoteEndpoint(t *testing.T) {
	if _, err := NewProvider("https://speech.example.com", time.Second); err == nil {
		t.Fatal("expected remote transcriber endpoint to be rejected")
	}
}

func TestTranscribeSendsRawAudioOnlyToLoopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transcribe" || r.Header.Get("X-Audio-Format") != "webm" {
			t.Fatalf("unexpected request: %s format=%s", r.URL.Path, r.Header.Get("X-Audio-Format"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "fictitious-audio" {
			t.Fatal("audio body mismatch")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"Texto ficticio.","language":"es","duration_seconds":2,"transcription_seconds":0.5,"real_time_factor":0.25,"engine":"faster-whisper","model":"medium"}`))
	}))
	defer server.Close()
	provider, err := NewProvider(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.Transcribe(context.Background(), []byte("fictitious-audio"), "webm")
	if err != nil || result.Text != "Texto ficticio." {
		t.Fatalf("unexpected transcription: %+v err=%v", result, err)
	}
}
