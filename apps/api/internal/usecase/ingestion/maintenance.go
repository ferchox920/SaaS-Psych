package ingestion

import (
	"context"
	"errors"
	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"strings"
	"time"
)

// Unknown permanent files are retained for operator reconciliation, never
// silently destroyed. Only this safe code escapes the maintenance boundary.
var ErrStorageInconsistent = errors.New("artifact_storage_inconsistent")

type StoredEntry struct {
	Key        string
	Temporary  bool
	ModifiedAt time.Time
	Token      string
}
type MaintenanceStore interface {
	AudioStore
	Exists(context.Context, string) (bool, error)
	Scan(context.Context, uuid.UUID, int, int) ([]StoredEntry, int, error)
	RemoveEntry(context.Context, uuid.UUID, StoredEntry) error
}
type MaintenanceRepository interface {
	GetArtifact(context.Context, uuid.UUID, uuid.UUID) (Artifact, error)
	MaintenanceArtifacts(context.Context, uuid.UUID, uuid.UUID, int) ([]Artifact, error)
	BeginDeleteArtifact(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (Artifact, error)
	FinishDeleteArtifact(context.Context, uuid.UUID, uuid.UUID) error
	MarkArtifactMissing(context.Context, uuid.UUID, uuid.UUID) error
}

// Maintenance uses two bounded cursors: metadata UUID and directory offset.
// Repeated passes tolerate concurrent directory changes; nothing is recursively
// deleted. Unknown files and other tenants' files are never deletion targets.
type Maintenance struct {
	repo  MaintenanceRepository
	store MaintenanceStore
}

func NewMaintenance(r MaintenanceRepository, s MaintenanceStore) *Maintenance {
	return &Maintenance{r, s}
}
func (m *Maintenance) RunBatch(ctx context.Context, t, after uuid.UUID, offset int) (uuid.UUID, int, error) {
	items, err := m.repo.MaintenanceArtifacts(ctx, t, after, 50)
	if err != nil {
		return after, offset, err
	}
	next := uuid.Nil
	for _, v := range items {
		next = v.ID
		if v.Status == "deleting" || v.Status == "uploading" && !v.RetentionUntil.After(time.Now()) || v.Status == "available" && !v.RetentionUntil.After(time.Now()) || v.Status == "missing" && !v.RetentionUntil.After(time.Now()) {
			if _, err = m.repo.BeginDeleteArtifact(ctx, t, v.ID, v.ActorID); err != nil {
				return next, offset, err
			}
			if err := m.store.Delete(ctx, v.StorageKey); err != nil {
				return next, offset, err
			}
			if err := m.repo.FinishDeleteArtifact(ctx, t, v.ID); err != nil {
				return next, offset, err
			}
		} else if v.Status == "available" {
			exists, e := m.store.Exists(ctx, v.StorageKey)
			if e != nil {
				return next, offset, e
			}
			if !exists {
				if err := m.repo.MarkArtifactMissing(ctx, t, v.ID); err != nil {
					return next, offset, err
				}
			}
		}
	}
	if len(items) < 50 {
		next = uuid.Nil
	}
	entries, newOffset, err := m.store.Scan(ctx, t, offset, 100)
	if err != nil {
		return next, offset, err
	}
	for _, entry := range entries {
		// The storage implementation validates the tenant before any removal.
		if entry.Temporary {
			if time.Since(entry.ModifiedAt) < 24*time.Hour {
				continue
			}
			if err := m.store.RemoveEntry(ctx, t, entry); err != nil {
				return next, newOffset, err
			}
			continue
		}
		parts := strings.Split(entry.Key, "/")
		if len(parts) != 4 || parts[0] != t.String() {
			return next, newOffset, ErrStorageInconsistent
		}
		id, e := uuid.Parse(parts[3])
		if e != nil {
			return next, newOffset, ErrStorageInconsistent
		}
		v, e := m.repo.GetArtifact(ctx, t, id)
		if errors.Is(e, domainerrors.ErrNotFound) {
			return next, newOffset, ErrStorageInconsistent
		}
		if e != nil {
			return next, newOffset, e
		}
		if v.StorageKey != entry.Key {
			return next, newOffset, ErrStorageInconsistent
		}
		if v.Status == "deleted" {
			if err := m.store.Delete(ctx, v.StorageKey); err != nil {
				return next, newOffset, err
			}
		}
	}
	return next, newOffset, nil
}
