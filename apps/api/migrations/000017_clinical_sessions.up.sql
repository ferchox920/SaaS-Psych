CREATE TABLE clinical_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    appointment_id UUID,
    therapist_user_id UUID NOT NULL,
    status TEXT NOT NULL DEFAULT 'in_progress'
        CHECK (status IN ('in_progress', 'completed', 'voided')),
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_sessions_lifecycle_check CHECK (
        (status = 'in_progress' AND ended_at IS NULL)
        OR (status IN ('completed', 'voided') AND ended_at IS NOT NULL AND ended_at >= started_at)
    ),
    CONSTRAINT clinical_sessions_tenant_client_fkey
        FOREIGN KEY (tenant_id, client_id) REFERENCES clients (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_sessions_tenant_appointment_fkey
        FOREIGN KEY (tenant_id, appointment_id) REFERENCES appointments (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_sessions_tenant_therapist_fkey
        FOREIGN KEY (tenant_id, therapist_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_sessions_tenant_id_unique UNIQUE (tenant_id, id)
);

CREATE UNIQUE INDEX clinical_sessions_one_nonvoid_per_appointment
    ON clinical_sessions (tenant_id, appointment_id)
    WHERE appointment_id IS NOT NULL AND status <> 'voided';

CREATE INDEX clinical_sessions_client_started
    ON clinical_sessions (tenant_id, client_id, started_at DESC, id DESC);

CREATE FUNCTION prevent_clinical_session_delete() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'clinical sessions cannot be physically deleted' USING ERRCODE = '23514';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER clinical_sessions_no_delete
    BEFORE DELETE ON clinical_sessions
    FOR EACH ROW EXECUTE FUNCTION prevent_clinical_session_delete();
