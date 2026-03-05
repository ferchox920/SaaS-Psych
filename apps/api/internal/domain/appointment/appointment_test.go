package appointment

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

func TestNewEntityValidatesDateRange(t *testing.T) {
	at := time.Now()
	_, err := NewEntity(uuid.New(), uuid.New(), at, at, "", time.Time{})
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestOverlaps(t *testing.T) {
	start := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	if !Overlaps(start, end, start.Add(30*time.Minute), end.Add(30*time.Minute)) {
		t.Fatalf("expected overlap")
	}
	if Overlaps(start, end, end, end.Add(time.Hour)) {
		t.Fatalf("expected no overlap on touching ranges")
	}
}
