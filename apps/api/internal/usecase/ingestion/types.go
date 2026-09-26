package ingestion

import (
	"github.com/google/uuid"
	"time"
)

const (
	Transcribe = "transcribe_session_audio"
	Analyze    = "analyze_session"
	Cleanup    = "artifact_cleanup"
)

type Job struct {
	ID                uuid.UUID  `json:"id"`
	TenantID          uuid.UUID  `json:"tenant_id"`
	ClientID          uuid.UUID  `json:"client_id"`
	SessionID         uuid.UUID  `json:"session_id"`
	Type              string     `json:"job_type"`
	ArtifactID        *uuid.UUID `json:"artifact_id,omitempty"`
	TranscriptID      *uuid.UUID `json:"transcript_version_id,omitempty"`
	Status            string     `json:"status"`
	Attempt           int        `json:"attempt"`
	MaxAttempts       int        `json:"max_attempts"`
	LeaseToken        *uuid.UUID `json:"-"`
	LeaseUntil        *time.Time `json:"lease_until,omitempty"`
	ErrorCode         *string    `json:"error_code,omitempty"`
	ActorID           uuid.UUID  `json:"created_by_user_id"`
	ConfigurationHash string     `json:"configuration_hash"`
	Language          string     `json:"language"`
}

func Retryable(code string) bool {
	return code == "sidecar_unavailable" || code == "provider_unavailable" || code == "timeout" || code == "worker_lost" || code == "storage_unavailable"
}
