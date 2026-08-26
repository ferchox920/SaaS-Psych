package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domaincalendar "sessionflow/apps/api/internal/domain/googlecalendar"
)

type GoogleCalendarRepository struct{ pool *pgxpool.Pool }

func NewGoogleCalendarRepository(pool *pgxpool.Pool) *GoogleCalendarRepository {
	return &GoogleCalendarRepository{pool: pool}
}

func (r *GoogleCalendarRepository) SaveOAuthState(ctx context.Context, state domaincalendar.OAuthState) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO google_calendar_oauth_states (state_hash, tenant_id, user_id, encrypted_code_verifier, expires_at)
		VALUES ($1,$2,$3,$4,$5)
	`, state.StateHash, state.TenantID, state.UserID, state.EncryptedCodeVerifier, state.ExpiresAt)
	if err != nil {
		return fmt.Errorf("save google oauth state: %w", err)
	}
	return nil
}

func (r *GoogleCalendarRepository) ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (domaincalendar.OAuthState, error) {
	var state domaincalendar.OAuthState
	err := r.pool.QueryRow(ctx, `
		UPDATE google_calendar_oauth_states
		SET consumed_at=$2
		WHERE state_hash=$1 AND consumed_at IS NULL AND expires_at>$2
		RETURNING state_hash, tenant_id, user_id, encrypted_code_verifier, expires_at
	`, stateHash, now).Scan(&state.StateHash, &state.TenantID, &state.UserID, &state.EncryptedCodeVerifier, &state.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincalendar.OAuthState{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return domaincalendar.OAuthState{}, fmt.Errorf("consume google oauth state: %w", err)
	}
	return state, nil
}

func (r *GoogleCalendarRepository) UpsertConnection(ctx context.Context, connection domaincalendar.Connection) (domaincalendar.Connection, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO google_calendar_connections
			(id, tenant_id, user_id, calendar_id, encrypted_refresh_token, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$7)
		ON CONFLICT (tenant_id,user_id) DO UPDATE SET
			calendar_id=EXCLUDED.calendar_id, encrypted_refresh_token=EXCLUDED.encrypted_refresh_token,
			status='connected', updated_at=EXCLUDED.updated_at
		RETURNING id,tenant_id,user_id,calendar_id,COALESCE(encrypted_refresh_token,''),status,last_sync_at,created_at,updated_at
	`, connection.ID, connection.TenantID, connection.UserID, connection.CalendarID,
		connection.EncryptedRefreshToken, domaincalendar.StatusConnected, connection.UpdatedAt,
	).Scan(&connection.ID, &connection.TenantID, &connection.UserID, &connection.CalendarID, &connection.EncryptedRefreshToken,
		&connection.Status, &connection.LastSyncAt, &connection.CreatedAt, &connection.UpdatedAt)
	if err != nil {
		return domaincalendar.Connection{}, fmt.Errorf("upsert google calendar connection: %w", err)
	}
	return connection, nil
}

func (r *GoogleCalendarRepository) GetConnection(ctx context.Context, tenantID, userID uuid.UUID) (domaincalendar.Connection, error) {
	var connection domaincalendar.Connection
	err := r.pool.QueryRow(ctx, `
		SELECT id,tenant_id,user_id,calendar_id,COALESCE(encrypted_refresh_token,''),status,last_sync_at,created_at,updated_at
		FROM google_calendar_connections WHERE tenant_id=$1 AND user_id=$2
	`, tenantID, userID).Scan(&connection.ID, &connection.TenantID, &connection.UserID, &connection.CalendarID,
		&connection.EncryptedRefreshToken, &connection.Status, &connection.LastSyncAt, &connection.CreatedAt, &connection.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincalendar.Connection{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return domaincalendar.Connection{}, fmt.Errorf("get google calendar connection: %w", err)
	}
	return connection, nil
}

func (r *GoogleCalendarRepository) MarkConnectionStatus(ctx context.Context, tenantID, userID uuid.UUID, status string, clearToken bool, now time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE google_calendar_connections
		SET status=$3, encrypted_refresh_token=CASE WHEN $4 THEN NULL ELSE encrypted_refresh_token END, updated_at=$5
		WHERE tenant_id=$1 AND user_id=$2
	`, tenantID, userID, status, clearToken, now)
	if err != nil {
		return fmt.Errorf("update google calendar connection: %w", err)
	}
	return nil
}

func (r *GoogleCalendarRepository) TouchSync(ctx context.Context, tenantID, userID uuid.UUID, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE google_calendar_connections SET last_sync_at=$3,updated_at=$3 WHERE tenant_id=$1 AND user_id=$2`, tenantID, userID, now)
	return err
}

func (r *GoogleCalendarRepository) LinkedEventIDs(ctx context.Context, tenantID, connectionID uuid.UUID) (map[string]struct{}, error) {
	rows, err := r.pool.Query(ctx, `SELECT google_event_id FROM appointment_google_calendar_links WHERE tenant_id=$1 AND connection_id=$2 AND sync_status<>'deleted'`, tenantID, connectionID)
	if err != nil {
		return nil, fmt.Errorf("list linked google event ids: %w", err)
	}
	defer rows.Close()
	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = struct{}{}
	}
	return ids, rows.Err()
}

func (r *GoogleCalendarRepository) GetLinkByAppointment(ctx context.Context, tenantID, connectionID, appointmentID uuid.UUID) (domaincalendar.Link, error) {
	var link domaincalendar.Link
	err := r.pool.QueryRow(ctx, `
		SELECT id,tenant_id,appointment_id,connection_id,google_event_id,etag,sync_status,google_updated_at,last_synced_at
		FROM appointment_google_calendar_links WHERE tenant_id=$1 AND connection_id=$2 AND appointment_id=$3
	`, tenantID, connectionID, appointmentID).Scan(&link.ID, &link.TenantID, &link.AppointmentID, &link.ConnectionID, &link.GoogleEventID, &link.ETag, &link.SyncStatus, &link.GoogleUpdatedAt, &link.LastSyncedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincalendar.Link{}, domainerrors.ErrNotFound
	}
	if err != nil {
		return domaincalendar.Link{}, fmt.Errorf("get appointment calendar link: %w", err)
	}
	return link, nil
}

func (r *GoogleCalendarRepository) CreateLink(ctx context.Context, link domaincalendar.Link) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO appointment_google_calendar_links
			(id,tenant_id,appointment_id,connection_id,google_event_id,etag,sync_status,google_updated_at,last_synced_at,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$9)
	`, link.ID, link.TenantID, link.AppointmentID, link.ConnectionID, link.GoogleEventID, link.ETag, link.SyncStatus, link.GoogleUpdatedAt, link.LastSyncedAt)
	if err != nil {
		return fmt.Errorf("create appointment calendar link: %w", err)
	}
	return nil
}

func (r *GoogleCalendarRepository) UpdateLink(ctx context.Context, link domaincalendar.Link) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE appointment_google_calendar_links SET etag=$4,sync_status=$5,google_updated_at=$6,last_synced_at=$7,updated_at=$7
		WHERE tenant_id=$1 AND connection_id=$2 AND appointment_id=$3
	`, link.TenantID, link.ConnectionID, link.AppointmentID, link.ETag, link.SyncStatus, link.GoogleUpdatedAt, link.LastSyncedAt)
	if err != nil {
		return fmt.Errorf("update appointment calendar link: %w", err)
	}
	return nil
}
