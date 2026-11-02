package googlecalendar

import "time"

type LinkedEvent struct {
	AppointmentID string    `json:"appointment_id"`
	ClientID      string    `json:"client_id"`
	ClientName    string    `json:"client_name"`
	EventID       string    `json:"event_id"`
	StartsAt      time.Time `json:"starts_at"`
	EndsAt        time.Time `json:"ends_at"`
	SyncStatus    string    `json:"sync_status"`
}
