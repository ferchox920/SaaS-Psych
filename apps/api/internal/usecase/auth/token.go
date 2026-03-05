package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AccessClaims struct {
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type TokenService struct {
	accessSecret []byte
	accessTTL    time.Duration
}

func NewTokenService(accessSecret string, accessTTL time.Duration) *TokenService {
	return &TokenService{
		accessSecret: []byte(accessSecret),
		accessTTL:    accessTTL,
	}
}

func (s *TokenService) IssueAccessToken(userID, tenantID uuid.UUID, role string) (string, int64, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(s.accessTTL)

	claims := AccessClaims{
		TenantID: tenantID.String(),
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.accessSecret)
	if err != nil {
		return "", 0, fmt.Errorf("sign access token: %w", err)
	}

	return signed, int64(s.accessTTL.Seconds()), nil
}

func (s *TokenService) GenerateRefreshToken() (plain string, hash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generate random refresh token: %w", err)
	}

	plain = hex.EncodeToString(bytes)
	hash = HashRefreshToken(plain)
	return plain, hash, nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
