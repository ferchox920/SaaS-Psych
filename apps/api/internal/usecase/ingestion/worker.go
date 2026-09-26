package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/clinicalanalysis"
	"sessionflow/apps/api/internal/usecase/sessionreport"
	"sessionflow/apps/api/internal/usecase/transcription"
)

type WorkerRepository interface {
	Claim(context.Context, uuid.UUID, time.Duration) (Job, error)
	RecoverExpired(context.Context, uuid.UUID, int) (int, error)
	AuthorizeAttempt(context.Context, Job) error
	GetArtifact(context.Context, uuid.UUID, uuid.UUID) (Artifact, error)
	CompleteTranscription(context.Context, Job, TranscriptContent) (Transcript, error)
	StartAnalysis(context.Context, Job, string, string, string) (uuid.UUID, Transcript, error)
	CompleteAnalysis(context.Context, Job, uuid.UUID, sessionreport.ReportV1) (sessionreport.Report, error)
	FailAttempt(context.Context, Job, string) error
}
type Transcriber interface {
	TranscribeDurable(context.Context, []byte, string, uuid.UUID, string) (TranscriptContent, error)
}
type Worker struct {
	metrics              WorkerMetrics
	lastTick             atomic.Int64
	active               atomic.Bool
	repo                 WorkerRepository
	store                AudioStore
	transcriber          Transcriber
	reports              sessionreport.Provider
	model, app, revision string
	timeout              time.Duration
}
type WorkerMetrics interface {
	RecordIngestionJob(string, string, time.Duration)
}

func (w *Worker) WithMetrics(m WorkerMetrics) *Worker { w.metrics = m; return w }
func (w *Worker) Health() map[string]any {
	return map[string]any{"last_tick_unix": w.lastTick.Load(), "active": w.active.Load()}
}

func NewWorker(repo WorkerRepository, store AudioStore, tr Transcriber, reports sessionreport.Provider, model, app, revision string, timeout time.Duration) (*Worker, error) {
	if repo == nil || store == nil || tr == nil || reports == nil || model == "" || app == "" || revision == "" || timeout < time.Second || timeout > 25*time.Minute {
		return nil, domainerrors.NewValidation("invalid local worker configuration")
	}
	return &Worker{repo: repo, store: store, transcriber: tr, reports: reports, model: model, app: app, revision: revision, timeout: timeout}, nil
}

// RunOne processes at most one job. A scheduler calls it for explicitly scoped
// tenants. There is no remote provider, hidden fallback, or clinical merge here.
func (w *Worker) RunOne(ctx context.Context, tenant uuid.UUID) (bool, error) {
	w.lastTick.Store(time.Now().Unix())
	if _, err := w.repo.RecoverExpired(ctx, tenant, 20); err != nil {
		return false, err
	}
	job, err := w.repo.Claim(ctx, tenant, w.timeout+time.Minute)
	if errors.Is(err, domainerrors.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	attempt, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()
	started := time.Now()
	w.active.Store(true)
	defer func() { w.active.Store(false); w.lastTick.Store(time.Now().Unix()) }()
	err = w.execute(attempt, job)
	if w.metrics != nil {
		status := "succeeded"
		if err != nil {
			status = "failed"
		}
		w.metrics.RecordIngestionJob(job.Type, status, time.Since(started))
	}
	if err != nil {
		code := "validation"
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			code = "timeout"
		case errors.Is(err, context.Canceled):
			code = "cancelled"
		case errors.Is(err, domainerrors.ErrForbidden):
			code = "consent_revoked"
		case errors.Is(err, transcription.ErrUnavailable) || errors.Is(err, transcription.ErrBusy):
			code = "sidecar_unavailable"
		case errors.Is(err, clinicalanalysis.ErrProviderUnavailable) || errors.Is(err, clinicalanalysis.ErrProviderBusy):
			code = "provider_unavailable"
		}
		finish, cancelFinish := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelFinish()
		if failErr := w.repo.FailAttempt(finish, job, code); failErr != nil && !errors.Is(failErr, domainerrors.ErrConflict) {
			return true, failErr
		}
		// No transcript, provider output, file path or raw error becomes a job error.
		return true, errors.New(code)
	}
	return true, nil
}
func (w *Worker) execute(ctx context.Context, j Job) error {
	if err := w.repo.AuthorizeAttempt(ctx, j); err != nil {
		return err
	}
	switch j.Type {
	case Transcribe:
		if j.ArtifactID == nil {
			return domainerrors.ErrConflict
		}
		v, err := w.repo.GetArtifact(ctx, j.TenantID, *j.ArtifactID)
		if err != nil {
			return err
		}
		if v.ClientID != j.ClientID || v.SessionID != j.SessionID || v.Status != "available" || v.Hash == nil || !v.RetentionUntil.After(time.Now()) {
			return domainerrors.ErrConflict
		}
		audio, err := w.store.Read(ctx, v.StorageKey, *v.Hash, v.KeyID)
		if err != nil {
			return err
		}
		if err := w.repo.AuthorizeAttempt(ctx, j); err != nil {
			return err
		}
		content, err := w.transcriber.TranscribeDurable(ctx, audio, v.MediaType, j.ID, j.ConfigurationHash)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err = w.repo.CompleteTranscription(ctx, j, content)
		return err
	case Analyze:
		run, v, err := w.repo.StartAnalysis(ctx, j, w.model, w.app, w.revision)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"schema_version": sessionreport.SchemaVersion, "session_text": v.Text})
		result, err := w.reports.GenerateSessionReport(ctx, sessionreport.SystemPromptV1(), payload, sessionreport.JSONSchemaV1())
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		document, err := sessionreport.DecodeAndValidate(result.JSON)
		if err != nil {
			return err
		}
		_, err = w.repo.CompleteAnalysis(ctx, j, run, document)
		return err
	default:
		return domainerrors.NewValidation("unsupported worker operation")
	}
}
