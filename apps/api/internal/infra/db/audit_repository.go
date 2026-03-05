package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	auditusecase "sessionflow/apps/api/internal/usecase/audit"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) RecordAuthEvent(ctx context.Context, event authusecase.AuthAuditEvent) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal auth audit metadata: %w", err)
	}

	const query = `
		INSERT INTO audit_logs (tenant_id, actor_user_id, action, entity, metadata)
		VALUES ($1, $2, $3, $4, $5::jsonb)
	`

	var actorUserID any
	if event.ActorUserID == uuid.Nil {
		actorUserID = nil
	} else {
		actorUserID = event.ActorUserID
	}

	if _, err := r.pool.Exec(ctx, query, event.TenantID, actorUserID, event.Action, "auth", metadataJSON); err != nil {
		return fmt.Errorf("insert auth audit event: %w", err)
	}

	return nil
}

func (r *AuditRepository) RecordDomainEvent(
	ctx context.Context,
	tenantID uuid.UUID,
	actorUserID uuid.UUID,
	action string,
	entity string,
	entityID *uuid.UUID,
	metadata map[string]any,
) error {
	if metadata == nil {
		metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal domain audit metadata: %w", err)
	}

	const query = `
		INSERT INTO audit_logs (tenant_id, actor_user_id, action, entity, entity_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
	`

	var actor any
	if actorUserID == uuid.Nil {
		actor = nil
	} else {
		actor = actorUserID
	}

	var entityAny any
	if entityID == nil || *entityID == uuid.Nil {
		entityAny = nil
	} else {
		entityAny = *entityID
	}

	if _, err := r.pool.Exec(ctx, query, tenantID, actor, action, entity, entityAny, metadataJSON); err != nil {
		return fmt.Errorf("insert domain audit event: %w", err)
	}

	return nil
}

func (r *AuditRepository) ListAuditLogs(ctx context.Context, filter auditusecase.ListFilter) ([]auditusecase.LogEntry, error) {
	clauses := []string{"tenant_id = $1"}
	args := []any{filter.TenantID}
	argPos := 2

	if filter.ActorUserID != nil {
		clauses = append(clauses, fmt.Sprintf("actor_user_id = $%d", argPos))
		args = append(args, *filter.ActorUserID)
		argPos++
	}
	if filter.Action != "" {
		clauses = append(clauses, fmt.Sprintf("action = $%d", argPos))
		args = append(args, filter.Action)
		argPos++
	}
	if filter.ActionPrefix != "" {
		clauses = append(clauses, fmt.Sprintf("action LIKE $%d", argPos))
		args = append(args, filter.ActionPrefix+"%")
		argPos++
	}
	if filter.Entity != "" {
		clauses = append(clauses, fmt.Sprintf("entity = $%d", argPos))
		args = append(args, filter.Entity)
		argPos++
	}
	if filter.From != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", argPos))
		args = append(args, *filter.From)
		argPos++
	}
	if filter.To != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", argPos))
		args = append(args, *filter.To)
		argPos++
	}
	if filter.Cursor != nil {
		operator := "<"
		if filter.Order == "asc" {
			operator = ">"
		}
		cursorID := uuid.Nil
		if filter.CursorID != nil {
			cursorID = *filter.CursorID
		}
		clauses = append(clauses, fmt.Sprintf("(created_at, id) %s ($%d, $%d)", operator, argPos, argPos+1))
		args = append(args, *filter.Cursor, cursorID)
		argPos += 2
		if filter.CursorID != nil {
			clauses = append(clauses, fmt.Sprintf("id <> $%d", argPos))
			args = append(args, *filter.CursorID)
		}
	}

	order := "DESC"
	if filter.Order == "asc" {
		order = "ASC"
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, actor_user_id, action, entity, entity_id, metadata, created_at
		FROM audit_logs
		WHERE %s
		ORDER BY created_at %s, id %s
		LIMIT $%d OFFSET $%d
	`, strings.Join(clauses, " AND "), order, order, argPos, argPos+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	entries := make([]auditusecase.LogEntry, 0)
	for rows.Next() {
		var (
			entry       auditusecase.LogEntry
			actorUserID *uuid.UUID
			entityID    *uuid.UUID
			metadataRaw []byte
			createdAt   time.Time
		)
		if err := rows.Scan(
			&entry.ID,
			&entry.TenantID,
			&actorUserID,
			&entry.Action,
			&entry.Entity,
			&entityID,
			&metadataRaw,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit log row: %w", err)
		}

		metadata := make(map[string]any)
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
				return nil, fmt.Errorf("unmarshal audit metadata: %w", err)
			}
		}

		entry.ActorUserID = actorUserID
		entry.EntityID = entityID
		entry.Metadata = metadata
		entry.CreatedAt = createdAt
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit log rows: %w", err)
	}

	return entries, nil
}

func (r *AuditRepository) CountAuditLogs(ctx context.Context, filter auditusecase.ListFilter) (int, error) {
	clauses := []string{"tenant_id = $1"}
	args := []any{filter.TenantID}
	argPos := 2

	if filter.ActorUserID != nil {
		clauses = append(clauses, fmt.Sprintf("actor_user_id = $%d", argPos))
		args = append(args, *filter.ActorUserID)
		argPos++
	}
	if filter.Action != "" {
		clauses = append(clauses, fmt.Sprintf("action = $%d", argPos))
		args = append(args, filter.Action)
		argPos++
	}
	if filter.ActionPrefix != "" {
		clauses = append(clauses, fmt.Sprintf("action LIKE $%d", argPos))
		args = append(args, filter.ActionPrefix+"%")
		argPos++
	}
	if filter.Entity != "" {
		clauses = append(clauses, fmt.Sprintf("entity = $%d", argPos))
		args = append(args, filter.Entity)
		argPos++
	}
	if filter.From != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", argPos))
		args = append(args, *filter.From)
		argPos++
	}
	if filter.To != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", argPos))
		args = append(args, *filter.To)
		argPos++
	}
	if filter.Cursor != nil {
		operator := "<"
		if filter.Order == "asc" {
			operator = ">"
		}
		cursorID := uuid.Nil
		if filter.CursorID != nil {
			cursorID = *filter.CursorID
		}
		clauses = append(clauses, fmt.Sprintf("(created_at, id) %s ($%d, $%d)", operator, argPos, argPos+1))
		args = append(args, *filter.Cursor, cursorID)
		argPos += 2
		if filter.CursorID != nil {
			clauses = append(clauses, fmt.Sprintf("id <> $%d", argPos))
			args = append(args, *filter.CursorID)
		}
	}

	query := fmt.Sprintf(`
		SELECT COUNT(1)
		FROM audit_logs
		WHERE %s
	`, strings.Join(clauses, " AND "))

	var total int
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count audit logs: %w", err)
	}

	return total, nil
}
