// Package artifactstore stores only encrypted local clinical audio. Callers
// receive opaque UUID keys; no user-supplied filename is ever accepted.
package artifactstore

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"sessionflow/apps/api/internal/usecase/ingestion"
)

var ErrIntegrity = errors.New("artifact integrity check failed")

type Identity struct{ Tenant, Client, Session, Artifact uuid.UUID }

func (i Identity) Key() string {
	return strings.Join([]string{i.Tenant.String(), i.Client.String(), i.Session.String(), i.Artifact.String()}, "/")
}
func ParseKey(key string) (Identity, error) {
	parts := strings.Split(key, "/")
	if len(parts) != 4 {
		return Identity{}, ErrIntegrity
	}
	ids := make([]uuid.UUID, 4)
	for n, p := range parts {
		v, err := uuid.Parse(p)
		if err != nil || v == uuid.Nil || v.String() != p {
			return Identity{}, ErrIntegrity
		}
		ids[n] = v
	}
	return Identity{ids[0], ids[1], ids[2], ids[3]}, nil
}

type Receipt struct {
	Key, Hash, KeyID string
	Size             int64
}
type Store struct {
	root, keyID string
	aead        cipher.AEAD
	maxBytes    int64
}

func (s *Store) WriteAudio(ctx context.Context, key string, input io.Reader) (ingestion.AudioReceipt, error) {
	id, err := ParseKey(key)
	if err != nil {
		return ingestion.AudioReceipt{}, err
	}
	r, err := s.Write(ctx, id, input)
	return ingestion.AudioReceipt{Key: r.Key, Hash: r.Hash, KeyID: r.KeyID, Size: r.Size}, err
}

func New(root, keyID, encodedKey string, maxBytes int64) (*Store, error) {
	if !filepath.IsAbs(root) || keyID == "" || maxBytes < 1 {
		return nil, errors.New("absolute artifact root, key ID and size limit are required")
	}
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("artifact encryption key must be 32 base64-encoded bytes")
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, errors.New("artifact storage unavailable")
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil || !strings.EqualFold(filepath.Clean(real), filepath.Clean(root)) {
		return nil, errors.New("artifact root must not be a symbolic link")
	}
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	return &Store{root: root, keyID: keyID, aead: aead, maxBytes: maxBytes}, nil
}
func (s *Store) path(key string) (string, error) {
	if _, err := ParseKey(key); err != nil {
		return "", err
	}
	// Flat filenames avoid intermediate tenant directories or reparse-point traversal.
	return filepath.Join(s.root, strings.ReplaceAll(key, "/", "_")+".audio.enc"), nil
}
func (s *Store) Write(ctx context.Context, id Identity, input io.Reader) (Receipt, error) {
	key := id.Key()
	path, err := s.path(key)
	if err != nil {
		return Receipt{}, err
	}
	// Bounded buffering permits a single authenticated AES-GCM envelope; plaintext
	// is never written to a temporary file. The configured cap is mandatory.
	data, err := io.ReadAll(io.LimitReader(input, s.maxBytes+1))
	if err != nil || len(data) == 0 || int64(len(data)) > s.maxBytes {
		return Receipt{}, errors.New("invalid artifact size or stream")
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	hash := sha256.Sum256(data)
	nonce := make([]byte, s.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return Receipt{}, err
	}
	encrypted := append([]byte("SFA1"), nonce...)
	encrypted = s.aead.Seal(encrypted, nonce, data, []byte(key+"/"+s.keyID))
	file, err := os.CreateTemp(s.root, strings.ReplaceAll(key, "/", "_")+".upload-*.tmp")
	if err != nil {
		return Receipt{}, errors.New("artifact storage unavailable")
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err = file.Write(encrypted); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return Receipt{}, errors.New("artifact write failed")
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	// Hard-link publication is atomic and refuses an existing target. Both paths
	// are in the same directory/filesystem; the temporary encrypted link is removed.
	if err = os.Link(temporary, path); err != nil {
		return Receipt{}, errors.New("artifact finalize failed")
	}
	return Receipt{Key: key, Hash: hex.EncodeToString(hash[:]), KeyID: s.keyID, Size: int64(len(data))}, nil
}
func (s *Store) Read(ctx context.Context, key, hash, keyID string) ([]byte, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, os.ErrNotExist
	}
	if !info.Mode().IsRegular() || info.Size() > s.maxBytes+128 {
		return nil, ErrIntegrity
	}
	if keyID != s.keyID {
		return nil, ErrIntegrity
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, os.ErrNotExist
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, s.maxBytes+129))
	if err != nil {
		return nil, ErrIntegrity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n := s.aead.NonceSize()
	if len(raw) < 4+n || string(raw[:4]) != "SFA1" {
		return nil, ErrIntegrity
	}
	data, err := s.aead.Open(nil, raw[4:4+n], raw[4+n:], []byte(key+"/"+keyID))
	if err != nil {
		return nil, ErrIntegrity
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != hash {
		return nil, ErrIntegrity
	}
	return data, nil
}
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
