CREATE TABLE session_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    clinical_session_id UUID NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    schema_version TEXT NOT NULL CHECK (schema_version IN ('session-report-v1', 'session-report-v1.1')),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'approved', 'superseded')),
    report_json JSONB NOT NULL CHECK (jsonb_typeof(report_json) = 'object'),
    created_by_user_id UUID NOT NULL,
    source_ai_run_id UUID,
    approved_by_user_id UUID,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT session_reports_approval_check CHECK (
        (status = 'draft' AND approved_by_user_id IS NULL AND approved_at IS NULL)
        OR (status IN ('approved', 'superseded') AND approved_by_user_id IS NOT NULL AND approved_at IS NOT NULL)
    ),
    CONSTRAINT session_reports_tenant_session_fkey
        FOREIGN KEY (tenant_id, clinical_session_id) REFERENCES clinical_sessions (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT session_reports_tenant_creator_fkey
        FOREIGN KEY (tenant_id, created_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT session_reports_tenant_run_fkey
        FOREIGN KEY (tenant_id, source_ai_run_id, clinical_session_id)
        REFERENCES clinical_ai_runs (tenant_id, id, clinical_session_id) ON DELETE RESTRICT,
    CONSTRAINT session_reports_tenant_approver_fkey
        FOREIGN KEY (tenant_id, approved_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT session_reports_session_version_unique UNIQUE (tenant_id, clinical_session_id, version),
    CONSTRAINT session_reports_tenant_id_unique UNIQUE (tenant_id, id)
);

CREATE UNIQUE INDEX session_reports_one_approved_per_session
    ON session_reports (tenant_id, clinical_session_id) WHERE status = 'approved';
CREATE INDEX session_reports_session_history
    ON session_reports (tenant_id, clinical_session_id, version DESC);
