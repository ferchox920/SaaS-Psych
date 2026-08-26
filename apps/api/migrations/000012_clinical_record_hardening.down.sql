DROP TABLE IF EXISTS session_note_versions;

ALTER TABLE session_notes
    DROP CONSTRAINT IF EXISTS session_notes_tenant_author_user_fkey;

ALTER TABLE session_notes
    ADD CONSTRAINT session_notes_author_user_id_fkey
    FOREIGN KEY (author_user_id)
    REFERENCES users (id)
    ON DELETE CASCADE;

ALTER TABLE session_notes
    DROP CONSTRAINT IF EXISTS session_notes_tenant_appointment_fkey;

ALTER TABLE session_notes
    ADD CONSTRAINT session_notes_tenant_appointment_fkey
    FOREIGN KEY (tenant_id, appointment_id)
    REFERENCES appointments (tenant_id, id)
    ON DELETE CASCADE;

ALTER TABLE session_notes
    DROP CONSTRAINT IF EXISTS session_notes_status_check,
    DROP COLUMN IF EXISTS signed_at,
    DROP COLUMN IF EXISTS current_version,
    DROP COLUMN IF EXISTS status;

ALTER TABLE appointments
    DROP CONSTRAINT IF EXISTS appointments_tenant_client_fkey;

ALTER TABLE appointments
    ADD CONSTRAINT appointments_tenant_client_fkey
    FOREIGN KEY (tenant_id, client_id)
    REFERENCES clients (tenant_id, id)
    ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_clients_tenant_active_created;

ALTER TABLE clients
    DROP CONSTRAINT IF EXISTS clients_tenant_archived_by_fkey,
    DROP COLUMN IF EXISTS archive_reason,
    DROP COLUMN IF EXISTS archived_by_user_id,
    DROP COLUMN IF EXISTS archived_at;

