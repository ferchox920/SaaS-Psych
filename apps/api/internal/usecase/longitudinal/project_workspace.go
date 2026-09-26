package longitudinal

import (
	"context"
	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"time"
)

// Export history contains receipt metadata only, never source mappings or narratives.
type ProjectExportSummary struct {
	ID           uuid.UUID `json:"export_id"`
	GeneratedAt  time.Time `json:"generated_at"`
	StateVersion int64     `json:"state_version"`
	ContentHash  string    `json:"content_hash"`
	ProcessRef   string    `json:"process_ref"`
}
type ProjectExportPage struct {
	Items                   []ProjectExportSummary `json:"items"`
	Offset                  int                    `json:"offset"`
	HasMore                 bool                   `json:"has_more"`
	RequiresExternalConsent bool                   `json:"requires_external_manual_consent"`
}

func (s *Service) ProjectExports(ctx context.Context, t, c, a uuid.UUID, offset int) (ProjectExportPage, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return ProjectExportPage{}, err
	}
	if offset < 0 || offset > 1000000 {
		return ProjectExportPage{}, domainerrors.ErrValidation
	}
	r, ok := s.repo.(interface {
		ProjectExports(context.Context, uuid.UUID, uuid.UUID, int) (ProjectExportPage, error)
	})
	if !ok {
		return ProjectExportPage{}, domainerrors.ErrNotFound
	}
	out, err := r.ProjectExports(ctx, t, c, offset)
	out.RequiresExternalConsent = s.consent != nil
	return out, err
}
