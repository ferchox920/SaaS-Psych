package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

type ClinicalLongitudinalHandler struct{ service *longitudinal.Service }

func NewClinicalLongitudinalHandler(s *longitudinal.Service) *ClinicalLongitudinalHandler {
	return &ClinicalLongitudinalHandler{service: s}
}

func (h *ClinicalLongitudinalHandler) Analyze(c echo.Context) error {
	t, p, s, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.Analyze(c.Request().Context(), t, s, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	status := http.StatusCreated
	if out.Reused {
		status = http.StatusOK
	}
	return c.JSON(status, out)
}
func (h *ClinicalLongitudinalHandler) State(c echo.Context) error {
	return h.withClient(c, func(t, id, a uuid.UUID) (any, error) { return h.service.State(c.Request().Context(), t, id, a) })
}
func (h *ClinicalLongitudinalHandler) Evidence(c echo.Context) error {
	return h.withClient(c, func(t, id, a uuid.UUID) (any, error) {
		x, e := h.service.ListEvidence(c.Request().Context(), t, id, a)
		return map[string]any{"items": x}, e
	})
}
func (h *ClinicalLongitudinalHandler) Events(c echo.Context) error {
	return h.withClient(c, func(t, id, a uuid.UUID) (any, error) {
		x, e := h.service.ListEvents(c.Request().Context(), t, id, a)
		return map[string]any{"items": x}, e
	})
}
func (h *ClinicalLongitudinalHandler) Processes(c echo.Context) error {
	return h.withClient(c, func(t, id, a uuid.UUID) (any, error) {
		x, e := h.service.ListProcesses(c.Request().Context(), t, id, a)
		return map[string]any{"items": x}, e
	})
}
func (h *ClinicalLongitudinalHandler) Hypotheses(c echo.Context) error {
	return h.withClient(c, func(t, id, a uuid.UUID) (any, error) {
		x, e := h.service.ListHypotheses(c.Request().Context(), t, id, a)
		return map[string]any{"items": x}, e
	})
}
func (h *ClinicalLongitudinalHandler) Diffs(c echo.Context) error {
	return h.withClient(c, func(t, id, a uuid.UUID) (any, error) {
		x, e := h.service.ListDiffs(c.Request().Context(), t, id, a)
		return map[string]any{"items": x}, e
	})
}
func (h *ClinicalLongitudinalHandler) withClient(c echo.Context, fn func(uuid.UUID, uuid.UUID, uuid.UUID) (any, error)) error {
	t, p, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "client id must be a valid uuid")
	}
	out, err := fn(t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}
func (h *ClinicalLongitudinalHandler) GetDiff(c echo.Context) error {
	t, p, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "diff id must be a valid uuid")
	}
	out, err := h.service.GetDiff(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

type decideDiffOperationRequest struct {
	ExpectedDiffRevision int             `json:"expected_diff_revision"`
	Decision             string          `json:"decision"`
	Modification         json.RawMessage `json:"modification"`
}

func (h *ClinicalLongitudinalHandler) Decide(c echo.Context) error {
	t, p, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	diffID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "diff id must be a valid uuid")
	}
	opID, err := uuid.Parse(c.Param("operation_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "operation id must be a valid uuid")
	}
	var req decideDiffOperationRequest
	if err = c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	out, err := h.service.Decide(c.Request().Context(), longitudinal.DecisionInput{TenantID: t, DiffID: diffID, OperationID: opID, ActorID: p.UserID, ExpectedDiffRevision: req.ExpectedDiffRevision, Decision: req.Decision, Modification: req.Modification})
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

type mergeClinicalDiffRequest struct {
	ExpectedDiffRevision int `json:"expected_diff_revision"`
}

func (h *ClinicalLongitudinalHandler) Merge(c echo.Context) error {
	t, p, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "diff id must be a valid uuid")
	}
	var req mergeClinicalDiffRequest
	if err = c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	out, err := h.service.Merge(c.Request().Context(), longitudinal.MergeInput{TenantID: t, DiffID: id, ActorID: p.UserID, ExpectedDiffRevision: req.ExpectedDiffRevision})
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}
func handleLongitudinalError(c echo.Context, err error) error {
	return handleDomainError(c, err, []domainErrorMapping{{Target: domainerrors.ErrValidation, Status: http.StatusBadRequest, Code: "validation_error"}, {Target: domainerrors.ErrNotFound, Status: http.StatusNotFound, Code: "not_found", Message: "longitudinal resource not found"}, {Target: domainerrors.ErrForbidden, Status: http.StatusForbidden, Code: "forbidden", Message: "clinical assignment required"}, {Target: domainerrors.ErrConflict, Status: http.StatusConflict, Code: "conflict", Message: "longitudinal version or state conflict"}})
}
