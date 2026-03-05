package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	auditusecase "sessionflow/apps/api/internal/usecase/audit"
)

type AuditLister interface {
	List(ctx context.Context, filter auditusecase.ListFilter) (auditusecase.ListResult, error)
}

type AuditHandler struct {
	service AuditLister
}

func NewAuditHandler(service AuditLister) *AuditHandler {
	return &AuditHandler{service: service}
}

type listAuditResponse struct {
	Items      []auditItemResponse `json:"items"`
	Pagination auditPagination     `json:"pagination"`
}

type auditItemResponse struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	ActorUserID *string        `json:"actor_user_id,omitempty"`
	Action      string         `json:"action"`
	Entity      string         `json:"entity"`
	EntityID    *string        `json:"entity_id,omitempty"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   string         `json:"created_at"`
}

type auditPagination struct {
	Limit        int     `json:"limit"`
	Offset       int     `json:"offset"`
	Count        int     `json:"count"`
	TotalCount   int     `json:"total_count"`
	NextCursor   *string `json:"next_cursor,omitempty"`
	NextCursorID *string `json:"next_cursor_id,omitempty"`
}

func (h *AuditHandler) List(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "audit service unavailable")
	}

	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}

	filter := auditusecase.ListFilter{
		TenantID: tenantID,
	}

	if actor := strings.TrimSpace(c.QueryParam("actor")); actor != "" {
		actorID, err := uuid.Parse(actor)
		if err != nil {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid actor query param")
		}
		filter.ActorUserID = &actorID
	}
	if action := strings.TrimSpace(c.QueryParam("action")); action != "" {
		filter.Action = action
	}
	if actionPrefix := strings.TrimSpace(c.QueryParam("action_prefix")); actionPrefix != "" {
		filter.ActionPrefix = actionPrefix
	}
	if entity := strings.TrimSpace(c.QueryParam("entity")); entity != "" {
		filter.Entity = entity
	}

	if from := strings.TrimSpace(c.QueryParam("from")); from != "" {
		fromTime, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid from query param, expected RFC3339")
		}
		filter.From = &fromTime
	}

	if to := strings.TrimSpace(c.QueryParam("to")); to != "" {
		toTime, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid to query param, expected RFC3339")
		}
		filter.To = &toTime
	}

	if cursor := strings.TrimSpace(c.QueryParam("cursor")); cursor != "" {
		cursorTime, err := time.Parse(time.RFC3339, cursor)
		if err != nil {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid cursor query param, expected RFC3339")
		}
		filter.Cursor = &cursorTime
	}
	if cursorID := strings.TrimSpace(c.QueryParam("cursor_id")); cursorID != "" {
		parsedCursorID, err := uuid.Parse(cursorID)
		if err != nil {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid cursor_id query param")
		}
		filter.CursorID = &parsedCursorID
	}

	if order := strings.ToLower(strings.TrimSpace(c.QueryParam("order"))); order != "" {
		filter.Order = order
	}

	if limit := strings.TrimSpace(c.QueryParam("limit")); limit != "" {
		value, err := strconv.Atoi(limit)
		if err != nil {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid limit query param")
		}
		filter.Limit = value
	}

	if offset := strings.TrimSpace(c.QueryParam("offset")); offset != "" {
		value, err := strconv.Atoi(offset)
		if err != nil {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid offset query param")
		}
		filter.Offset = value
	}

	result, err := h.service.List(c.Request().Context(), filter)
	if err != nil {
		return handleDomainError(c, err, defaultAuditErrorMappings())
	}

	responseItems := make([]auditItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		responseItem := auditItemResponse{
			ID:        item.ID.String(),
			TenantID:  item.TenantID.String(),
			Action:    item.Action,
			Entity:    item.Entity,
			Metadata:  item.Metadata,
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
		}
		if item.ActorUserID != nil {
			actor := item.ActorUserID.String()
			responseItem.ActorUserID = &actor
		}
		if item.EntityID != nil {
			entityID := item.EntityID.String()
			responseItem.EntityID = &entityID
		}
		responseItems = append(responseItems, responseItem)
	}

	var nextCursor *string
	if result.NextCursor != nil {
		cursor := result.NextCursor.UTC().Format(time.RFC3339)
		nextCursor = &cursor
	}
	var nextCursorID *string
	if result.NextCursorID != nil {
		cursorID := result.NextCursorID.String()
		nextCursorID = &cursorID
	}

	return c.JSON(http.StatusOK, listAuditResponse{
		Items: responseItems,
		Pagination: auditPagination{
			Limit:        result.Limit,
			Offset:       result.Offset,
			Count:        len(responseItems),
			TotalCount:   result.TotalCount,
			NextCursor:   nextCursor,
			NextCursorID: nextCursorID,
		},
	})
}
