package googlecalendar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	domaincalendar "sessionflow/apps/api/internal/domain/googlecalendar"
)

const ownedEventsScope = "https://www.googleapis.com/auth/calendar.events.owned"

var ErrUnauthorized = errors.New("google calendar authorization is invalid")

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Timeout      time.Duration
}

type Provider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	client       *http.Client
}

type Token struct {
	AccessToken  string
	RefreshToken string
}

func NewProvider(cfg Config) (*Provider, error) {
	redirect, err := url.Parse(strings.TrimSpace(cfg.RedirectURL))
	if err != nil || redirect.Scheme == "" || redirect.Host == "" {
		return nil, errors.New("google calendar redirect URL must be absolute")
	}
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, errors.New("google calendar OAuth credentials are required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	return &Provider{
		clientID: strings.TrimSpace(cfg.ClientID), clientSecret: strings.TrimSpace(cfg.ClientSecret),
		redirectURL: redirect.String(), client: &http.Client{Timeout: cfg.Timeout},
	}, nil
}

func (p *Provider) AuthorizationURL(state, codeChallenge string) string {
	values := url.Values{
		"client_id": {p.clientID}, "redirect_uri": {p.redirectURL}, "response_type": {"code"},
		"scope": {ownedEventsScope}, "access_type": {"offline"}, "prompt": {"consent"},
		"include_granted_scopes": {"true"}, "state": {state},
		"code_challenge": {codeChallenge}, "code_challenge_method": {"S256"},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + values.Encode()
}

func CodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (p *Provider) Exchange(ctx context.Context, code, verifier string) (Token, error) {
	return p.tokenRequest(ctx, url.Values{
		"client_id": {p.clientID}, "client_secret": {p.clientSecret}, "code": {code},
		"code_verifier": {verifier}, "grant_type": {"authorization_code"}, "redirect_uri": {p.redirectURL},
	})
}

func (p *Provider) Refresh(ctx context.Context, refreshToken string) (string, error) {
	token, err := p.tokenRequest(ctx, url.Values{
		"client_id": {p.clientID}, "client_secret": {p.clientSecret},
		"refresh_token": {refreshToken}, "grant_type": {"refresh_token"},
	})
	return token.AccessToken, err
}

func (p *Provider) tokenRequest(ctx context.Context, values url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(values.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := p.client.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("google token request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		if res.StatusCode == http.StatusBadRequest || res.StatusCode == http.StatusUnauthorized {
			return Token{}, ErrUnauthorized
		}
		return Token{}, fmt.Errorf("google token request returned %d", res.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return Token{}, err
	}
	if payload.AccessToken == "" {
		return Token{}, errors.New("google token response did not include access token")
	}
	return Token{AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken}, nil
}

func (p *Provider) ListEvents(ctx context.Context, accessToken, calendarID string, from, to time.Time) ([]domaincalendar.Event, error) {
	items := make([]domaincalendar.Event, 0)
	pageToken := ""
	for {
		values := url.Values{
			"timeMin": {from.UTC().Format(time.RFC3339)}, "timeMax": {to.UTC().Format(time.RFC3339)},
			"singleEvents": {"true"}, "showDeleted": {"false"}, "orderBy": {"startTime"}, "maxResults": {"2500"},
		}
		if pageToken != "" {
			values.Set("pageToken", pageToken)
		}
		var payload eventsListResponse
		if err := p.doJSON(ctx, http.MethodGet, eventsURL(calendarID, "")+"?"+values.Encode(), accessToken, nil, &payload); err != nil {
			return nil, err
		}
		for _, raw := range payload.Items {
			if event, ok := raw.toDomain(); ok {
				items = append(items, event)
			}
		}
		if payload.NextPageToken == "" {
			break
		}
		pageToken = payload.NextPageToken
	}
	return items, nil
}

func (p *Provider) GetEvent(ctx context.Context, accessToken, calendarID, eventID string) (domaincalendar.Event, error) {
	var raw eventResource
	if err := p.doJSON(ctx, http.MethodGet, eventsURL(calendarID, eventID), accessToken, nil, &raw); err != nil {
		return domaincalendar.Event{}, err
	}
	event, ok := raw.toDomain()
	if !ok {
		return domaincalendar.Event{}, errors.New("google event is not a timed event")
	}
	return event, nil
}

func (p *Provider) CreateEvent(ctx context.Context, accessToken, calendarID string, appointmentID, tenantID uuid.UUID, startsAt, endsAt time.Time, location string) (domaincalendar.Event, error) {
	body := eventWriteResource(startsAt, endsAt, location, appointmentID, tenantID)
	var raw eventResource
	if err := p.doJSON(ctx, http.MethodPost, eventsURL(calendarID, ""), accessToken, body, &raw); err != nil {
		return domaincalendar.Event{}, err
	}
	event, ok := raw.toDomain()
	if !ok {
		return domaincalendar.Event{}, errors.New("google returned an invalid event")
	}
	return event, nil
}

func (p *Provider) UpdateEvent(ctx context.Context, accessToken, calendarID, eventID string, appointmentID, tenantID uuid.UUID, startsAt, endsAt time.Time, location string) (domaincalendar.Event, error) {
	body := eventWriteResource(startsAt, endsAt, location, appointmentID, tenantID)
	var raw eventResource
	if err := p.doJSON(ctx, http.MethodPatch, eventsURL(calendarID, eventID), accessToken, body, &raw); err != nil {
		return domaincalendar.Event{}, err
	}
	event, ok := raw.toDomain()
	if !ok {
		return domaincalendar.Event{}, errors.New("google returned an invalid event")
	}
	return event, nil
}

func (p *Provider) DeleteEvent(ctx context.Context, accessToken, calendarID, eventID string) error {
	return p.doJSON(ctx, http.MethodDelete, eventsURL(calendarID, eventID), accessToken, nil, nil)
}

func (p *Provider) Revoke(ctx context.Context, refreshToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/revoke?token="+url.QueryEscape(refreshToken), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("google revoke returned %d", res.StatusCode)
	}
	return nil
}

func (p *Provider) doJSON(ctx context.Context, method, endpoint, accessToken string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("google calendar request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return fmt.Errorf("google calendar request returned %d", res.StatusCode)
	}
	if out == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func eventsURL(calendarID, eventID string) string {
	base := "https://www.googleapis.com/calendar/v3/calendars/" + url.PathEscape(calendarID) + "/events"
	if eventID != "" {
		return base + "/" + url.PathEscape(eventID)
	}
	return base
}

type eventDateTime struct {
	DateTime string `json:"dateTime,omitempty"`
	Date     string `json:"date,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}
type eventResource struct {
	ID       string        `json:"id"`
	Summary  string        `json:"summary"`
	Location string        `json:"location"`
	Status   string        `json:"status"`
	HTMLLink string        `json:"htmlLink"`
	ETag     string        `json:"etag"`
	Updated  string        `json:"updated"`
	Start    eventDateTime `json:"start"`
	End      eventDateTime `json:"end"`
}
type eventsListResponse struct {
	Items         []eventResource `json:"items"`
	NextPageToken string          `json:"nextPageToken"`
}

func (e eventResource) toDomain() (domaincalendar.Event, bool) {
	if e.ID == "" || e.Start.DateTime == "" || e.End.DateTime == "" || e.Status == "cancelled" {
		return domaincalendar.Event{}, false
	}
	start, err := time.Parse(time.RFC3339, e.Start.DateTime)
	if err != nil {
		return domaincalendar.Event{}, false
	}
	end, err := time.Parse(time.RFC3339, e.End.DateTime)
	if err != nil || !start.Before(end) {
		return domaincalendar.Event{}, false
	}
	var updated *time.Time
	if parsed, err := time.Parse(time.RFC3339, e.Updated); err == nil {
		parsed = parsed.UTC()
		updated = &parsed
	}
	return domaincalendar.Event{ID: e.ID, Summary: e.Summary, Location: e.Location, Status: e.Status, HTMLLink: e.HTMLLink, ETag: e.ETag, StartsAt: start.UTC(), EndsAt: end.UTC(), UpdatedAt: updated}, true
}

func eventWriteResource(startsAt, endsAt time.Time, location string, appointmentID, tenantID uuid.UUID) map[string]any {
	return map[string]any{
		"summary": "Sesión", "description": "Turno administrado por SessionFlow. Sin información clínica.",
		"location": strings.TrimSpace(location), "visibility": "private",
		"start": map[string]string{"dateTime": startsAt.UTC().Format(time.RFC3339)},
		"end":   map[string]string{"dateTime": endsAt.UTC().Format(time.RFC3339)},
		"extendedProperties": map[string]any{"private": map[string]string{
			"sessionflow_appointment_id": appointmentID.String(), "sessionflow_tenant_id": tenantID.String(),
		}},
	}
}
