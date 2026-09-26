package artifactstore

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"strings"
)

func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	path, err := s.path(key)
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("storage unavailable")
	}
	if !info.Mode().IsRegular() {
		return false, ErrIntegrity
	}
	return true, nil
}
func (s *Store) Scan(ctx context.Context, t uuid.UUID, offset, limit int) ([]ingestion.StoredEntry, int, error) {
	if t == uuid.Nil || offset < 0 || offset > 100000 || limit < 1 || limit > 100 {
		return nil, offset, ErrIntegrity
	}
	dir, err := os.Open(s.root)
	if err != nil {
		return nil, offset, errors.New("storage unavailable")
	}
	defer dir.Close()
	for skipped := 0; skipped < offset; {
		if err := ctx.Err(); err != nil {
			return nil, offset, err
		}
		n := min(100, offset-skipped)
		names, e := dir.Readdirnames(n)
		skipped += len(names)
		if e == io.EOF {
			return []ingestion.StoredEntry{}, 0, nil
		}
		if e != nil {
			return nil, offset, errors.New("storage unavailable")
		}
	}
	names, err := dir.Readdirnames(limit)
	if err != nil && err != io.EOF {
		return nil, offset, errors.New("storage unavailable")
	}
	next := offset + len(names)
	if len(names) < limit || next >= 100000 {
		next = 0
	}
	out := []ingestion.StoredEntry{}
	for _, name := range names {
		if !strings.HasPrefix(name, t.String()+"_") {
			continue
		}
		stem := strings.TrimSuffix(name, ".audio.enc")
		temporary := false
		if index := strings.Index(name, ".upload-"); index >= 0 && strings.HasSuffix(name, ".tmp") {
			stem = name[:index]
			temporary = true
		}
		key := strings.ReplaceAll(stem, "_", "/")
		id, e := ParseKey(key)
		if e != nil || id.Tenant != t {
			continue
		}
		info, e := os.Lstat(filepath.Join(s.root, name))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, offset, errors.New("storage unavailable")
		}
		if !info.Mode().IsRegular() {
			continue
		}
		out = append(out, ingestion.StoredEntry{Key: key, Temporary: temporary, ModifiedAt: info.ModTime(), Token: name})
	}
	return out, next, nil
}
func (s *Store) RemoveEntry(ctx context.Context, t uuid.UUID, entry ingestion.StoredEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := ParseKey(entry.Key)
	if err != nil || id.Tenant != t {
		return ErrIntegrity
	}
	prefix := strings.ReplaceAll(entry.Key, "/", "_") + ".upload-"
	if !entry.Temporary || filepath.Base(entry.Token) != entry.Token || !strings.HasPrefix(entry.Token, prefix) || !strings.HasSuffix(entry.Token, ".tmp") {
		return ErrIntegrity
	}
	path := filepath.Join(s.root, entry.Token)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || !info.ModTime().Equal(entry.ModifiedAt) {
		return ErrIntegrity
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
