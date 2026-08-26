package googlecalendar

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domaincalendar "sessionflow/apps/api/internal/domain/googlecalendar"
	googleinfra "sessionflow/apps/api/internal/infra/googlecalendar"
	appointmentusecase "sessionflow/apps/api/internal/usecase/appointment"
)

var (
	ErrDisabled                = errors.New("google calendar integration disabled")
	ErrNotConnected            = errors.New("google calendar is not connected")
	ErrReauthorizationRequired = errors.New("google calendar reauthorization required")
)

type Repository interface {
	SaveOAuthState(context.Context, domaincalendar.OAuthState) error
	ConsumeOAuthState(context.Context, string, time.Time) (domaincalendar.OAuthState, error)
	UpsertConnection(context.Context, domaincalendar.Connection) (domaincalendar.Connection, error)
	GetConnection(context.Context, uuid.UUID, uuid.UUID) (domaincalendar.Connection, error)
	MarkConnectionStatus(context.Context, uuid.UUID, uuid.UUID, string, bool, time.Time) error
	TouchSync(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	LinkedEventIDs(context.Context, uuid.UUID, uuid.UUID) (map[string]struct{}, error)
	GetLinkByAppointment(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domaincalendar.Link, error)
	CreateLink(context.Context, domaincalendar.Link) error
	UpdateLink(context.Context, domaincalendar.Link) error
}

type Provider interface {
	AuthorizationURL(state, challenge string) string
	Exchange(context.Context, string, string) (googleinfra.Token, error)
	Refresh(context.Context, string) (string, error)
	ListEvents(context.Context, string, string, time.Time, time.Time) ([]domaincalendar.Event, error)
	GetEvent(context.Context, string, string, string) (domaincalendar.Event, error)
	CreateEvent(context.Context, string, string, uuid.UUID, uuid.UUID, time.Time, time.Time, string) (domaincalendar.Event, error)
	UpdateEvent(context.Context, string, string, string, uuid.UUID, uuid.UUID, time.Time, time.Time, string) (domaincalendar.Event, error)
	DeleteEvent(context.Context, string, string, string) error
	Revoke(context.Context, string) error
}

type SecretBox interface {
	Seal(string) (string, error)
	Open(string) (string, error)
}
type AppointmentCreator interface {
	Create(context.Context, appointmentusecase.CreateInput) (domainappointment.Entity, error)
}
type AppointmentRepository interface {
	GetByID(context.Context, uuid.UUID, uuid.UUID) (domainappointment.Entity, error)
}
type ClinicalAccess interface {
	CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error)
}
type Auditor interface {
	RecordDomainEvent(context.Context, uuid.UUID, uuid.UUID, string, string, *uuid.UUID, map[string]any) error
}

type Service struct {
	enabled         bool
	repo            Repository
	provider        Provider
	box             SecretBox
	appointments    AppointmentCreator
	appointmentRepo AppointmentRepository
	access          ClinicalAccess
	auditor         Auditor
	now             func() time.Time
}

type Status struct {
	Enabled                 bool       `json:"enabled"`
	Connected               bool       `json:"connected"`
	ReauthorizationRequired bool       `json:"reauthorization_required"`
	CalendarID              string     `json:"calendar_id,omitempty"`
	LastSyncAt              *time.Time `json:"last_sync_at,omitempty"`
}
type Candidate struct {
	EventID  string    `json:"event_id"`
	Summary  string    `json:"summary"`
	Location string    `json:"location"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	HTMLLink string    `json:"html_link,omitempty"`
}

func NewService(enabled bool, repo Repository, provider Provider, box SecretBox, appointments AppointmentCreator, appointmentRepo AppointmentRepository, access ClinicalAccess, auditor Auditor) *Service {
	return &Service{enabled: enabled, repo: repo, provider: provider, box: box, appointments: appointments, appointmentRepo: appointmentRepo, access: access, auditor: auditor, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Status(ctx context.Context, tenantID, userID uuid.UUID) (Status, error) {
	if !s.enabled {
		return Status{Enabled: false}, nil
	}
	connection, err := s.repo.GetConnection(ctx, tenantID, userID)
	if errors.Is(err, domainerrors.ErrNotFound) {
		return Status{Enabled: true}, nil
	}
	if err != nil {
		return Status{}, err
	}
	return Status{Enabled: true, Connected: connection.Status == domaincalendar.StatusConnected, ReauthorizationRequired: connection.Status == domaincalendar.StatusReauthorizationRequired, CalendarID: connection.CalendarID, LastSyncAt: connection.LastSyncAt}, nil
}

func (s *Service) BeginAuthorization(ctx context.Context, tenantID, userID uuid.UUID) (string, error) {
	if !s.enabled {
		return "", ErrDisabled
	}
	state, err := randomValue(32)
	if err != nil {
		return "", err
	}
	verifier, err := randomValue(48)
	if err != nil {
		return "", err
	}
	encrypted, err := s.box.Seal(verifier)
	if err != nil {
		return "", err
	}
	if err := s.repo.SaveOAuthState(ctx, domaincalendar.OAuthState{StateHash: hashValue(state), TenantID: tenantID, UserID: userID, EncryptedCodeVerifier: encrypted, ExpiresAt: s.now().Add(10 * time.Minute)}); err != nil {
		return "", err
	}
	return s.provider.AuthorizationURL(state, googleinfra.CodeChallenge(verifier)), nil
}

func (s *Service) CompleteAuthorization(ctx context.Context, state, code string) (uuid.UUID, uuid.UUID, error) {
	if !s.enabled {
		return uuid.Nil, uuid.Nil, ErrDisabled
	}
	if strings.TrimSpace(state) == "" || strings.TrimSpace(code) == "" {
		return uuid.Nil, uuid.Nil, domainerrors.NewValidation("state and code are required")
	}
	oauthState, err := s.repo.ConsumeOAuthState(ctx, hashValue(state), s.now())
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid or expired OAuth state: %w", domainerrors.ErrValidation)
	}
	verifier, err := s.box.Open(oauthState.EncryptedCodeVerifier)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	token, err := s.provider.Exchange(ctx, code, verifier)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if token.RefreshToken == "" {
		return uuid.Nil, uuid.Nil, errors.New("Google did not return a refresh token; reconnect with consent")
	}
	encrypted, err := s.box.Seal(token.RefreshToken)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	now := s.now()
	connection, err := s.repo.UpsertConnection(ctx, domaincalendar.Connection{ID: uuid.New(), TenantID: oauthState.TenantID, UserID: oauthState.UserID, CalendarID: "primary", EncryptedRefreshToken: encrypted, Status: domaincalendar.StatusConnected, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	_ = s.audit(ctx, connection.TenantID, connection.UserID, "google_calendar.connect", "google_calendar_connection", connection.ID, map[string]any{"calendar_id": "primary"})
	return connection.TenantID, connection.UserID, nil
}

func (s *Service) ListCandidates(ctx context.Context, tenantID, userID uuid.UUID, from, to time.Time) ([]Candidate, error) {
	if !from.Before(to) || to.Sub(from) > 90*24*time.Hour {
		return nil, domainerrors.NewValidation("calendar range must be positive and at most 90 days")
	}
	connection, accessToken, err := s.authorizedConnection(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	events, err := s.provider.ListEvents(ctx, accessToken, connection.CalendarID, from.UTC(), to.UTC())
	if err != nil {
		return nil, s.handleProviderError(ctx, tenantID, userID, err)
	}
	linked, err := s.repo.LinkedEventIDs(ctx, tenantID, connection.ID)
	if err != nil {
		return nil, err
	}
	items := make([]Candidate, 0, len(events))
	for _, event := range events {
		if _, ok := linked[event.ID]; ok {
			continue
		}
		items = append(items, Candidate{EventID: event.ID, Summary: event.Summary, Location: event.Location, StartsAt: event.StartsAt, EndsAt: event.EndsAt, HTMLLink: event.HTMLLink})
	}
	_ = s.repo.TouchSync(ctx, tenantID, userID, s.now())
	_ = s.audit(ctx, tenantID, userID, "google_calendar.candidates.list", "google_calendar_connection", connection.ID, map[string]any{"count": len(items)})
	return items, nil
}

func (s *Service) ImportEvent(ctx context.Context, tenantID, userID, clientID uuid.UUID, eventID string) (domainappointment.Entity, error) {
	connection, accessToken, err := s.authorizedConnection(ctx, tenantID, userID)
	if err != nil {
		return domainappointment.Entity{}, err
	}
	event, err := s.provider.GetEvent(ctx, accessToken, connection.CalendarID, eventID)
	if err != nil {
		return domainappointment.Entity{}, s.handleProviderError(ctx, tenantID, userID, err)
	}
	appointment, err := s.appointments.Create(ctx, appointmentusecase.CreateInput{TenantID: tenantID, ClientID: clientID, ActorUserID: userID, StartsAt: event.StartsAt, EndsAt: event.EndsAt, Location: event.Location})
	if err != nil {
		return domainappointment.Entity{}, err
	}
	if err := s.repo.CreateLink(ctx, domaincalendar.Link{ID: uuid.New(), TenantID: tenantID, AppointmentID: appointment.ID, ConnectionID: connection.ID, GoogleEventID: event.ID, ETag: event.ETag, SyncStatus: "synced", GoogleUpdatedAt: event.UpdatedAt, LastSyncedAt: s.now()}); err != nil {
		return domainappointment.Entity{}, err
	}
	_ = s.audit(ctx, tenantID, userID, "google_calendar.event.import", "appointment", appointment.ID, map[string]any{"google_event_linked": true})
	return appointment, nil
}

func (s *Service) PushAppointment(ctx context.Context, tenantID, userID, appointmentID uuid.UUID) (domaincalendar.Link, error) {
	appointment, err := s.appointmentRepo.GetByID(ctx, tenantID, appointmentID)
	if err != nil {
		return domaincalendar.Link{}, err
	}
	if s.access != nil {
		allowed, accessErr := s.access.CanAccessClient(ctx, tenantID, userID, appointment.ClientID, "treating")
		if accessErr != nil {
			return domaincalendar.Link{}, accessErr
		}
		if !allowed {
			return domaincalendar.Link{}, domainerrors.ErrForbidden
		}
	}
	connection, accessToken, err := s.authorizedConnection(ctx, tenantID, userID)
	if err != nil {
		return domaincalendar.Link{}, err
	}
	link, linkErr := s.repo.GetLinkByAppointment(ctx, tenantID, connection.ID, appointmentID)
	now := s.now()
	if appointment.Status == domainappointment.StatusCanceled {
		if errors.Is(linkErr, domainerrors.ErrNotFound) {
			return domaincalendar.Link{}, domainerrors.NewValidation("canceled appointment is not linked")
		}
		if linkErr != nil {
			return domaincalendar.Link{}, linkErr
		}
		if err := s.provider.DeleteEvent(ctx, accessToken, connection.CalendarID, link.GoogleEventID); err != nil {
			return domaincalendar.Link{}, s.handleProviderError(ctx, tenantID, userID, err)
		}
		link.SyncStatus = "deleted"
		link.LastSyncedAt = now
		if err := s.repo.UpdateLink(ctx, link); err != nil {
			return domaincalendar.Link{}, err
		}
		return link, nil
	}
	var event domaincalendar.Event
	if errors.Is(linkErr, domainerrors.ErrNotFound) {
		event, err = s.provider.CreateEvent(ctx, accessToken, connection.CalendarID, appointment.ID, tenantID, appointment.StartsAt, appointment.EndsAt, appointment.Location)
		if err == nil {
			link = domaincalendar.Link{ID: uuid.New(), TenantID: tenantID, AppointmentID: appointment.ID, ConnectionID: connection.ID, GoogleEventID: event.ID}
		}
	} else if linkErr == nil {
		event, err = s.provider.UpdateEvent(ctx, accessToken, connection.CalendarID, link.GoogleEventID, appointment.ID, tenantID, appointment.StartsAt, appointment.EndsAt, appointment.Location)
	} else {
		return domaincalendar.Link{}, linkErr
	}
	if err != nil {
		return domaincalendar.Link{}, s.handleProviderError(ctx, tenantID, userID, err)
	}
	link.ETag = event.ETag
	link.GoogleUpdatedAt = event.UpdatedAt
	link.SyncStatus = "synced"
	link.LastSyncedAt = now
	if errors.Is(linkErr, domainerrors.ErrNotFound) {
		err = s.repo.CreateLink(ctx, link)
	} else {
		err = s.repo.UpdateLink(ctx, link)
	}
	if err != nil {
		return domaincalendar.Link{}, err
	}
	_ = s.repo.TouchSync(ctx, tenantID, userID, now)
	_ = s.audit(ctx, tenantID, userID, "google_calendar.appointment.push", "appointment", appointment.ID, map[string]any{"created": errors.Is(linkErr, domainerrors.ErrNotFound)})
	return link, nil
}

func (s *Service) Disconnect(ctx context.Context, tenantID, userID uuid.UUID) error {
	connection, err := s.repo.GetConnection(ctx, tenantID, userID)
	if errors.Is(err, domainerrors.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if connection.EncryptedRefreshToken != "" {
		if token, openErr := s.box.Open(connection.EncryptedRefreshToken); openErr == nil {
			_ = s.provider.Revoke(ctx, token)
		}
	}
	if err := s.repo.MarkConnectionStatus(ctx, tenantID, userID, domaincalendar.StatusDisconnected, true, s.now()); err != nil {
		return err
	}
	return s.audit(ctx, tenantID, userID, "google_calendar.disconnect", "google_calendar_connection", connection.ID, map[string]any{})
}

func (s *Service) authorizedConnection(ctx context.Context, tenantID, userID uuid.UUID) (domaincalendar.Connection, string, error) {
	if !s.enabled {
		return domaincalendar.Connection{}, "", ErrDisabled
	}
	connection, err := s.repo.GetConnection(ctx, tenantID, userID)
	if errors.Is(err, domainerrors.ErrNotFound) {
		return domaincalendar.Connection{}, "", ErrNotConnected
	}
	if err != nil {
		return domaincalendar.Connection{}, "", err
	}
	if connection.Status != domaincalendar.StatusConnected || connection.EncryptedRefreshToken == "" {
		return domaincalendar.Connection{}, "", ErrReauthorizationRequired
	}
	refreshToken, err := s.box.Open(connection.EncryptedRefreshToken)
	if err != nil {
		return domaincalendar.Connection{}, "", err
	}
	accessToken, err := s.provider.Refresh(ctx, refreshToken)
	if err != nil {
		return domaincalendar.Connection{}, "", s.handleProviderError(ctx, tenantID, userID, err)
	}
	return connection, accessToken, nil
}

func (s *Service) handleProviderError(ctx context.Context, tenantID, userID uuid.UUID, err error) error {
	if errors.Is(err, googleinfra.ErrUnauthorized) {
		_ = s.repo.MarkConnectionStatus(ctx, tenantID, userID, domaincalendar.StatusReauthorizationRequired, false, s.now())
		return ErrReauthorizationRequired
	}
	return err
}

func (s *Service) audit(ctx context.Context, tenantID, userID uuid.UUID, action, entity string, entityID uuid.UUID, metadata map[string]any) error {
	if s.auditor == nil {
		return nil
	}
	id := entityID
	return s.auditor.RecordDomainEvent(ctx, tenantID, userID, action, entity, &id, metadata)
}

func randomValue(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
