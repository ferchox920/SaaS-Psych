package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Repository interface {
	GetUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (User, error)
	GetUserRole(ctx context.Context, tenantID, userID uuid.UUID) (string, error)
	CreateRefreshToken(ctx context.Context, token RefreshTokenWrite) error
	GetRefreshToken(ctx context.Context, tenantID uuid.UUID, tokenHash string) (StoredRefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tenantID uuid.UUID, tokenHash string) error
}

type Auditor interface {
	RecordAuthEvent(ctx context.Context, event AuthAuditEvent) error
}

type AuthAuditEvent struct {
	TenantID    uuid.UUID
	ActorUserID uuid.UUID
	Action      string
	Metadata    map[string]any
}

type User struct {
	ID           uuid.UUID
	PasswordHash string
}

type RefreshTokenWrite struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
}

type StoredRefreshToken struct {
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type LoginOutput struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type Service struct {
	repo       Repository
	token      *TokenService
	refreshTTL time.Duration
	auditor    Auditor
	metrics    Metrics
}

type Metrics interface {
	RecordAuthError(endpoint string, err error)
}

func NewService(repo Repository, token *TokenService, refreshTTL time.Duration, auditor Auditor) *Service {
	return &Service{
		repo:       repo,
		token:      token,
		refreshTTL: refreshTTL,
		auditor:    auditor,
	}
}

func (s *Service) WithMetrics(metrics Metrics) *Service {
	s.metrics = metrics
	return s
}

func (s *Service) Login(ctx context.Context, tenantID uuid.UUID, email, password string) (LoginOutput, error) {
	if tenantID == uuid.Nil {
		err := domainerrors.NewValidation("tenant_id is required")
		s.recordAuthError("login", err)
		return LoginOutput{}, err
	}
	if strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" {
		err := domainerrors.NewValidation("email and password are required")
		s.recordAuthError("login", err)
		return LoginOutput{}, err
	}

	user, err := s.repo.GetUserByEmail(ctx, tenantID, email)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			err = domainerrors.ErrUnauthorized
			s.recordAuthError("login", err)
			return LoginOutput{}, err
		}

		err = fmt.Errorf("find user by email: %w", err)
		s.recordAuthError("login", err)
		return LoginOutput{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		err = domainerrors.ErrUnauthorized
		s.recordAuthError("login", err)
		return LoginOutput{}, err
	}

	role, err := s.repo.GetUserRole(ctx, tenantID, user.ID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			err = domainerrors.ErrUnauthorized
			s.recordAuthError("login", err)
			return LoginOutput{}, err
		}

		err = fmt.Errorf("get user role: %w", err)
		s.recordAuthError("login", err)
		return LoginOutput{}, err
	}

	accessToken, expiresIn, err := s.token.IssueAccessToken(user.ID, tenantID, role)
	if err != nil {
		err = fmt.Errorf("issue access token: %w", err)
		s.recordAuthError("login", err)
		return LoginOutput{}, err
	}

	refreshPlain, refreshHash, err := s.token.GenerateRefreshToken()
	if err != nil {
		err = fmt.Errorf("generate refresh token: %w", err)
		s.recordAuthError("login", err)
		return LoginOutput{}, err
	}

	if err := s.repo.CreateRefreshToken(ctx, RefreshTokenWrite{
		TenantID:  tenantID,
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().UTC().Add(s.refreshTTL),
	}); err != nil {
		err = fmt.Errorf("store refresh token: %w", err)
		s.recordAuthError("login", err)
		return LoginOutput{}, err
	}
	s.recordAudit(ctx, AuthAuditEvent{
		TenantID:    tenantID,
		ActorUserID: user.ID,
		Action:      "auth.login.success",
		Metadata: map[string]any{
			"method": "password",
		},
	})

	return LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshPlain,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, tenantID uuid.UUID, refreshToken string) (LoginOutput, error) {
	if tenantID == uuid.Nil {
		err := domainerrors.NewValidation("tenant_id is required")
		s.recordAuthError("refresh", err)
		return LoginOutput{}, err
	}
	if strings.TrimSpace(refreshToken) == "" {
		err := domainerrors.NewValidation("refresh_token is required")
		s.recordAuthError("refresh", err)
		return LoginOutput{}, err
	}

	tokenHash := HashRefreshToken(refreshToken)
	stored, err := s.repo.GetRefreshToken(ctx, tenantID, tokenHash)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			err = domainerrors.ErrUnauthorized
			s.recordAuthError("refresh", err)
			return LoginOutput{}, err
		}

		err = fmt.Errorf("load refresh token: %w", err)
		s.recordAuthError("refresh", err)
		return LoginOutput{}, err
	}

	if stored.RevokedAt != nil || time.Now().UTC().After(stored.ExpiresAt) {
		err = domainerrors.ErrUnauthorized
		s.recordAuthError("refresh", err)
		return LoginOutput{}, err
	}

	role, err := s.repo.GetUserRole(ctx, tenantID, stored.UserID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			err = domainerrors.ErrUnauthorized
			s.recordAuthError("refresh", err)
			return LoginOutput{}, err
		}

		err = fmt.Errorf("get user role: %w", err)
		s.recordAuthError("refresh", err)
		return LoginOutput{}, err
	}

	if err := s.repo.RevokeRefreshToken(ctx, tenantID, tokenHash); err != nil {
		err = fmt.Errorf("revoke old refresh token: %w", err)
		s.recordAuthError("refresh", err)
		return LoginOutput{}, err
	}

	accessToken, expiresIn, err := s.token.IssueAccessToken(stored.UserID, tenantID, role)
	if err != nil {
		err = fmt.Errorf("issue access token: %w", err)
		s.recordAuthError("refresh", err)
		return LoginOutput{}, err
	}

	newPlain, newHash, err := s.token.GenerateRefreshToken()
	if err != nil {
		err = fmt.Errorf("generate refresh token: %w", err)
		s.recordAuthError("refresh", err)
		return LoginOutput{}, err
	}

	if err := s.repo.CreateRefreshToken(ctx, RefreshTokenWrite{
		TenantID:  tenantID,
		UserID:    stored.UserID,
		TokenHash: newHash,
		ExpiresAt: time.Now().UTC().Add(s.refreshTTL),
	}); err != nil {
		err = fmt.Errorf("store rotated refresh token: %w", err)
		s.recordAuthError("refresh", err)
		return LoginOutput{}, err
	}
	s.recordAudit(ctx, AuthAuditEvent{
		TenantID:    tenantID,
		ActorUserID: stored.UserID,
		Action:      "auth.refresh.success",
		Metadata: map[string]any{
			"rotation": true,
		},
	})

	return LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: newPlain,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	}, nil
}

func (s *Service) Logout(ctx context.Context, tenantID uuid.UUID, refreshToken string) error {
	if tenantID == uuid.Nil {
		err := domainerrors.NewValidation("tenant_id is required")
		s.recordAuthError("logout", err)
		return err
	}
	if strings.TrimSpace(refreshToken) == "" {
		err := domainerrors.NewValidation("refresh_token is required")
		s.recordAuthError("logout", err)
		return err
	}

	tokenHash := HashRefreshToken(refreshToken)
	stored, err := s.repo.GetRefreshToken(ctx, tenantID, tokenHash)
	if err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return nil
		}

		err = fmt.Errorf("load refresh token for logout: %w", err)
		s.recordAuthError("logout", err)
		return err
	}
	if err := s.repo.RevokeRefreshToken(ctx, tenantID, tokenHash); err != nil {
		if errors.Is(err, domainerrors.ErrNotFound) {
			return nil
		}

		err = fmt.Errorf("revoke refresh token: %w", err)
		s.recordAuthError("logout", err)
		return err
	}
	s.recordAudit(ctx, AuthAuditEvent{
		TenantID:    tenantID,
		ActorUserID: stored.UserID,
		Action:      "auth.logout.success",
		Metadata: map[string]any{
			"reason": "user_logout",
		},
	})

	return nil
}

func (s *Service) recordAudit(ctx context.Context, event AuthAuditEvent) {
	if s.auditor == nil {
		return
	}

	_ = s.auditor.RecordAuthEvent(ctx, event)
}

func (s *Service) recordAuthError(endpoint string, err error) {
	if s.metrics == nil || err == nil {
		return
	}
	s.metrics.RecordAuthError(endpoint, err)
}
