package longitudinal

import (
	"context"
	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/approvedcontext"
	"time"
)

// RunProvenance deliberately excludes provider parameters and raw diagnostic data.
type RunProvenance struct {
	ID                  uuid.UUID  `json:"id"`
	Provider            string     `json:"provider"`
	Model               string     `json:"model"`
	Operation           string     `json:"operation"`
	PromptName          string     `json:"prompt_name"`
	PromptVersion       string     `json:"prompt_version"`
	Status              string     `json:"status"`
	StartedAt           time.Time  `json:"started_at"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	TranscriptVersionID *uuid.UUID `json:"transcript_version_id,omitempty"`
}
type EvidencePage struct {
	Items   []Evidence `json:"items"`
	Offset  int        `json:"offset"`
	HasMore bool       `json:"has_more"`
}
type HistoryPage struct {
	Items   []HistoryTransition `json:"items"`
	Offset  int                 `json:"offset"`
	HasMore bool                `json:"has_more"`
}

func (s *Service) EvidencePage(ctx context.Context, t, c, a uuid.UUID, offset int) (EvidencePage, error) {
	if offset < 0 || offset > 1000000 {
		return EvidencePage{}, domainerrors.ErrValidation
	}
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return EvidencePage{}, err
	}
	r, ok := s.repo.(interface {
		EvidencePage(context.Context, uuid.UUID, uuid.UUID, int) (EvidencePage, error)
	})
	if !ok {
		return EvidencePage{}, domainerrors.ErrNotFound
	}
	return r.EvidencePage(ctx, t, c, offset)
}
func (s *Service) HistoryPage(ctx context.Context, t, c, id, a uuid.UUID, kind string, offset int) (HistoryPage, error) {
	if offset < 0 || offset > 1000000 {
		return HistoryPage{}, domainerrors.ErrValidation
	}
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return HistoryPage{}, err
	}
	r, ok := s.repo.(interface {
		HistoryPage(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, int) (HistoryPage, error)
	})
	if !ok {
		return HistoryPage{}, domainerrors.ErrNotFound
	}
	return r.HistoryPage(ctx, t, c, id, kind, offset)
}

func (s *Service) ApprovedContext(ctx context.Context, t, c, a uuid.UUID) (approvedcontext.ApprovedClinicalContext, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return approvedcontext.ApprovedClinicalContext{}, err
	}
	if s.approved == nil {
		return approvedcontext.ApprovedClinicalContext{}, domainerrors.ErrNotFound
	}
	return s.approved.Get(ctx, t, c, a)
}
func (s *Service) RunProvenance(ctx context.Context, t, c, id, a uuid.UUID) (RunProvenance, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return RunProvenance{}, err
	}
	reader, ok := s.repo.(interface {
		RunProvenance(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (RunProvenance, error)
	})
	if !ok {
		return RunProvenance{}, domainerrors.ErrNotFound
	}
	return reader.RunProvenance(ctx, t, c, id)
}
