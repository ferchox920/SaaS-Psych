package db

import (
	"context"
	"github.com/google/uuid"
	domaincalendar "sessionflow/apps/api/internal/domain/googlecalendar"
	"time"
)

func (r *GoogleCalendarRepository) ListLinkedEvents(ctx context.Context, tenantID, userID uuid.UUID, from, to time.Time, limit, offset int) ([]domaincalendar.LinkedEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT a.id::text,c.id::text,c.fullname,l.google_event_id,a.starts_at,a.ends_at,l.sync_status
 FROM appointment_google_calendar_links l
 JOIN google_calendar_connections g ON g.tenant_id=l.tenant_id AND g.id=l.connection_id
 JOIN appointments a ON a.tenant_id=l.tenant_id AND a.id=l.appointment_id
 JOIN clients c ON c.tenant_id=a.tenant_id AND c.id=a.client_id
 WHERE l.tenant_id=$1 AND g.user_id=$2 AND a.starts_at >= $3 AND a.starts_at < $4
 AND (EXISTS (SELECT 1 FROM client_clinical_assignments cca WHERE cca.tenant_id=c.tenant_id AND cca.client_id=c.id AND cca.user_id=$2 AND cca.relationship IN ('treating','supervisor') AND cca.starts_at<=now() AND (cca.ends_at IS NULL OR cca.ends_at>now()))
 OR EXISTS (SELECT 1 FROM clinical_access_exceptions cae WHERE cae.tenant_id=c.tenant_id AND cae.client_id=c.id AND cae.user_id=$2 AND cae.starts_at<=now() AND cae.expires_at>now() AND cae.revoked_at IS NULL))
 ORDER BY a.starts_at,a.id LIMIT $5 OFFSET $6`, tenantID, userID, from, to, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domaincalendar.LinkedEvent, 0)
	for rows.Next() {
		var item domaincalendar.LinkedEvent
		if err := rows.Scan(&item.AppointmentID, &item.ClientID, &item.ClientName, &item.EventID, &item.StartsAt, &item.EndsAt, &item.SyncStatus); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
