package approvedcontext

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalairun "sessionflow/apps/api/internal/usecase/clinicalairun"
	clinicalmemory "sessionflow/apps/api/internal/usecase/clinicalmemory"
	sessionreport "sessionflow/apps/api/internal/usecase/sessionreport"
)

type FormulationSource interface {
	GetApprovedContext(context.Context, uuid.UUID, uuid.UUID) (clinicalmemory.ApprovedContext, error)
}
type ReportSource interface {
	ListApprovedByClient(context.Context, uuid.UUID, uuid.UUID, int) ([]sessionreport.Report, error)
}
type ClinicalAccess interface {
	CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error)
}
type ApprovedClinicalContext struct {
	ClientID       uuid.UUID                       `json:"client_id"`
	Formulation    *clinicalmemory.ApprovedContext `json:"formulation,omitempty"`
	SessionReports []sessionreport.Report          `json:"session_reports"`
	Sources        []clinicalairun.Source          `json:"sources"`
	ContextHash    string                          `json:"context_hash"`
}
type Service struct {
	formulations FormulationSource
	reports      ReportSource
	access       ClinicalAccess
	reportLimit  int
}

func NewService(formulations FormulationSource, reports ReportSource, access ClinicalAccess) *Service {
	return &Service{formulations: formulations, reports: reports, access: access, reportLimit: 10}
}
func (s *Service) Get(ctx context.Context, tenantID, clientID, actorID uuid.UUID) (ApprovedClinicalContext, error) {
	if tenantID == uuid.Nil || clientID == uuid.Nil || actorID == uuid.Nil {
		return ApprovedClinicalContext{}, domainerrors.NewValidation("tenant_id, client_id and actor_user_id are required")
	}
	allowed, err := s.access.CanAccessClient(ctx, tenantID, actorID, clientID, "treating", "supervisor")
	if err != nil {
		return ApprovedClinicalContext{}, fmt.Errorf("check approved context access: %w", err)
	}
	if !allowed {
		return ApprovedClinicalContext{}, domainerrors.ErrForbidden
	}
	out := ApprovedClinicalContext{ClientID: clientID, SessionReports: []sessionreport.Report{}, Sources: []clinicalairun.Source{}}
	formulation, err := s.formulations.GetApprovedContext(ctx, tenantID, clientID)
	if err != nil && !errors.Is(err, domainerrors.ErrNotFound) {
		return ApprovedClinicalContext{}, err
	}
	if err == nil {
		out.Formulation = &formulation
		version := formulation.Version
		out.Sources = append(out.Sources, clinicalairun.Source{TenantID: tenantID, SourceType: clinicalairun.SourceFormulationSnapshot, SourceID: formulation.SnapshotID, SourceVersion: &version})
		for _, anchor := range formulation.Anchors {
			out.Sources = append(out.Sources, clinicalairun.Source{TenantID: tenantID, SourceType: clinicalairun.SourceFormulationAnchor, SourceID: anchor.ID, SourceVersion: &version})
		}
	}
	reports, err := s.reports.ListApprovedByClient(ctx, tenantID, clientID, s.reportLimit)
	if err != nil {
		return ApprovedClinicalContext{}, err
	}
	out.SessionReports = reports
	for _, report := range reports {
		version := report.Version
		out.Sources = append(out.Sources, clinicalairun.Source{TenantID: tenantID, SourceType: clinicalairun.SourceSessionReport, SourceID: report.ID, SourceVersion: &version})
	}
	canonical, err := clinicalairun.CanonicalSources(tenantID, out.Sources)
	if err != nil {
		return ApprovedClinicalContext{}, err
	}
	out.ContextHash = clinicalairun.HashSources(canonical)
	return out, nil
}
