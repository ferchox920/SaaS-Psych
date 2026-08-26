CREATE UNIQUE INDEX IF NOT EXISTS idx_appointments_tenant_id_client_id
    ON appointments (tenant_id, id, client_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_clinical_sessions_tenant_id_client_id
    ON clinical_sessions (tenant_id, id, client_id);

CREATE TABLE clinical_ai_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    appointment_id UUID,
    clinical_session_id UUID,
    created_by_user_id UUID NOT NULL,
    provider TEXT NOT NULL CHECK (length(btrim(provider)) > 0),
    model TEXT NOT NULL CHECK (length(btrim(model)) > 0),
    operation TEXT NOT NULL CHECK (length(btrim(operation)) > 0),
    prompt_name TEXT NOT NULL CHECK (length(btrim(prompt_name)) > 0),
    prompt_version TEXT NOT NULL CHECK (length(btrim(prompt_version)) > 0),
    app_version TEXT NOT NULL DEFAULT 'development' CHECK (length(btrim(app_version)) > 0),
    build_revision TEXT NOT NULL DEFAULT 'unknown' CHECK (length(btrim(build_revision)) > 0),
    parameters_json JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(parameters_json) = 'object'),
    input_hash TEXT NOT NULL CHECK (length(input_hash) = 64),
    context_hash TEXT NOT NULL CHECK (length(context_hash) = 64),
    output_hash TEXT CHECK (output_hash IS NULL OR length(output_hash) = 64),
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed', 'cancelled')),
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_ai_runs_lifecycle_check CHECK (
        (status = 'running' AND completed_at IS NULL AND output_hash IS NULL AND error_code IS NULL)
        OR (status = 'succeeded' AND completed_at IS NOT NULL AND output_hash IS NOT NULL AND error_code IS NULL)
        OR (status IN ('failed', 'cancelled') AND completed_at IS NOT NULL AND output_hash IS NULL AND length(btrim(error_code)) > 0)
    ),
    CONSTRAINT clinical_ai_runs_tenant_client_fkey
        FOREIGN KEY (tenant_id, client_id) REFERENCES clients (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_ai_runs_tenant_appointment_client_fkey
        FOREIGN KEY (tenant_id, appointment_id, client_id) REFERENCES appointments (tenant_id, id, client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_ai_runs_tenant_session_client_fkey
        FOREIGN KEY (tenant_id, clinical_session_id, client_id) REFERENCES clinical_sessions (tenant_id, id, client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_ai_runs_tenant_creator_fkey
        FOREIGN KEY (tenant_id, created_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_ai_runs_tenant_id_unique UNIQUE (tenant_id, id)
);

CREATE UNIQUE INDEX clinical_ai_runs_suggestion_provenance_unique
    ON clinical_ai_runs (tenant_id, id, client_id, appointment_id);
CREATE UNIQUE INDEX clinical_ai_runs_report_provenance_unique
    ON clinical_ai_runs (tenant_id, id, clinical_session_id);

CREATE INDEX clinical_ai_runs_client_started
    ON clinical_ai_runs (tenant_id, client_id, started_at DESC, id DESC);
CREATE INDEX clinical_ai_runs_session_started
    ON clinical_ai_runs (tenant_id, clinical_session_id, started_at DESC)
    WHERE clinical_session_id IS NOT NULL;

CREATE TABLE clinical_ai_run_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    ai_run_id UUID NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('formulation_snapshot', 'formulation_anchor', 'session_report')),
    source_id UUID NOT NULL,
    source_version INTEGER CHECK (source_version IS NULL OR source_version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_ai_run_sources_tenant_run_fkey
        FOREIGN KEY (tenant_id, ai_run_id) REFERENCES clinical_ai_runs (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT clinical_ai_run_sources_unique
        UNIQUE (tenant_id, ai_run_id, source_type, source_id, source_version)
);
CREATE INDEX clinical_ai_run_sources_run_order
    ON clinical_ai_run_sources (tenant_id, ai_run_id, source_type, source_id, source_version);

CREATE OR REPLACE FUNCTION validate_clinical_ai_run_source()
RETURNS trigger AS $$
BEGIN
    IF NEW.source_type = 'formulation_snapshot' AND NOT EXISTS (
        SELECT 1 FROM clinical_formulation_snapshots
        WHERE tenant_id = NEW.tenant_id AND id = NEW.source_id AND status = 'approved'
          AND (NEW.source_version IS NULL OR version = NEW.source_version)
    ) THEN
        RAISE EXCEPTION 'formulation snapshot source must be approved and tenant-local' USING ERRCODE = '23503';
    ELSIF NEW.source_type = 'formulation_anchor' AND NOT EXISTS (
        SELECT 1 FROM clinical_formulation_anchors a
        JOIN clinical_formulation_snapshots s ON s.tenant_id = a.tenant_id AND s.id = a.snapshot_id
        WHERE a.tenant_id = NEW.tenant_id AND a.id = NEW.source_id AND s.status = 'approved'
          AND (NEW.source_version IS NULL OR s.version = NEW.source_version)
    ) THEN
        RAISE EXCEPTION 'formulation anchor source must belong to an approved tenant-local snapshot' USING ERRCODE = '23503';
    ELSIF NEW.source_type = 'session_report' AND NOT EXISTS (
        SELECT 1 FROM session_reports
        WHERE tenant_id = NEW.tenant_id AND id = NEW.source_id AND status = 'approved'
          AND (NEW.source_version IS NULL OR version = NEW.source_version)
    ) THEN
        RAISE EXCEPTION 'session report source must be approved and tenant-local' USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER clinical_ai_run_sources_validate
BEFORE INSERT ON clinical_ai_run_sources
FOR EACH ROW EXECUTE FUNCTION validate_clinical_ai_run_source();

ALTER TABLE clinical_ai_suggestions ADD COLUMN ai_run_id UUID;
ALTER TABLE clinical_ai_suggestions ADD CONSTRAINT clinical_ai_suggestions_tenant_run_fkey
    FOREIGN KEY (tenant_id, ai_run_id, client_id, appointment_id)
    REFERENCES clinical_ai_runs (tenant_id, id, client_id, appointment_id) ON DELETE RESTRICT;
CREATE INDEX clinical_ai_suggestions_tenant_run
    ON clinical_ai_suggestions (tenant_id, ai_run_id) WHERE ai_run_id IS NOT NULL;
