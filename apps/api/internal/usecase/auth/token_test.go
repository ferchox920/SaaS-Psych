package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIssueAccessToken(t *testing.T) {
	t.Parallel()

	service := NewTokenService("secret", 15*time.Minute)
	token, expiresIn, err := service.IssueAccessToken(uuid.New(), uuid.New(), "member")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatalf("expected token to be generated")
	}
	if expiresIn != 900 {
		t.Fatalf("expected expiresIn=900, got %d", expiresIn)
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	t.Parallel()

	service := NewTokenService("secret", 15*time.Minute)
	plain, hash, err := service.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if plain == "" || hash == "" {
		t.Fatalf("expected plain/hash to be non-empty")
	}
	if HashRefreshToken(plain) != hash {
		t.Fatalf("hash mismatch for generated token")
	}
}
