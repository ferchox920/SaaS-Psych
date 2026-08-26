package googlecalendar

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusConnected               = "connected"
	StatusReauthorizationRequired = "reauthorization_required"
	StatusDisconnected            = "disconnected"
)

type Connection struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	UserID                uuid.UUID
	CalendarID            string
	EncryptedRefreshToken string
	Status                string
	LastSyncAt            *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type OAuthState struct {
	StateHash             string
	TenantID              uuid.UUID
	UserID                uuid.UUID
	EncryptedCodeVerifier string
	ExpiresAt             time.Time
}

type Event struct {
	ID        string
	Summary   string
	Location  string
	Status    string
	HTMLLink  string
	ETag      string
	StartsAt  time.Time
	EndsAt    time.Time
	UpdatedAt *time.Time
}

type Link struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	AppointmentID   uuid.UUID
	ConnectionID    uuid.UUID
	GoogleEventID   string
	ETag            string
	SyncStatus      string
	GoogleUpdatedAt *time.Time
	LastSyncedAt    time.Time
}
