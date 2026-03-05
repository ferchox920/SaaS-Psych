package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	clientusecase "sessionflow/apps/api/internal/usecase/client"
)

type ClientHandler struct {
	service *clientusecase.Service
}

func NewClientHandler(service *clientusecase.Service) *ClientHandler {
	return &ClientHandler{service: service}
}

type upsertClientRequest struct {
	FullName    string `json:"fullname"`
	Contact     string `json:"contact"`
	NotesPublic string `json:"notes_public"`
}

type clientResponse struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	FullName    string `json:"fullname"`
	Contact     string `json:"contact"`
	NotesPublic string `json:"notes_public"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (h *ClientHandler) Create(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "client service unavailable")
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}
	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "auth context missing")
	}

	var req upsertClientRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"})
	}

	out, err := h.service.Create(c.Request().Context(), clientusecase.CreateInput{
		TenantID:    tenantID,
		ActorUserID: principal.UserID,
		FullName:    req.FullName,
		Contact:     req.Contact,
		NotesPublic: req.NotesPublic,
	})
	if err != nil {
		return h.handleClientError(c, err)
	}

	return c.JSON(http.StatusCreated, toClientResponse(out))
}

func (h *ClientHandler) List(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "client service unavailable")
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}

	items, err := h.service.List(c.Request().Context(), tenantID)
	if err != nil {
		return h.handleClientError(c, err)
	}

	response := make([]clientResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toClientResponse(item))
	}

	return c.JSON(http.StatusOK, map[string]any{"items": response})
}

func (h *ClientHandler) Get(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "client service unavailable")
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}

	clientID, err := parseClientID(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", err.Error(), map[string]any{"field": "id"})
	}

	out, err := h.service.Get(c.Request().Context(), tenantID, clientID)
	if err != nil {
		return h.handleClientError(c, err)
	}

	return c.JSON(http.StatusOK, toClientResponse(out))
}

func (h *ClientHandler) Update(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "client service unavailable")
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}
	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "auth context missing")
	}

	clientID, err := parseClientID(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", err.Error(), map[string]any{"field": "id"})
	}

	var req upsertClientRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"})
	}

	out, err := h.service.Update(c.Request().Context(), clientusecase.UpdateInput{
		TenantID:    tenantID,
		ActorUserID: principal.UserID,
		ClientID:    clientID,
		FullName:    req.FullName,
		Contact:     req.Contact,
		NotesPublic: req.NotesPublic,
	})
	if err != nil {
		return h.handleClientError(c, err)
	}

	return c.JSON(http.StatusOK, toClientResponse(out))
}

func (h *ClientHandler) Delete(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "client service unavailable")
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}
	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "auth context missing")
	}

	clientID, err := parseClientID(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", err.Error(), map[string]any{"field": "id"})
	}

	if err := h.service.Delete(c.Request().Context(), tenantID, clientID, principal.UserID); err != nil {
		return h.handleClientError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func parseClientID(raw string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return uuid.Nil, domainerrors.NewValidation("client id is required")
	}

	clientID, err := uuid.Parse(trimmed)
	if err != nil {
		return uuid.Nil, domainerrors.NewValidation("client id must be a valid uuid")
	}

	return clientID, nil
}

func (h *ClientHandler) handleClientError(c echo.Context, err error) error {
	return handleDomainError(c, err, defaultClientErrorMappings())
}

func defaultClientErrorMappings() []domainErrorMapping {
	return []domainErrorMapping{
		{
			Target: domainerrors.ErrValidation,
			Status: http.StatusBadRequest,
			Code:   "validation_error",
		},
		{
			Target:  domainerrors.ErrNotFound,
			Status:  http.StatusNotFound,
			Code:    "not_found",
			Message: "client not found",
		},
	}
}

func toClientResponse(entity domainclient.Entity) clientResponse {
	return clientResponse{
		ID:          entity.ID.String(),
		TenantID:    entity.TenantID.String(),
		FullName:    entity.FullName,
		Contact:     entity.Contact,
		NotesPublic: entity.NotesPublic,
		CreatedAt:   entity.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   entity.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}
