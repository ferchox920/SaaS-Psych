package artifactstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestEncryptedArtifactIntegrityIsolationAndDelete(t *testing.T) {
	root := t.TempDir()
	store, err := New(root, "test-v1", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), 1024)
	if err != nil {
		t.Fatal(err)
	}
	id := Identity{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	ctx := context.Background()
	content := []byte("synthetic audio only")
	receipt, err := store.Write(ctx, id, bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	path, _ := store.path(receipt.Key)
	raw, err := os.ReadFile(path)
	if err != nil || bytes.Contains(raw, content) {
		t.Fatal("plaintext on disk")
	}
	decoded, err := store.Read(ctx, receipt.Key, receipt.Hash, receipt.KeyID)
	if err != nil || !bytes.Equal(decoded, content) {
		t.Fatal("read integrity")
	}
	if _, err = store.Write(ctx, id, bytes.NewReader(content)); err == nil {
		t.Fatal("existing artifact overwritten")
	}
	if _, err = store.Read(ctx, "../escape", receipt.Hash, receipt.KeyID); err == nil {
		t.Fatal("traversal allowed")
	}
	if _, err = store.Read(ctx, receipt.Key, receipt.Hash, "other-key"); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err = store.Read(ctx, receipt.Key, "wrong-hash", receipt.KeyID); err == nil {
		t.Fatal("wrong hash accepted")
	}
	other := id
	other.Client = uuid.New()
	if _, err = store.Read(ctx, other.Key(), receipt.Hash, receipt.KeyID); err == nil {
		t.Fatal("client collision")
	}
	if err = store.Delete(ctx, receipt.Key); err != nil {
		t.Fatal(err)
	}
	if err = store.Delete(ctx, receipt.Key); err != nil {
		t.Fatal("delete not idempotent")
	}
	if _, err = store.Read(ctx, receipt.Key, receipt.Hash, receipt.KeyID); err == nil {
		t.Fatal("deleted artifact readable")
	}
}
func TestArtifactBoundedWriteCancellationAndKeyRequirements(t *testing.T) {
	root := t.TempDir()
	if _, err := New(root, "key", "", 10); err == nil {
		t.Fatal("missing encryption key")
	}
	store, _ := New(root, "key", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32)), 10)
	id := Identity{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	if _, err := store.Write(context.Background(), id, bytes.NewReader(make([]byte, 11))); err == nil {
		t.Fatal("oversized artifact")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Write(ctx, id, bytes.NewReader([]byte("audio"))); err == nil {
		t.Fatal("cancelled write")
	}
	files, _ := os.ReadDir(root)
	if len(files) != 0 {
		t.Fatal("abandoned file after rejection")
	}
}
