CREATE TABLE client_clinical_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    user_id UUID NOT NULL,
    relationship TEXT NOT NULL CHECK (relationship IN ('treating', 'supervisor')),
    granted_by_user_id UUID NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at TIMESTAMPTZ,
    ended_by_user_id UUID,
    end_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT client_clinical_assignments_time_check
        CHECK (ends_at IS NULL OR ends_at > starts_at),
    CONSTRAINT client_clinical_assignments_tenant_client_fkey
        FOREIGN KEY (tenant_id, client_id)
        REFERENCES clients (tenant_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT client_clinical_assignments_tenant_user_fkey
        FOREIGN KEY (tenant_id, user_id)
        REFERENCES users (tenant_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT client_clinical_assignments_tenant_granted_by_fkey
        FOREIGN KEY (tenant_id, granted_by_user_id)
        REFERENCES users (tenant_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT client_clinical_assignments_tenant_ended_by_fkey
        FOREIGN KEY (tenant_id, ended_by_user_id)
        REFERENCES users (tenant_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT client_clinical_assignments_end_metadata_check
        CHECK (
            (ends_at IS NULL AND ended_by_user_id IS NULL AND end_reason = '')
            OR
            (ends_at IS NOT NULL AND ended_by_user_id IS NOT NULL AND length(btrim(end_reason)) > 0)
        )
);

CREATE UNIQUE INDEX client_clinical_assignments_active_unique
    ON client_clinical_assignments (tenant_id, client_id, user_id, relationship)
    WHERE ends_at IS NULL;

CREATE INDEX idx_client_clinical_assignments_active_user
    ON client_clinical_assignments (tenant_id, user_id, client_id)
    WHERE ends_at IS NULL;

-- Preserve legitimate access for authors of existing clinical notes without
-- granting blanket access to tenant owners or administrators.
INSERT INTO client_clinical_assignments (
    tenant_id,
    client_id,
    user_id,
    relationship,
    granted_by_user_id,
    starts_at
)
SELECT DISTINCT
    sn.tenant_id,
    a.client_id,
    sn.author_user_id,
    'treating',
    sn.author_user_id,
    LEAST(sn.created_at, NOW())
FROM session_notes sn
JOIN appointments a
  ON a.tenant_id = sn.tenant_id
 AND a.id = sn.appointment_id
ON CONFLICT (tenant_id, client_id, user_id, relationship)
    WHERE ends_at IS NULL
DO NOTHING;
