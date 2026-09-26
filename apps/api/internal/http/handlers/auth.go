package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

type AuthHandler struct {
	service      *authusecase.Service
	cookieConfig AuthCookieConfig
}

type AuthCookieConfig struct {
	Secure   bool
	SameSite http.SameSite
	Domain   string
	MaxAge   time.Duration
}

func NewAuthHandler(service *authusecase.Service, configs ...AuthCookieConfig) *AuthHandler {
	config := AuthCookieConfig{SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * time.Hour}
	if len(configs) > 0 {
		config = configs[0]
	}
	return &AuthHandler{service: service, cookieConfig: config}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
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

	h.setRefreshCookie(c, out.RefreshToken)
	return c.JSON(http.StatusOK, toAuthTokenResponse(out))
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "auth service unavailable")
	}

	refreshToken, err := h.refreshTokenFromCookie(c)
	if err != nil {
		return err
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}

	out, err := h.service.Refresh(c.Request().Context(), tenantID, refreshToken)
	if err != nil {
		return h.handleAuthError(c, err)
	}

	h.setRefreshCookie(c, out.RefreshToken)
	return c.JSON(http.StatusOK, toAuthTokenResponse(out))
}

func (h *AuthHandler) Logout(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "auth service unavailable")
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}
	cookie, err := c.Cookie(refreshCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		h.clearRefreshCookie(c)
		return c.NoContent(http.StatusNoContent)
	}

	if err := h.service.Logout(c.Request().Context(), tenantID, cookie.Value); err != nil {
		return h.handleAuthError(c, err)
	}

	h.clearRefreshCookie(c)
	return c.NoContent(http.StatusNoContent)
}

const refreshCookieName = "sessionflow_refresh"

func (h *AuthHandler) refreshTokenFromCookie(c echo.Context) (string, error) {
	cookie, err := c.Cookie(refreshCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", writeAPIError(c, http.StatusUnauthorized, "unauthorized", "refresh cookie is required")
	}
	return cookie.Value, nil
}

func (h *AuthHandler) setRefreshCookie(c echo.Context, token string) {
	maxAge := int(h.cookieConfig.MaxAge.Seconds())
	c.SetCookie(&http.Cookie{
		Name: refreshCookieName, Value: token, Path: "/api/v1/auth",
		Domain: h.cookieConfig.Domain, MaxAge: maxAge,
		HttpOnly: true, Secure: h.cookieConfig.Secure, SameSite: h.cookieConfig.SameSite,
	})
}

func (h *AuthHandler) clearRefreshCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name: refreshCookieName, Value: "", Path: "/api/v1/auth",
		Domain: h.cookieConfig.Domain, MaxAge: -1, Expires: time.Unix(1, 0),
		HttpOnly: true, Secure: h.cookieConfig.Secure, SameSite: h.cookieConfig.SameSite,
	})
}

func toAuthTokenResponse(out authusecase.LoginOutput) authTokenResponse {
	return authTokenResponse{AccessToken: out.AccessToken, TokenType: out.TokenType, ExpiresIn: out.ExpiresIn}
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
