package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	clinicalaccessusecase "sessionflow/apps/api/internal/usecase/clinicalaccess"
)

type ClinicalAccessHandler struct {
	service *clinicalaccessusecase.Service
	users   interface {
		ListTenantUsers(context.Context, uuid.UUID) ([]clinicalaccessusecase.TenantUser, error)
	}
}

func NewClinicalAccessHandler(service *clinicalaccessusecase.Service) *ClinicalAccessHandler {
	return &ClinicalAccessHandler{service: service}
}

func (h *ClinicalAccessHandler) WithUserDirectory(users interface {
	ListTenantUsers(context.Context, uuid.UUID) ([]clinicalaccessusecase.TenantUser, error)
}) *ClinicalAccessHandler {
	h.users = users
	return h
}

func (h *ClinicalAccessHandler) ListUsers(c echo.Context) error {
	tenantID, _, _, err := clinicalAssignmentContext(c)
	if err != nil {
		return err
	}
	if h.users == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "user directory unavailable")
	}
	users, err := h.users.ListTenantUsers(c.Request().Context(), tenantID)
	if err != nil {
		return err
	}
	items := make([]map[string]string, 0, len(users))
	for _, user := range users {
		items = append(items, map[string]string{"id": user.ID.String(), "email": user.Email})
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

type grantClinicalAssignmentRequest struct {
	UserID       string `json:"user_id"`
	Relationship string `json:"relationship"`
}

type endClinicalAssignmentRequest struct {
	Reason string `json:"reason"`
}

type grantAccessExceptionRequest struct {
	UserID    string `json:"user_id"`
	Reason    string `json:"reason"`
	Purpose   string `json:"purpose"`
	ExpiresAt string `json:"expires_at"`
}

type revokeAccessExceptionRequest struct {
	Reason string `json:"reason"`
}

type clinicalAssignmentResponse struct {
	ID              string  `json:"id"`
	ClientID        string  `json:"client_id"`
	UserID          string  `json:"user_id"`
	Relationship    string  `json:"relationship"`
	GrantedByUserID string  `json:"granted_by_user_id"`
	StartsAt        string  `json:"starts_at"`
	EndsAt          *string `json:"ends_at,omitempty"`
	EndedByUserID   *string `json:"ended_by_user_id,omitempty"`
	EndReason       string  `json:"end_reason,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

type accessExceptionResponse struct {
	ID              string  `json:"id"`
	ClientID        string  `json:"client_id"`
	UserID          string  `json:"user_id"`
	GrantedByUserID string  `json:"granted_by_user_id"`
	Reason          string  `json:"reason"`
	Purpose         string  `json:"purpose"`
	StartsAt        string  `json:"starts_at"`
	ExpiresAt       string  `json:"expires_at"`
	RevokedAt       *string `json:"revoked_at,omitempty"`
	RevokedByUserID *string `json:"revoked_by_user_id,omitempty"`
	RevokeReason    string  `json:"revoke_reason,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

func (h *ClinicalAccessHandler) Grant(c echo.Context) error {
	tenantID, principal, clientID, err := clinicalAssignmentContext(c)
	if err != nil {
		return err
	}
	var req grantClinicalAssignmentRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "user_id must be a valid uuid")
	}
	out, err := h.service.Grant(c.Request().Context(), clinicalaccessusecase.GrantInput{
		TenantID: tenantID, ClientID: clientID, UserID: userID,
		Relationship: req.Relationship, GrantedByUserID: principal.UserID,
	})
	if err != nil {
		return handleClinicalAccessError(c, err)
	}
	return c.JSON(http.StatusCreated, toClinicalAssignmentResponse(out))
}

func (h *ClinicalAccessHandler) List(c echo.Context) error {
	tenantID, principal, clientID, err := clinicalAssignmentContext(c)
	if err != nil {
		return err
	}
	items, err := h.service.List(c.Request().Context(), tenantID, clientID, principal.UserID)
	if err != nil {
		return handleClinicalAccessError(c, err)
	}
	response := make([]clinicalAssignmentResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toClinicalAssignmentResponse(item))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": response})
}

func (h *ClinicalAccessHandler) End(c echo.Context) error {
	tenantID, principal, clientID, err := clinicalAssignmentContext(c)
	if err != nil {
		return err
	}
	assignmentID, err := uuid.Parse(c.Param("assignment_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "assignment_id must be a valid uuid")
	}
	var req endClinicalAssignmentRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	if err := h.service.End(c.Request().Context(), tenantID, clientID, assignmentID, principal.UserID, req.Reason); err != nil {
		return handleClinicalAccessError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *ClinicalAccessHandler) GrantException(c echo.Context) error {
	tenantID, principal, clientID, err := clinicalAssignmentContext(c)
	if err != nil {
		return err
	}
	var req grantAccessExceptionRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "user_id must be a valid uuid")
	}
	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "expires_at must be RFC3339")
	}
	out, err := h.service.GrantException(c.Request().Context(), clinicalaccessusecase.GrantExceptionInput{
		TenantID: tenantID, ClientID: clientID, UserID: userID, GrantedByUserID: principal.UserID,
		Reason: req.Reason, Purpose: req.Purpose, ExpiresAt: expiresAt,
	})
	if err != nil {
		return handleClinicalAccessError(c, err)
	}
	return c.JSON(http.StatusCreated, toAccessExceptionResponse(out))
}

func (h *ClinicalAccessHandler) ListExceptions(c echo.Context) error {
	tenantID, principal, clientID, err := clinicalAssignmentContext(c)
	if err != nil {
		return err
	}
	items, err := h.service.ListExceptions(c.Request().Context(), tenantID, clientID, principal.UserID)
	if err != nil {
		return handleClinicalAccessError(c, err)
	}
	response := make([]accessExceptionResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toAccessExceptionResponse(item))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": response})
}

func (h *ClinicalAccessHandler) RevokeException(c echo.Context) error {
	tenantID, principal, clientID, err := clinicalAssignmentContext(c)
	if err != nil {
		return err
	}
	exceptionID, err := uuid.Parse(c.Param("exception_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "exception_id must be a valid uuid")
	}
	var req revokeAccessExceptionRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	if err := h.service.RevokeException(c.Request().Context(), tenantID, clientID, exceptionID, principal.UserID, req.Reason); err != nil {
		return handleClinicalAccessError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func clinicalAssignmentContext(c echo.Context) (uuid.UUID, httpmiddleware.Principal, uuid.UUID, error) {
	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return uuid.Nil, httpmiddleware.Principal{}, uuid.Nil, err
	}
	clientID, err := uuid.Parse(c.Param("client_id"))
	if err != nil {
		return uuid.Nil, httpmiddleware.Principal{}, uuid.Nil, writeAPIError(c, http.StatusBadRequest, "validation_error", "client_id must be a valid uuid")
	}
	return tenantID, principal, clientID, nil
}

func handleClinicalAccessError(c echo.Context, err error) error {
	return handleDomainError(c, err, []domainErrorMapping{
		{Target: domainerrors.ErrValidation, Status: http.StatusBadRequest, Code: "validation_error"},
		{Target: domainerrors.ErrNotFound, Status: http.StatusNotFound, Code: "not_found", Message: "clinical assignment not found"},
		{Target: domainerrors.ErrConflict, Status: http.StatusConflict, Code: "conflict", Message: "active assignment already exists"},
		{Target: domainerrors.ErrForbidden, Status: http.StatusForbidden, Code: "forbidden", Message: "cross-tenant assignment is not allowed"},
	})
}

func toClinicalAssignmentResponse(item clinicalaccessusecase.Assignment) clinicalAssignmentResponse {
	var endsAt, endedBy *string
	if item.EndsAt != nil {
		value := item.EndsAt.UTC().Format(time.RFC3339)
		endsAt = &value
	}
	if item.EndedByUserID != nil {
		value := item.EndedByUserID.String()
		endedBy = &value
	}
	return clinicalAssignmentResponse{
		ID: item.ID.String(), ClientID: item.ClientID.String(), UserID: item.UserID.String(),
		Relationship: item.Relationship, GrantedByUserID: item.GrantedByUserID.String(),
		StartsAt: item.StartsAt.UTC().Format(time.RFC3339), EndsAt: endsAt,
		EndedByUserID: endedBy, EndReason: item.EndReason, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toAccessExceptionResponse(item clinicalaccessusecase.AccessException) accessExceptionResponse {
	var revokedAt, revokedBy *string
	if item.RevokedAt != nil {
		value := item.RevokedAt.UTC().Format(time.RFC3339)
		revokedAt = &value
	}
	if item.RevokedByUserID != nil {
		value := item.RevokedByUserID.String()
		revokedBy = &value
	}
	return accessExceptionResponse{
		ID: item.ID.String(), ClientID: item.ClientID.String(), UserID: item.UserID.String(),
		GrantedByUserID: item.GrantedByUserID.String(), Reason: item.Reason, Purpose: item.Purpose,
		StartsAt: item.StartsAt.UTC().Format(time.RFC3339), ExpiresAt: item.ExpiresAt.UTC().Format(time.RFC3339),
		RevokedAt: revokedAt, RevokedByUserID: revokedBy, RevokeReason: item.RevokeReason,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
	}
}
