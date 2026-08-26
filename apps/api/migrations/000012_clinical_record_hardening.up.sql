CREATE UNIQUE INDEX IF NOT EXISTS idx_session_notes_tenant_id_id
    ON session_notes (tenant_id, id);

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS archived_by_user_id UUID,
    ADD COLUMN IF NOT EXISTS archive_reason TEXT NOT NULL DEFAULT '';

ALTER TABLE clients
    DROP CONSTRAINT IF EXISTS clients_tenant_archived_by_fkey;

ALTER TABLE clients
    ADD CONSTRAINT clients_tenant_archived_by_fkey
    FOREIGN KEY (tenant_id, archived_by_user_id)
    REFERENCES users (tenant_id, id)
    ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_clients_tenant_active_created
    ON clients (tenant_id, created_at DESC, id DESC)
    WHERE archived_at IS NULL;

ALTER TABLE appointments
    DROP CONSTRAINT IF EXISTS appointments_tenant_client_fkey;

ALTER TABLE appointments
    ADD CONSTRAINT appointments_tenant_client_fkey
    FOREIGN KEY (tenant_id, client_id)
    REFERENCES clients (tenant_id, id)
    ON DELETE RESTRICT;

ALTER TABLE session_notes
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS current_version INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS signed_at TIMESTAMPTZ;

ALTER TABLE session_notes
    DROP CONSTRAINT IF EXISTS session_notes_status_check;

ALTER TABLE session_notes
    ADD CONSTRAINT session_notes_status_check
    CHECK (status IN ('draft', 'signed'));

ALTER TABLE session_notes
    DROP CONSTRAINT IF EXISTS session_notes_tenant_appointment_fkey;

ALTER TABLE session_notes
    ADD CONSTRAINT session_notes_tenant_appointment_fkey
    FOREIGN KEY (tenant_id, appointment_id)
    REFERENCES appointments (tenant_id, id)
    ON DELETE RESTRICT;

ALTER TABLE session_notes
    DROP CONSTRAINT IF EXISTS session_notes_author_user_id_fkey;

ALTER TABLE session_notes
    DROP CONSTRAINT IF EXISTS session_notes_tenant_author_user_fkey;

ALTER TABLE session_notes
    ADD CONSTRAINT session_notes_tenant_author_user_fkey
    FOREIGN KEY (tenant_id, author_user_id)
    REFERENCES users (tenant_id, id)
    ON DELETE RESTRICT;

CREATE TABLE IF NOT EXISTS session_note_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    note_id UUID NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    body TEXT NOT NULL,
    is_private BOOLEAN NOT NULL DEFAULT TRUE,
    change_kind TEXT NOT NULL CHECK (change_kind IN ('draft', 'correction', 'addendum')),
    change_reason TEXT NOT NULL DEFAULT '',
    actor_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT session_note_versions_tenant_note_fkey
        FOREIGN KEY (tenant_id, note_id)
        REFERENCES session_notes (tenant_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT session_note_versions_tenant_actor_fkey
        FOREIGN KEY (tenant_id, actor_user_id)
        REFERENCES users (tenant_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT session_note_versions_tenant_note_version_unique
        UNIQUE (tenant_id, note_id, version)
);

INSERT INTO session_note_versions (
    tenant_id,
    note_id,
    version,
    body,
    is_private,
    change_kind,
    change_reason,
    actor_user_id,
    created_at
)
SELECT
    sn.tenant_id,
    sn.id,
    1,
    sn.body,
    sn.is_private,
    'draft',
    'Migrated from legacy session note',
    sn.author_user_id,
    sn.created_at
FROM session_notes sn
ON CONFLICT (tenant_id, note_id, version) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_session_note_versions_tenant_note_created
    ON session_note_versions (tenant_id, note_id, version DESC);

