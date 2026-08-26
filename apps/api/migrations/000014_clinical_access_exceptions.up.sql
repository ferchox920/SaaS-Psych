CREATE TABLE clinical_access_exceptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    user_id UUID NOT NULL,
    granted_by_user_id UUID NOT NULL,
    reason TEXT NOT NULL CHECK (length(btrim(reason)) > 0),
    purpose TEXT NOT NULL CHECK (length(btrim(purpose)) > 0),
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoked_by_user_id UUID,
    revoke_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_access_exceptions_time_check
        CHECK (expires_at > starts_at AND expires_at <= starts_at + INTERVAL '24 hours'),
    CONSTRAINT clinical_access_exceptions_revoke_check
        CHECK (
            (revoked_at IS NULL AND revoked_by_user_id IS NULL AND revoke_reason = '')
            OR
            (revoked_at IS NOT NULL AND revoked_by_user_id IS NOT NULL AND length(btrim(revoke_reason)) > 0)
        ),
    CONSTRAINT clinical_access_exceptions_tenant_client_fkey
        FOREIGN KEY (tenant_id, client_id) REFERENCES clients (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_access_exceptions_tenant_user_fkey
        FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_access_exceptions_tenant_granted_by_fkey
        FOREIGN KEY (tenant_id, granted_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_access_exceptions_tenant_revoked_by_fkey
        FOREIGN KEY (tenant_id, revoked_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_clinical_access_exceptions_active_user
    ON clinical_access_exceptions (tenant_id, user_id, client_id, expires_at)
    WHERE revoked_at IS NULL;

CREATE INDEX idx_clinical_access_exceptions_client_history
    ON clinical_access_exceptions (tenant_id, client_id, created_at DESC);
