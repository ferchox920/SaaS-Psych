package manualsource

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

const HashScheme = "manual-content-json-v1"

type Message struct {
	ID           string     `json:"id"`
	Role         string     `json:"role"`
	Text         string     `json:"text"`
	RecordedAt   *time.Time `json:"recorded_at,omitempty"`
	StartSeconds *float64   `json:"start_seconds,omitempty"`
	EndSeconds   *float64   `json:"end_seconds,omitempty"`
}

// File references describe existing originals. They grant no filesystem access.
type FileReference struct {
	Name      string `json:"name"`
	LocalPath string `json:"local_path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
	MediaType string `json:"media_type"`
}
type Content struct {
	Title    string          `json:"title"`
	Messages []Message       `json:"messages"`
	Text     string          `json:"text"`
	Files    []FileReference `json:"files"`
	Metadata json.RawMessage `json:"metadata"`
}
type Import struct {
	Kind            string     `json:"source_kind"`
	ProjectID       string     `json:"project_id"`
	SourceID        string     `json:"source_id"`
	SourceCreatedAt *time.Time `json:"source_created_at"`
	SourceUpdatedAt *time.Time `json:"source_updated_at"`
	CandidateDate   *string    `json:"candidate_session_date"`
	DateBasis       *string    `json:"session_date_basis"`
	Timezone        string     `json:"source_timezone"`
	SessionID       *uuid.UUID `json:"clinical_session_id"`
	Content         Content    `json:"content"`
}
type Material struct {
	ID              uuid.UUID  `json:"id"`
	ClientID        uuid.UUID  `json:"client_id"`
	Kind            string     `json:"source_kind"`
	ProjectID       string     `json:"project_id"`
	SourceID        string     `json:"source_id"`
	SourceCreatedAt *time.Time `json:"source_created_at"`
	SourceUpdatedAt *time.Time `json:"source_updated_at"`
	CandidateDate   *string    `json:"candidate_session_date"`
	DateBasis       *string    `json:"session_date_basis"`
	Timezone        string     `json:"source_timezone"`
	SessionID       *uuid.UUID `json:"clinical_session_id"`
	RevisionID      uuid.UUID  `json:"revision_id"`
	Hash            string     `json:"content_hash"`
	ReviewStatus    string     `json:"review_status"`
	ImportedAt      time.Time  `json:"imported_at"`
	VersionCount    int        `json:"version_count"`
	Content         Content    `json:"content"`
}

var projectID = regexp.MustCompile(`^g-p-[a-f0-9]{32}$`)
var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (v Import) Canonical() ([]byte, string, error) {
	invalid := func() ([]byte, string, error) {
		return nil, "", domainerrors.NewValidation("invalid imported source content or identity")
	}
	if !projectID.MatchString(v.ProjectID) || strings.TrimSpace(v.SourceID) == "" || len(v.SourceID) > 512 {
		return invalid()
	}
	switch v.Kind {
	case "chatgpt_project", "project_instructions", "desktop_recording", "phone_recording", "local_transcript", "comparison":
	default:
		return invalid()
	}
	if v.Content.Title == "" || len([]rune(v.Content.Title)) > 250 || len(v.Content.Messages) > 2000 || len(v.Content.Files) > 30 {
		return invalid()
	}
	if len(v.Content.Messages) == 0 && v.Content.Text == "" && len(v.Content.Files) == 0 {
		return invalid()
	}
	seen := map[string]bool{}
	for _, m := range v.Content.Messages {
		if m.ID == "" || len(m.ID) > 512 || seen[m.ID] || m.Text == "" {
			return invalid()
		}
		seen[m.ID] = true
		switch m.Role {
		case "user", "assistant", "patient", "therapist", "system":
		default:
			return invalid()
		}
		if (m.StartSeconds == nil) != (m.EndSeconds == nil) {
			return invalid()
		}
		if m.StartSeconds != nil && (*m.StartSeconds < 0 || *m.EndSeconds < *m.StartSeconds) {
			return invalid()
		}
	}
	for _, f := range v.Content.Files {
		if f.Name == "" || !digest.MatchString(f.SHA256) || f.SizeBytes < 1 || len(f.LocalPath) > 2048 {
			return invalid()
		}
	}
	if (v.CandidateDate == nil) != (v.DateBasis == nil) {
		return invalid()
	}
	if v.CandidateDate != nil {
		if _, err := time.Parse("2006-01-02", *v.CandidateDate); err != nil {
			return invalid()
		}
		switch *v.DateBasis {
		case "source_timestamp", "explicit_text", "professional_confirmed":
		default:
			return invalid()
		}
	}
	if v.Timezone != "" {
		if _, err := time.LoadLocation(v.Timezone); err != nil {
			return invalid()
		}
	}
	if v.SessionID != nil && *v.SessionID == uuid.Nil {
		return invalid()
	}
	if v.SourceCreatedAt != nil && v.SourceUpdatedAt != nil && v.SourceUpdatedAt.Before(*v.SourceCreatedAt) {
		return invalid()
	}
	// Round-trip metadata so key order/whitespace do not create duplicate versions.
	var metadata any
	if len(v.Content.Metadata) > 0 {
		if json.Unmarshal(v.Content.Metadata, &metadata) != nil {
			return invalid()
		}
	}
	canonicalMetadata, _ := json.Marshal(metadata)
	v.Content.Metadata = canonicalMetadata
	if v.Content.Messages == nil {
		v.Content.Messages = []Message{}
	}
	if v.Content.Files == nil {
		v.Content.Files = []FileReference{}
	}
	raw, err := json.Marshal(v.Content)
	if err != nil || len(raw) > 900000 {
		return invalid()
	}
	h := sha256.Sum256(raw)
	return raw, hex.EncodeToString(h[:]), nil
}
