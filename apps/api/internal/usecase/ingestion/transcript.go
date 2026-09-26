package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}
type TranscriptContent struct {
	DurationSeconds   float64   `json:"duration_seconds,omitempty"`
	ProcessingSeconds float64   `json:"processing_seconds,omitempty"`
	Text              string    `json:"text"`
	Segments          []Segment `json:"segments"`
	Language          string    `json:"language"`
	Engine            string    `json:"engine"`
	Model             string    `json:"model"`
	EngineVersion     string    `json:"engine_version"`
	ConfigurationHash string    `json:"configuration_hash"`
}
type Transcript struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	ClientID        uuid.UUID  `json:"client_id"`
	SessionID       uuid.UUID  `json:"session_id"`
	Version         int        `json:"version"`
	ParentID        *uuid.UUID `json:"parent_version_id,omitempty"`
	ArtifactID      uuid.UUID  `json:"source_artifact_id"`
	ArtifactHash    string     `json:"source_artifact_hash"`
	ArtifactVersion int        `json:"source_artifact_version"`
	Origin          string     `json:"origin"`
	Status          string     `json:"status"`
	ContentHash     string     `json:"content_hash"`
	CreatedAt       time.Time  `json:"created_at"`
	TranscriptContent
}

func Hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func ValidHash(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && strings.ToLower(s) == s
}
func (c TranscriptContent) Validate() error {
	if math.IsNaN(c.DurationSeconds) || math.IsInf(c.DurationSeconds, 0) || c.DurationSeconds < 0 || math.IsNaN(c.ProcessingSeconds) || math.IsInf(c.ProcessingSeconds, 0) || c.ProcessingSeconds < 0 {
		return domainerrors.NewValidation("invalid transcript timing")
	}
	if len(c.Text) == 0 || len(c.Text) > 160000 || len(c.Segments) == 0 || len(c.Segments) > 10000 || c.Language != "es" || c.Engine != "faster-whisper" || len(c.Model) == 0 || len(c.Model) > 128 || len(c.EngineVersion) == 0 || len(c.EngineVersion) > 128 || !ValidHash(c.ConfigurationHash) {
		return domainerrors.NewValidation("invalid transcript content or provenance")
	}
	texts := make([]string, 0, len(c.Segments))
	previous := 0.0
	for _, s := range c.Segments {
		if math.IsNaN(s.Start) || math.IsNaN(s.End) || math.IsInf(s.Start, 0) || math.IsInf(s.End, 0) || s.Start < previous || s.End < s.Start || s.End > 14400 || strings.TrimSpace(s.Text) == "" {
			return domainerrors.NewValidation("invalid transcript segments")
		}
		previous = s.End
		texts = append(texts, strings.TrimSpace(s.Text))
	}
	if strings.Join(texts, " ") != strings.TrimSpace(c.Text) {
		return domainerrors.NewValidation("transcript text and segments differ")
	}
	return nil
}
