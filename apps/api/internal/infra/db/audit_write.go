package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"sessionflow/apps/api/internal/requestcontext"
)

type auditExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insertAuditEvent(
	ctx context.Context,
	executor auditExecutor,
	tenantID, actorUserID uuid.UUID,
	action, entity string,
	entityID uuid.UUID,
	metadata map[string]any,
) error {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if requestID, ok := requestcontext.RequestID(ctx); ok {
		metadata["request_id"] = requestID
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	const query = `
		INSERT INTO audit_logs (tenant_id, actor_user_id, action, entity, entity_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
	`
	if _, err := executor.Exec(ctx, query, tenantID, actorUserID, action, entity, entityID, metadataJSON); err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}
