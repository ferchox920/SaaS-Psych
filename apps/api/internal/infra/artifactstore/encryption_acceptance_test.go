package artifactstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestEncryptedArtifactWrongKeyTamperTruncation(t *testing.T) {
	for _, attack := range []string{"wrong-key-material", "tamper", "truncate"} {
		t.Run(attack, func(t *testing.T) {
			root := t.TempDir()
			s, err := New(root, "same-key-id", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)), 1024)
			if err != nil {
				t.Fatal(err)
			}
			id := Identity{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
			plain := []byte("synthetic audio plaintext never persisted")
			receipt, err := s.Write(context.Background(), id, bytes.NewReader(plain))
			if err != nil {
				t.Fatal(err)
			}
			path, _ := s.path(receipt.Key)
			raw, err := os.ReadFile(path)
			if err != nil || bytes.Contains(raw, plain) {
				t.Fatal("plaintext persisted")
			}
			reader := s
			switch attack {
			case "wrong-key-material":
				reader, err = New(root, "same-key-id", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)), 1024)
			case "tamper":
				raw[len(raw)-1] ^= 0xff
				err = os.WriteFile(path, raw, 0600)
			case "truncate":
				err = os.WriteFile(path, raw[:len(raw)/2], 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if out, e := reader.Read(context.Background(), receipt.Key, receipt.Hash, receipt.KeyID); e == nil || out != nil {
				t.Fatal("corrupt ciphertext released plaintext")
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 {
				t.Fatal("upload left temporary residue")
			}
		})
	}
}
