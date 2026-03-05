package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

type AuthHandler struct {
	service *authusecase.Service
}

func NewAuthHandler(service *authusecase.Service) *AuthHandler {
	return &AuthHandler{service: service}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type meResponse struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "auth service unavailable")
	}

	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"})
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "email and password are required", map[string]any{"fields": []string{"email", "password"}})
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}

	out, err := h.service.Login(c.Request().Context(), tenantID, req.Email, req.Password)
	if err != nil {
		return h.handleAuthError(c, err)
	}

	return c.JSON(http.StatusOK, out)
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "auth service unavailable")
	}

	var req refreshRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"})
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "refresh_token is required", map[string]any{"field": "refresh_token"})
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}

	out, err := h.service.Refresh(c.Request().Context(), tenantID, req.RefreshToken)
	if err != nil {
		return h.handleAuthError(c, err)
	}

	return c.JSON(http.StatusOK, out)
}

func (h *AuthHandler) Logout(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "auth service unavailable")
	}

	var req logoutRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"})
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "refresh_token is required", map[string]any{"field": "refresh_token"})
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}

	if err := h.service.Logout(c.Request().Context(), tenantID, req.RefreshToken); err != nil {
		return h.handleAuthError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) Me(c echo.Context) error {
	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}

	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "auth context missing")
	}

	return c.JSON(http.StatusOK, meResponse{
		UserID:   principal.UserID.String(),
		TenantID: tenantID.String(),
		Role:     principal.Role,
	})
}

func (h *AuthHandler) handleAuthError(c echo.Context, err error) error {
	return handleDomainError(c, err, defaultAuthErrorMappings())
}
