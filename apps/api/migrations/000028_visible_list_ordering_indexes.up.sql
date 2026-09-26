CREATE INDEX clients_active_tenant_created
    ON clients (tenant_id, created_at DESC, id DESC)
    WHERE archived_at IS NULL;

CREATE INDEX clients_archived_tenant_created
    ON clients (tenant_id, created_at DESC, id DESC)
    WHERE archived_at IS NOT NULL;

CREATE INDEX appointments_tenant_starts_id
    ON appointments (tenant_id, starts_at ASC, id ASC);
