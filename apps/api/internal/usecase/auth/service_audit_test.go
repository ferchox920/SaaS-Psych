package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type fakeAuthRepository struct {
	user      User
	role      string
	tenantID  uuid.UUID
	email     string
	byHash    map[string]StoredRefreshToken
	byHashMtx sync.Mutex
	createErr error
}

func (r *fakeAuthRepository) GetUserByEmail(_ context.Context, tenantID uuid.UUID, email string) (User, error) {
	if tenantID != r.tenantID || email != r.email {
		return User{}, domainerrors.ErrNotFound
	}
	return r.user, nil
}

func (r *fakeAuthRepository) GetUserRole(_ context.Context, tenantID, userID uuid.UUID) (string, error) {
	if tenantID != r.tenantID || userID != r.user.ID {
		return "", domainerrors.ErrNotFound
	}
	return r.role, nil
}

func (r *fakeAuthRepository) CreateRefreshToken(_ context.Context, token RefreshTokenWrite) error {
	r.byHashMtx.Lock()
	defer r.byHashMtx.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	r.byHash[token.TokenHash] = StoredRefreshToken{
		UserID:    token.UserID,
		ExpiresAt: token.ExpiresAt,
	}
	return nil
}

func TestRefreshRollsBackOldTokenWhenReplacementCannotBeStored(t *testing.T) {
	t.Parallel()

	tenantID, userID := uuid.New(), uuid.New()
	oldPlain := "synthetic-old-refresh-token"
	oldHash := HashRefreshToken(oldPlain)
	repo := &fakeAuthRepository{
		user:      User{ID: userID},
		role:      "owner",
		tenantID:  tenantID,
		byHash:    map[string]StoredRefreshToken{oldHash: {UserID: userID, ExpiresAt: time.Now().UTC().Add(time.Hour)}},
		createErr: errors.New("synthetic insert failure"),
	}
	service := NewService(repo, NewTokenService("test-secret", 15*time.Minute), 30*24*time.Hour, nil)

	if _, err := service.Refresh(context.Background(), tenantID, oldPlain); err == nil {
		t.Fatal("expected replacement storage failure")
	}
	stored, err := repo.GetRefreshToken(context.Background(), tenantID, oldHash)
	if err != nil {
		t.Fatal(err)
	}
	if stored.RevokedAt != nil {
		t.Fatal("old refresh token was revoked even though replacement storage rolled back")
	}
}

func (r *fakeAuthRepository) GetRefreshToken(_ context.Context, tenantID uuid.UUID, tokenHash string) (StoredRefreshToken, error) {
	if tenantID != r.tenantID {
		return StoredRefreshToken{}, domainerrors.ErrNotFound
	}
	r.byHashMtx.Lock()
	defer r.byHashMtx.Unlock()
	token, ok := r.byHash[tokenHash]
	if !ok {
		return StoredRefreshToken{}, domainerrors.ErrNotFound
	}
	return token, nil
}

func (r *fakeAuthRepository) RotateRefreshToken(_ context.Context, tenantID uuid.UUID, oldTokenHash string, replacement RefreshTokenWrite, now time.Time) error {
	if tenantID != r.tenantID {
		return domainerrors.ErrNotFound
	}
	r.byHashMtx.Lock()
	defer r.byHashMtx.Unlock()
	old, ok := r.byHash[oldTokenHash]
	if !ok {
		return domainerrors.ErrNotFound
	}
	if old.RevokedAt != nil || !old.ExpiresAt.After(now) || replacement.TenantID != tenantID || replacement.UserID != old.UserID {
		return domainerrors.ErrUnauthorized
	}
	if r.createErr != nil {
		return r.createErr
	}
	old.RevokedAt = &now
	r.byHash[oldTokenHash] = old
	r.byHash[replacement.TokenHash] = StoredRefreshToken{UserID: replacement.UserID, ExpiresAt: replacement.ExpiresAt}
	return nil
}

func (r *fakeAuthRepository) RevokeRefreshToken(_ context.Context, tenantID uuid.UUID, tokenHash string) error {
	if tenantID != r.tenantID {
		return domainerrors.ErrNotFound
	}
	r.byHashMtx.Lock()
	defer r.byHashMtx.Unlock()
	token, ok := r.byHash[tokenHash]
	if !ok || token.RevokedAt != nil {
		return domainerrors.ErrNotFound
	}
	now := time.Now().UTC()
	token.RevokedAt = &now
	r.byHash[tokenHash] = token
	return nil
}

type fakeAuditor struct {
	mu      sync.Mutex
	actions []string
	err     error
}

func (a *fakeAuditor) RecordAuthEvent(_ context.Context, event AuthAuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.actions = append(a.actions, event.Action)
	return a.err
}

func TestService_AuditsLoginRefreshLogout(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Pass123!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}

	repo := &fakeAuthRepository{
		user: User{
			ID:           userID,
			PasswordHash: string(passwordHash),
		},
		role:     "owner",
		tenantID: tenantID,
		email:    "owner@test.local",
		byHash:   make(map[string]StoredRefreshToken),
	}
	auditor := &fakeAuditor{}
	service := NewService(repo, NewTokenService("test-secret", 15*time.Minute), 30*24*time.Hour, auditor)

	loginOut, err := service.Login(context.Background(), tenantID, "owner@test.local", "Pass123!")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	refreshOut, err := service.Refresh(context.Background(), tenantID, loginOut.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if err := service.Logout(context.Background(), tenantID, refreshOut.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}

	want := []string{"auth.login.success", "auth.refresh.success", "auth.logout.success"}
	if len(auditor.actions) != len(want) {
		t.Fatalf("expected %d audit events, got %d (%v)", len(want), len(auditor.actions), auditor.actions)
	}
	for i := range want {
		if auditor.actions[i] != want[i] {
			t.Fatalf("expected action[%d]=%q, got %q", i, want[i], auditor.actions[i])
		}
	}
}

func TestService_AuditFailureDoesNotFailLogin(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Pass123!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}

	repo := &fakeAuthRepository{
		user: User{
			ID:           userID,
			PasswordHash: string(passwordHash),
		},
		role:     "owner",
		tenantID: tenantID,
		email:    "owner@test.local",
		byHash:   make(map[string]StoredRefreshToken),
	}
	auditor := &fakeAuditor{err: errors.New("audit down")}
	service := NewService(repo, NewTokenService("test-secret", 15*time.Minute), 30*24*time.Hour, auditor)

	if _, err := service.Login(context.Background(), tenantID, "owner@test.local", "Pass123!"); err != nil {
		t.Fatalf("login should succeed even if audit fails, got error: %v", err)
	}
}
