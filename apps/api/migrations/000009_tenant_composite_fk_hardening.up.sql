CREATE UNIQUE INDEX IF NOT EXISTS idx_users_tenant_id_id ON users (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_appointments_tenant_id_id ON appointments (tenant_id, id);

ALTER TABLE refresh_tokens
    DROP CONSTRAINT IF EXISTS refresh_tokens_user_id_fkey;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'refresh_tokens_tenant_user_fkey'
          AND conrelid = 'refresh_tokens'::regclass
    ) THEN
        ALTER TABLE refresh_tokens
            ADD CONSTRAINT refresh_tokens_tenant_user_fkey
            FOREIGN KEY (tenant_id, user_id)
            REFERENCES users (tenant_id, id)
            ON DELETE CASCADE;
    END IF;
END $$;

ALTER TABLE session_notes
    DROP CONSTRAINT IF EXISTS session_notes_appointment_id_fkey;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'session_notes_tenant_appointment_fkey'
          AND conrelid = 'session_notes'::regclass
    ) THEN
        ALTER TABLE session_notes
            ADD CONSTRAINT session_notes_tenant_appointment_fkey
            FOREIGN KEY (tenant_id, appointment_id)
            REFERENCES appointments (tenant_id, id)
            ON DELETE CASCADE;
    END IF;
END $$;
