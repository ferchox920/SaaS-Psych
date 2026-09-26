package ingestion

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
	"sessionflow/apps/api/internal/usecase/sessionreport"
)

type Artifact struct {
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	TenantID       uuid.UUID `json:"tenant_id"`
	ClientID       uuid.UUID `json:"client_id"`
	SessionID      uuid.UUID `json:"session_id"`
	Status         string    `json:"status"`
	MediaType      string    `json:"media_type"`
	StorageKey     string    `json:"-"`
	KeyID          string    `json:"-"`
	Hash           *string   `json:"content_hash,omitempty"`
	Size           *int64    `json:"size_bytes,omitempty"`
	RetentionUntil time.Time `json:"retention_until"`
	ActorID        uuid.UUID `json:"created_by_user_id"`
}
type AudioReceipt struct {
	Key, Hash, KeyID string
	Size             int64
}
type AudioStore interface {
	WriteAudio(context.Context, string, io.Reader) (AudioReceipt, error)
	Read(context.Context, string, string, string) ([]byte, error)
	Delete(context.Context, string) error
}
type Repository interface {
	ListJobs(context.Context, uuid.UUID, uuid.UUID) ([]Job, error)
	DeleteTranscript(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Transcript, error)
	ListArtifacts(context.Context, uuid.UUID, uuid.UUID) ([]Artifact, error)
	ListTranscripts(context.Context, uuid.UUID, uuid.UUID) ([]Transcript, error)
	RetryJob(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Job, error)
	ReserveArtifact(context.Context, Artifact) (Artifact, error)
	FinalizeArtifact(context.Context, Artifact, AudioReceipt) (Artifact, error)
	GetArtifact(context.Context, uuid.UUID, uuid.UUID) (Artifact, error)
	BeginDeleteArtifact(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Artifact, error)
	FinishDeleteArtifact(context.Context, uuid.UUID, uuid.UUID) error
	Enqueue(context.Context, Job) (Job, bool, error)
	GetJob(context.Context, uuid.UUID, uuid.UUID) (Job, error)
	CancelJob(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Job, error)
	GetTranscript(context.Context, uuid.UUID, uuid.UUID) (Transcript, error)
	CorrectTranscript(context.Context, Transcript, uuid.UUID) (Transcript, error)
}

func (s *Service) ListJobs(ctx context.Context, t, sid, a uuid.UUID) ([]Job, error) {
	v, err := s.sessions.SessionDetails(ctx, t, sid)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, t, v.ClientID, a, false); err != nil {
		return nil, err
	}
	return s.repo.ListJobs(ctx, t, sid)
}

func (s *Service) DeleteTranscript(ctx context.Context, t, id, a uuid.UUID) (Transcript, error) {
	v, err := s.repo.GetTranscript(ctx, t, id)
	if err != nil {
		return Transcript{}, err
	}
	if err := s.require(ctx, t, v.ClientID, a, true); err != nil {
		return Transcript{}, err
	}
	return s.repo.DeleteTranscript(ctx, t, id, a)
}

func (s *Service) ListArtifacts(ctx context.Context, t, sid, a uuid.UUID) ([]Artifact, error) {
	v, err := s.sessions.SessionDetails(ctx, t, sid)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, t, v.ClientID, a, false); err != nil {
		return nil, err
	}
	return s.repo.ListArtifacts(ctx, t, sid)
}
func (s *Service) ListTranscripts(ctx context.Context, t, sid, a uuid.UUID) ([]Transcript, error) {
	v, err := s.sessions.SessionDetails(ctx, t, sid)
	if err != nil {
		return nil, err
	}
	if err := s.require(ctx, t, v.ClientID, a, false); err != nil {
		return nil, err
	}
	return s.repo.ListTranscripts(ctx, t, sid)
}
func (s *Service) RetryJob(ctx context.Context, t, id, a uuid.UUID) (Job, error) {
	j, err := s.repo.GetJob(ctx, t, id)
	if err != nil {
		return Job{}, err
	}
	if err := s.require(ctx, t, j.ClientID, a, true); err != nil {
		return Job{}, err
	}
	return s.repo.RetryJob(ctx, t, id, a)
}

type Sessions interface {
	SessionDetails(context.Context, uuid.UUID, uuid.UUID) (sessionreport.SessionDetails, error)
}
type Service struct {
	repo      Repository
	sessions  Sessions
	access    consent.Access
	store     AudioStore
	keyID     string
	retention time.Duration
}

func NewService(r Repository, s Sessions, a consent.Access, store AudioStore, keyID string, retention time.Duration) *Service {
	return &Service{r, s, a, store, keyID, retention}
}
func (s *Service) require(ctx context.Context, t, c, a uuid.UUID, write bool) error {
	if t == uuid.Nil || c == uuid.Nil || a == uuid.Nil {
		return domainerrors.ErrForbidden
	}
	roles := []string{"treating"}
	if !write {
		roles = append(roles, "supervisor")
	}
	ok, err := s.access.CanAccessClient(ctx, t, a, c, roles...)
	if err != nil {
		return err
	}
	if !ok {
		return domainerrors.ErrForbidden
	}
	return nil
}
func (s *Service) Upload(ctx context.Context, t, sid, a uuid.UUID, format string, input io.Reader) (Artifact, error) {
	if s.store == nil || s.keyID == "" || s.retention <= 0 {
		return Artifact{}, domainerrors.NewValidation("durable audio storage is unavailable")
	}
	switch format {
	case "wav", "webm", "ogg", "mp4", "m4a":
	default:
		return Artifact{}, domainerrors.NewValidation("unsupported audio format")
	}
	session, err := s.sessions.SessionDetails(ctx, t, sid)
	if err != nil {
		return Artifact{}, err
	}
	if err := s.require(ctx, t, session.ClientID, a, true); err != nil {
		return Artifact{}, err
	}
	if session.Status == "voided" {
		return Artifact{}, domainerrors.ErrConflict
	}
	id := uuid.New()
	v := Artifact{ID: id, TenantID: t, ClientID: session.ClientID, SessionID: sid, MediaType: format, StorageKey: strings.Join([]string{t.String(), session.ClientID.String(), sid.String(), id.String()}, "/"), KeyID: s.keyID, RetentionUntil: time.Now().UTC().Add(s.retention), ActorID: a}
	v, err = s.repo.ReserveArtifact(ctx, v)
	if err != nil {
		return Artifact{}, err
	}
	receipt, err := s.store.WriteAudio(ctx, v.StorageKey, input)
	if err == nil {
		var result Artifact
		result, err = s.repo.FinalizeArtifact(ctx, v, receipt)
		if err == nil {
			return result, nil
		}
	}
	// Metadata remains recoverable if compensation cannot complete. Do not reuse a
	// cancelled request context for cleanup, and never expose storage error details.
	cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, e := s.repo.BeginDeleteArtifact(cleanup, t, id, a); e == nil {
		if e = s.store.Delete(cleanup, v.StorageKey); e == nil {
			_ = s.repo.FinishDeleteArtifact(cleanup, t, id)
		}
	}
	return Artifact{}, domainerrors.NewValidation("audio upload could not be finalized")
}
func (s *Service) DeleteArtifact(ctx context.Context, t, id, a uuid.UUID) (Artifact, error) {
	v, err := s.repo.GetArtifact(ctx, t, id)
	if err != nil {
		return v, err
	}
	if err := s.require(ctx, t, v.ClientID, a, true); err != nil {
		return Artifact{}, err
	}
	if s.store == nil {
		return Artifact{}, domainerrors.ErrConflict
	}
	v, err = s.repo.BeginDeleteArtifact(ctx, t, id, a)
	if err != nil {
		return v, err
	}
	if err = s.store.Delete(ctx, v.StorageKey); err != nil {
		return v, domainerrors.ErrConflict
	}
	if err := s.repo.FinishDeleteArtifact(ctx, t, id); err != nil {
		return v, err
	}
	return s.repo.GetArtifact(ctx, t, id)
}
func (s *Service) Enqueue(ctx context.Context, t, sid, a, source uuid.UUID, kind, configurationHash string) (Job, bool, error) {
	if source == uuid.Nil || !ValidHash(configurationHash) || (kind != Transcribe && kind != Analyze) {
		return Job{}, false, domainerrors.NewValidation("invalid job source or configuration")
	}
	session, err := s.sessions.SessionDetails(ctx, t, sid)
	if err != nil {
		return Job{}, false, err
	}
	if err := s.require(ctx, t, session.ClientID, a, true); err != nil {
		return Job{}, false, err
	}
	if session.Status == "voided" || (kind == Analyze && session.Status != "completed") {
		return Job{}, false, domainerrors.ErrConflict
	}
	j := Job{ID: uuid.New(), TenantID: t, ClientID: session.ClientID, SessionID: sid, Type: kind, ActorID: a, ConfigurationHash: configurationHash, Language: "es", MaxAttempts: 3}
	if kind == Transcribe {
		j.ArtifactID = &source
	} else {
		j.TranscriptID = &source
	}
	return s.repo.Enqueue(ctx, j)
}
func (s *Service) GetJob(ctx context.Context, t, id, a uuid.UUID) (Job, error) {
	j, err := s.repo.GetJob(ctx, t, id)
	if err != nil {
		return Job{}, err
	}
	if err := s.require(ctx, t, j.ClientID, a, false); err != nil {
		return Job{}, err
	}
	return j, nil
}
func (s *Service) CancelJob(ctx context.Context, t, id, a uuid.UUID) (Job, error) {
	j, err := s.repo.GetJob(ctx, t, id)
	if err != nil {
		return Job{}, err
	}
	if err := s.require(ctx, t, j.ClientID, a, true); err != nil {
		return Job{}, err
	}
	return s.repo.CancelJob(ctx, t, id, a)
}
func (s *Service) GetTranscript(ctx context.Context, t, id, a uuid.UUID) (Transcript, error) {
	v, err := s.repo.GetTranscript(ctx, t, id)
	if err != nil {
		return Transcript{}, err
	}
	if err := s.require(ctx, t, v.ClientID, a, false); err != nil {
		return Transcript{}, err
	}
	return v, nil
}
func (s *Service) CorrectTranscript(ctx context.Context, t, id, a uuid.UUID, text string, segments []Segment) (Transcript, error) {
	v, err := s.repo.GetTranscript(ctx, t, id)
	if err != nil {
		return Transcript{}, err
	}
	if err := s.require(ctx, t, v.ClientID, a, true); err != nil {
		return Transcript{}, err
	}
	if v.Status != "available" {
		return Transcript{}, domainerrors.ErrConflict
	}
	v.ParentID = &id
	v.ID = uuid.New()
	v.Text = text
	v.Segments = segments
	v.Origin = "human_asr_correction"
	if err := v.TranscriptContent.Validate(); err != nil {
		return Transcript{}, err
	}
	return s.repo.CorrectTranscript(ctx, v, a)
}
