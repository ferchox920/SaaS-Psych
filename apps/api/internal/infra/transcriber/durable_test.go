package transcriber

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"strings"
	"testing"
	"time"
)

func TestDurableSidecarStrictContractAndFailures(t *testing.T) {
	id := uuid.New()
	hash := ingestion.Hash("synthetic-config")
	content := ingestion.TranscriptContent{Text: "Texto ficticio.", Segments: []ingestion.Segment{{Start: 0, End: 1, Text: "Texto ficticio."}}, Language: "es", Engine: "faster-whisper", Model: "test", EngineVersion: "1.0", ConfigurationHash: hash}
	raw, _ := json.Marshal(map[string]any{"contract": "local-transcription-v1", "request_id": id, "content": content})
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{{"valid", 200, string(raw), true}, {"malformed", 200, "{", false}, {"partial", 200, string(raw[:len(raw)/2]), false}, {"unknown-field", 200, strings.TrimSuffix(string(raw), "}") + `,"debug":"secret"}`, false}, {"trailing", 200, string(raw) + ` {}`, false}, {"wrong-request", 200, strings.ReplaceAll(string(raw), id.String(), uuid.NewString()), false}, {"wrong-config", 200, strings.ReplaceAll(string(raw), hash, ingestion.Hash("other")), false}, {"busy", 409, "", false}, {"crashed", 500, "", false}, {"unsupported", 400, "", false}}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/transcriptions" || r.Header.Get("X-Request-ID") != id.String() || r.Header.Get("X-Configuration-Hash") != hash {
					t.Error("bad request provenance")
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			p, _ := NewProvider(server.URL, time.Second)
			_, err := p.TranscribeDurable(context.Background(), []byte("synthetic"), "wav", id, hash)
			if (err == nil) != tt.want {
				t.Fatalf("result=%v", err)
			}
		})
	}
	t.Run("connection-refused", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := server.URL
		server.Close()
		p, _ := NewProvider(url, time.Second)
		if _, err := p.TranscribeDurable(context.Background(), []byte("synthetic"), "wav", id, hash); err == nil {
			t.Fatal("accepted unavailable")
		}
	})
	t.Run("timeout-cancel-redirect", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(100 * time.Millisecond):
			}
		}))
		defer server.Close()
		p, _ := NewProvider(server.URL, 20*time.Millisecond)
		if _, err := p.TranscribeDurable(context.Background(), []byte("synthetic"), "wav", id, hash); err == nil {
			t.Fatal("timeout ignored")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := p.TranscribeDurable(ctx, []byte("synthetic"), "wav", id, hash); err == nil {
			t.Fatal("cancellation ignored")
		}
		remoteCalls := 0
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { remoteCalls++ }))
		defer target.Close()
		redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
		}))
		defer redirect.Close()
		p, _ = NewProvider(redirect.URL, time.Second)
		_, _ = p.TranscribeDurable(context.Background(), []byte("synthetic"), "wav", id, hash)
		if remoteCalls != 0 {
			t.Fatal("audio followed redirect")
		}
	})
}
