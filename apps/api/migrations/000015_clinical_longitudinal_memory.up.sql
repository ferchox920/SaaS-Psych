CREATE TABLE clinical_ai_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    appointment_id UUID NOT NULL,
    created_by_user_id UUID NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('live', 'review')),
    prompt_version TEXT NOT NULL CHECK (length(btrim(prompt_version)) > 0),
    result JSONB NOT NULL CHECK (jsonb_typeof(result) = 'object'),
    disposition TEXT NOT NULL DEFAULT 'pending'
        CHECK (disposition IN ('pending', 'accepted', 'corrected', 'discarded', 'postponed')),
    correction_text TEXT NOT NULL DEFAULT '',
    decision_reason TEXT NOT NULL DEFAULT '',
    decided_by_user_id UUID,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_ai_suggestions_decision_check CHECK (
        (disposition = 'pending' AND decided_by_user_id IS NULL AND decided_at IS NULL AND correction_text = '' AND decision_reason = '')
        OR
        (disposition <> 'pending' AND decided_by_user_id IS NOT NULL AND decided_at IS NOT NULL)
    ),
    CONSTRAINT clinical_ai_suggestions_correction_check CHECK (
        (disposition = 'corrected' AND length(btrim(correction_text)) > 0)
        OR
        (disposition <> 'corrected' AND correction_text = '')
    ),
    CONSTRAINT clinical_ai_suggestions_tenant_client_fkey
        FOREIGN KEY (tenant_id, client_id) REFERENCES clients (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_ai_suggestions_tenant_appointment_fkey
        FOREIGN KEY (tenant_id, appointment_id) REFERENCES appointments (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_ai_suggestions_tenant_creator_fkey
        FOREIGN KEY (tenant_id, created_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_ai_suggestions_tenant_decider_fkey
        FOREIGN KEY (tenant_id, decided_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX idx_clinical_ai_suggestions_tenant_id
    ON clinical_ai_suggestions (tenant_id, id);
CREATE INDEX idx_clinical_ai_suggestions_appointment
    ON clinical_ai_suggestions (tenant_id, appointment_id, created_at DESC);
CREATE INDEX idx_clinical_ai_suggestions_client_disposition
    ON clinical_ai_suggestions (tenant_id, client_id, disposition, created_at DESC);

CREATE TABLE clinical_formulation_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    approved_summary TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'approved', 'superseded')),
    created_by_user_id UUID NOT NULL,
    approved_by_user_id UUID,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_formulation_snapshots_approval_check CHECK (
        (status = 'draft' AND approved_by_user_id IS NULL AND approved_at IS NULL)
        OR
        (status IN ('approved', 'superseded') AND approved_by_user_id IS NOT NULL AND approved_at IS NOT NULL)
    ),
    CONSTRAINT clinical_formulation_snapshots_tenant_client_fkey
        FOREIGN KEY (tenant_id, client_id) REFERENCES clients (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_formulation_snapshots_tenant_creator_fkey
        FOREIGN KEY (tenant_id, created_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_formulation_snapshots_tenant_approver_fkey
        FOREIGN KEY (tenant_id, approved_by_user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_formulation_snapshots_client_version_unique
        UNIQUE (tenant_id, client_id, version),
    CONSTRAINT clinical_formulation_snapshots_tenant_id_unique
        UNIQUE (tenant_id, id)
);

CREATE UNIQUE INDEX idx_clinical_formulation_one_approved
    ON clinical_formulation_snapshots (tenant_id, client_id)
    WHERE status = 'approved';
CREATE INDEX idx_clinical_formulation_client_history
    ON clinical_formulation_snapshots (tenant_id, client_id, version DESC);

CREATE TABLE clinical_formulation_anchors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    snapshot_id UUID NOT NULL,
    source_id TEXT NOT NULL CHECK (length(btrim(source_id)) > 0),
    kind TEXT NOT NULL CHECK (kind IN ('fact', 'hypothesis')),
    summary TEXT NOT NULL CHECK (length(btrim(summary)) > 0),
    traffic_light TEXT,
    source_suggestion_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_formulation_anchors_traffic_check CHECK (
        (kind = 'fact' AND traffic_light IS NULL)
        OR
        (kind = 'hypothesis' AND traffic_light IN ('green', 'yellow', 'red'))
    ),
    CONSTRAINT clinical_formulation_anchors_tenant_snapshot_fkey
        FOREIGN KEY (tenant_id, snapshot_id) REFERENCES clinical_formulation_snapshots (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_formulation_anchors_tenant_suggestion_fkey
        FOREIGN KEY (tenant_id, source_suggestion_id) REFERENCES clinical_ai_suggestions (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT clinical_formulation_anchors_source_unique
        UNIQUE (tenant_id, snapshot_id, source_id)
);

CREATE INDEX idx_clinical_formulation_anchors_snapshot
    ON clinical_formulation_anchors (tenant_id, snapshot_id, created_at, id);
