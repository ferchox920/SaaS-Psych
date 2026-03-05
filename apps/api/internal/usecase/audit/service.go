package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type Repository interface {
	ListAuditLogs(ctx context.Context, filter ListFilter) ([]LogEntry, error)
	CountAuditLogs(ctx context.Context, filter ListFilter) (int, error)
}

type ListFilter struct {
	TenantID     uuid.UUID
	ActorUserID  *uuid.UUID
	Action       string
	ActionPrefix string
	Entity       string
	From         *time.Time
	To           *time.Time
	Cursor       *time.Time
	CursorID     *uuid.UUID
	Order        string
	Limit        int
	Offset       int
}

type LogEntry struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ActorUserID *uuid.UUID
	Action      string
	Entity      string
	EntityID    *uuid.UUID
	Metadata    map[string]any
	CreatedAt   time.Time
}

type Service struct {
	repo Repository
}

type ListResult struct {
	Items        []LogEntry
	Limit        int
	Offset       int
	TotalCount   int
	NextCursor   *time.Time
	NextCursorID *uuid.UUID
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	if filter.TenantID == uuid.Nil {
		return ListResult{}, domainerrors.NewValidation("tenant_id is required")
	}
	if filter.Action != "" && filter.ActionPrefix != "" {
		return ListResult{}, domainerrors.NewValidation("action and action_prefix are mutually exclusive")
	}
	if filter.CursorID != nil && filter.Cursor == nil {
		return ListResult{}, domainerrors.NewValidation("cursor_id requires cursor")
	}

	limit := filter.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 0 || limit > maxLimit {
		return ListResult{}, domainerrors.NewValidation(fmt.Sprintf("limit must be between 1 and %d", maxLimit))
	}

	offset := filter.Offset
	if offset < 0 {
		return ListResult{}, domainerrors.NewValidation("offset must be >= 0")
	}
	if filter.Cursor != nil && offset > 0 {
		return ListResult{}, domainerrors.NewValidation("offset cannot be used with cursor")
	}

	order := filter.Order
	if order == "" {
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		return ListResult{}, domainerrors.NewValidation("order must be one of: asc, desc")
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return ListResult{}, domainerrors.NewValidation("from must be <= to")
	}

	filter.Limit = limit
	filter.Offset = offset
	filter.Order = order

	entries, err := s.repo.ListAuditLogs(ctx, filter)
	if err != nil {
		return ListResult{}, fmt.Errorf("list audit logs: %w", err)
	}
	totalCount, err := s.repo.CountAuditLogs(ctx, filter)
	if err != nil {
		return ListResult{}, fmt.Errorf("count audit logs: %w", err)
	}

	var nextCursor *time.Time
	var nextCursorID *uuid.UUID
	if len(entries) > 0 && totalCount > (offset+len(entries)) {
		last := entries[len(entries)-1]
		cursor := last.CreatedAt.UTC()
		cursorID := last.ID
		nextCursor = &cursor
		nextCursorID = &cursorID
	}

	return ListResult{
		Items:        entries,
		Limit:        limit,
		Offset:       offset,
		TotalCount:   totalCount,
		NextCursor:   nextCursor,
		NextCursorID: nextCursorID,
	}, nil
}
