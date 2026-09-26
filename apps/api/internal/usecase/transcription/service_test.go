package transcription

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
)

type providerStub struct{ called bool }

func (p *providerStub) Health(context.Context) (Status, error) { return Status{Available: true}, nil }
func (p *providerStub) Transcribe(context.Context, []byte, string) (Result, error) {
	p.called = true
	return Result{Text: "Texto ficticio.", Language: "es", Engine: "faster-whisper", Model: "medium"}, nil
}

type appointmentStub struct{ clientID uuid.UUID }

func (a appointmentStub) GetByID(context.Context, uuid.UUID, uuid.UUID) (domainappointment.Entity, error) {
	return domainappointment.Entity{ClientID: a.clientID}, nil
}

type transcriptionAccessStub struct{ allowed bool }

func (a transcriptionAccessStub) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return a.allowed, nil
}

type transcriptionAuditStub struct{ metadata []map[string]any }

func (a *transcriptionAuditStub) RecordDomainEvent(_ context.Context, _, _ uuid.UUID, _ string, _ string, _ *uuid.UUID, metadata map[string]any) error {
	a.metadata = append(a.metadata, metadata)
	return nil
}

func TestTranscribeRequiresExplicitConsentBeforeProvider(t *testing.T) {
	provider := &providerStub{}
	service := NewService(true, 1024, provider, appointmentStub{clientID: uuid.New()}, transcriptionAccessStub{allowed: true}, nil)
	_, err := service.Transcribe(context.Background(), TranscribeInput{
		TenantID: uuid.New(), ActorUserID: uuid.New(), AppointmentID: uuid.New(), Format: "wav", Audio: []byte("audio"),
	})
	if err == nil || provider.called {
		t.Fatal("provider must not receive audio without explicit consent")
	}
}

func TestTranscribeAuditsMetricsWithoutAudioOrText(t *testing.T) {
	provider := &providerStub{}
	auditor := &transcriptionAuditStub{}
	service := NewService(true, 1024, provider, appointmentStub{clientID: uuid.New()}, transcriptionAccessStub{allowed: true}, auditor)
	_, err := service.Transcribe(context.Background(), TranscribeInput{
		TenantID: uuid.New(), ActorUserID: uuid.New(), AppointmentID: uuid.New(), ExplicitConsent: true,
		Format: "wav", Audio: []byte("fictitious-audio-bytes"),
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(auditor.metadata)
	if len(encoded) == 0 || contains(string(encoded), "fictitious-audio") || contains(string(encoded), "Texto ficticio") {
		t.Fatalf("audit must not contain audio or transcript: %s", encoded)
	}
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
