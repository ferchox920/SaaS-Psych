-- Identity and revision ledger for manually reviewed external sources. This
-- migration does not fetch content or create clinical sessions.
CREATE TABLE clinical_manual_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('chatgpt_project')),
    project_id TEXT NOT NULL CHECK (project_id ~ '^g-p-[a-f0-9]{32}$'),
    source_id TEXT NOT NULL CHECK (length(source_id) BETWEEN 1 AND 512),
    source_created_at TIMESTAMPTZ,
    source_updated_at TIMESTAMPTZ,
    candidate_session_date DATE,
    session_date_basis TEXT CHECK (session_date_basis IN ('source_timestamp', 'explicit_text', 'professional_confirmed')),
    source_timezone TEXT,
    clinical_session_id UUID,
    created_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, client_id) REFERENCES clients (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, clinical_session_id, client_id)
        REFERENCES clinical_sessions (tenant_id, id, client_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, created_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    UNIQUE (tenant_id, source_kind, project_id, source_id),
    UNIQUE (tenant_id, id, client_id),
    CHECK ((candidate_session_date IS NULL AND session_date_basis IS NULL)
        OR (candidate_session_date IS NOT NULL AND session_date_basis IS NOT NULL)),
    CHECK (source_updated_at IS NULL OR source_created_at IS NULL OR source_updated_at >= source_created_at)
);

CREATE TABLE clinical_manual_source_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    source_id UUID NOT NULL,
    content_hash TEXT NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    hash_scheme TEXT NOT NULL CHECK (length(hash_scheme) BETWEEN 1 AND 100),
    review_status TEXT NOT NULL DEFAULT 'pending' CHECK (review_status IN ('pending', 'accepted', 'rejected')),
    reviewed_by_user_id UUID,
    reviewed_at TIMESTAMPTZ,
    created_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id, source_id, client_id)
        REFERENCES clinical_manual_sources (tenant_id, id, client_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, created_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, reviewed_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    UNIQUE (tenant_id, source_id, hash_scheme, content_hash),
    CHECK ((review_status = 'pending' AND reviewed_by_user_id IS NULL AND reviewed_at IS NULL)
        OR (review_status IN ('accepted', 'rejected') AND reviewed_by_user_id IS NOT NULL AND reviewed_at IS NOT NULL))
);

CREATE INDEX clinical_manual_sources_client ON clinical_manual_sources (tenant_id, client_id, created_at DESC);
CREATE INDEX clinical_manual_source_revisions_source ON clinical_manual_source_revisions (tenant_id, source_id, created_at DESC);
