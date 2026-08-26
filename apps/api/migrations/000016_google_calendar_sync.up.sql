CREATE TABLE IF NOT EXISTS google_calendar_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    calendar_id TEXT NOT NULL DEFAULT 'primary',
    encrypted_refresh_token TEXT,
    status TEXT NOT NULL DEFAULT 'connected',
    last_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT google_calendar_connections_status_check
        CHECK (status IN ('connected', 'reauthorization_required', 'disconnected')),
    CONSTRAINT google_calendar_connections_tenant_user_unique UNIQUE (tenant_id, user_id),
    CONSTRAINT google_calendar_connections_tenant_id_id_unique UNIQUE (tenant_id, id),
    CONSTRAINT google_calendar_connections_tenant_user_fkey
        FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS google_calendar_oauth_states (
    state_hash TEXT PRIMARY KEY,
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    encrypted_code_verifier TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT google_calendar_oauth_states_tenant_user_fkey
        FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS appointment_google_calendar_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    appointment_id UUID NOT NULL,
    connection_id UUID NOT NULL,
    google_event_id TEXT NOT NULL,
    etag TEXT NOT NULL DEFAULT '',
    sync_status TEXT NOT NULL DEFAULT 'synced',
    google_updated_at TIMESTAMPTZ,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT appointment_google_calendar_links_status_check
        CHECK (sync_status IN ('synced', 'local_changed', 'remote_changed', 'conflict', 'deleted')),
    CONSTRAINT appointment_google_calendar_links_connection_appointment_unique UNIQUE (connection_id, appointment_id),
    CONSTRAINT appointment_google_calendar_links_connection_event_unique UNIQUE (connection_id, google_event_id),
    CONSTRAINT appointment_google_calendar_links_tenant_appointment_fkey
        FOREIGN KEY (tenant_id, appointment_id) REFERENCES appointments (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT appointment_google_calendar_links_tenant_connection_fkey
        FOREIGN KEY (tenant_id, connection_id) REFERENCES google_calendar_connections (tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_google_calendar_oauth_states_expiry
    ON google_calendar_oauth_states (expires_at) WHERE consumed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_appointment_google_calendar_links_connection
    ON appointment_google_calendar_links (tenant_id, connection_id, last_synced_at DESC);
