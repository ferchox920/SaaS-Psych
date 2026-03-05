package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type fakeRepo struct {
	lastFilter ListFilter
	result     []LogEntry
	total      int
	err        error
}

func (r *fakeRepo) ListAuditLogs(_ context.Context, filter ListFilter) ([]LogEntry, error) {
	r.lastFilter = filter
	return r.result, r.err
}

func (r *fakeRepo) CountAuditLogs(_ context.Context, _ ListFilter) (int, error) {
	return r.total, r.err
}

func TestList_DefaultPagination(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	service := NewService(repo)

	tenantID := uuid.New()
	result, err := service.List(context.Background(), ListFilter{TenantID: tenantID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Limit != 20 || result.Offset != 0 {
		t.Fatalf("expected default pagination 20/0, got %d/%d", result.Limit, result.Offset)
	}
	if repo.lastFilter.Limit != 20 || repo.lastFilter.Offset != 0 {
		t.Fatalf("expected forwarded filter 20/0, got %d/%d", repo.lastFilter.Limit, repo.lastFilter.Offset)
	}
}

func TestList_InvalidLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	service := NewService(repo)

	_, err := service.List(context.Background(), ListFilter{
		TenantID: uuid.New(),
		Limit:    101,
	})
	if err == nil {
		t.Fatalf("expected error for invalid limit")
	}
}

func TestList_InvalidOrder(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	service := NewService(repo)

	_, err := service.List(context.Background(), ListFilter{
		TenantID: uuid.New(),
		Order:    "sideways",
	})
	if err == nil {
		t.Fatalf("expected error for invalid order")
	}
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestList_NextCursorIncludesID(t *testing.T) {
	t.Parallel()

	entryID := uuid.New()
	repo := &fakeRepo{
		result: []LogEntry{
			{
				ID:        entryID,
				TenantID:  uuid.New(),
				CreatedAt: time.Now().UTC(),
			},
		},
		total: 1,
	}
	service := NewService(repo)

	result, err := service.List(context.Background(), ListFilter{
		TenantID: uuid.New(),
		Limit:    1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextCursor == nil {
		t.Fatalf("expected next cursor")
	}
	if result.NextCursorID == nil || *result.NextCursorID != entryID {
		t.Fatalf("expected next cursor id %s", entryID)
	}
}

func TestList_ForwardsActionPrefix(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	service := NewService(repo)

	_, err := service.List(context.Background(), ListFilter{
		TenantID:     uuid.New(),
		ActionPrefix: "auth.login",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastFilter.ActionPrefix != "auth.login" {
		t.Fatalf("expected forwarded action prefix auth.login, got %q", repo.lastFilter.ActionPrefix)
	}
}

func TestList_ActionAndActionPrefixMutuallyExclusive(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	service := NewService(repo)

	_, err := service.List(context.Background(), ListFilter{
		TenantID:     uuid.New(),
		Action:       "auth.login.success",
		ActionPrefix: "auth",
	})
	if err == nil {
		t.Fatalf("expected error for mutually exclusive action filters")
	}
}

func TestList_CursorIDRequiresCursor(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	service := NewService(repo)
	cursorID := uuid.New()

	_, err := service.List(context.Background(), ListFilter{
		TenantID: uuid.New(),
		CursorID: &cursorID,
	})
	if err == nil {
		t.Fatalf("expected error when cursor_id is set without cursor")
	}
}

func TestList_OffsetCannotBeUsedWithCursor(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	service := NewService(repo)
	cursor := time.Now().UTC()

	_, err := service.List(context.Background(), ListFilter{
		TenantID: uuid.New(),
		Cursor:   &cursor,
		Offset:   1,
	})
	if err == nil {
		t.Fatalf("expected error when using offset with cursor")
	}
}

func TestList_FromMustBeBeforeOrEqualToTo(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	service := NewService(repo)
	from := time.Now().UTC()
	to := from.Add(-1 * time.Minute)

	_, err := service.List(context.Background(), ListFilter{
		TenantID: uuid.New(),
		From:     &from,
		To:       &to,
	})
	if err == nil {
		t.Fatalf("expected error for invalid date range")
	}
}
