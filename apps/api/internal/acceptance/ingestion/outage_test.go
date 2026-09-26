package ingestionacceptance

import (
	"context"
	"net"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"testing"
	"time"

	"sessionflow/apps/api/internal/infra/transcriber"
)

func TestStage2D1RealWhisperOutageRecovery(t *testing.T) {
	if os.Getenv("RUN_STAGE2D1_REAL") != "1" {
		t.Skip("real synthetic sidecar opt-in")
	}
	h := newHarness(t, os.Getenv("SYNTHETIC_TRANSCRIPTION_CONFIGURATION_HASH"))
	h.grants()
	audio, err := os.ReadFile(os.Getenv("SYNTHETIC_TRANSCRIPTION_AUDIO"))
	if err != nil {
		t.Fatal("synthetic audio required")
	}
	v := h.upload(audio)
	j := h.enqueue(v.ID)
	// A loopback proxy controls transport availability without touching another
	// application's sidecar. When restored it forwards to actual faster-whisper.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	tr, err := transcriber.NewProvider("http://"+address, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	w := h.worker(tr, nil)
	if _, err = w.RunOne(context.Background(), h.tenant); err == nil {
		t.Fatal("outage accepted")
	}
	failed := h.job(j.ID)
	if failed.Status != "queued" || failed.ErrorCode == nil || *failed.ErrorCode != "sidecar_unavailable" {
		t.Fatal("outage did not schedule durable retry")
	}
	base := os.Getenv("SYNTHETIC_TRANSCRIBER_URL")
	if base == "" {
		base = "http://127.0.0.1:8092"
	}
	target, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	listener, err = net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewUnstartedServer(httputil.NewSingleHostReverseProxy(target))
	_ = proxy.Listener.Close()
	proxy.Listener = listener
	proxy.Start()
	defer proxy.Close()
	h.want("POST", "/api/v1/clinical-jobs/"+j.ID.String()+"/retry", nil, 202)
	if _, err = w.RunOne(context.Background(), h.tenant); err != nil {
		t.Fatal(err)
	}
	current := h.job(j.ID)
	if current.Status != "succeeded" || current.Attempt != 2 || len(h.transcripts()) != 1 {
		t.Fatal("restored sidecar duplicated/lost output")
	}
	h.assertStorageConsistency()
	t.Log("real worker transport outage -> queued retry -> real faster-whisper: attempts=2 durable_transcripts=1")
}
