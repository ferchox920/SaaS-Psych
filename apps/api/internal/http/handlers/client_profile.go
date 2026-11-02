package handlers

import (
	"github.com/labstack/echo/v4"
	"net/http"
	domainclient "sessionflow/apps/api/internal/domain/client"
)

func (h *ClientHandler) GetProfile(c echo.Context) error {
	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	clientID, err := parseClientID(c.Param("id"))
	if err != nil {
		return h.handleClientError(c, err)
	}
	profile, err := h.service.GetProfile(c.Request().Context(), tenantID, clientID, principal.UserID)
	if err != nil {
		return h.handleClientError(c, err)
	}
	return c.JSON(http.StatusOK, profile)
}

func (h *ClientHandler) SaveProfile(c echo.Context) error {
	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	clientID, err := parseClientID(c.Param("id"))
	if err != nil {
		return h.handleClientError(c, err)
	}
	var profile domainclient.Profile
	if err = c.Bind(&profile); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid profile")
	}
	profile, err = h.service.SaveProfile(c.Request().Context(), tenantID, clientID, principal.UserID, profile)
	if err != nil {
		return h.handleClientError(c, err)
	}
	return c.JSON(http.StatusOK, profile)
}
