CREATE UNIQUE INDEX IF NOT EXISTS idx_users_tenant_id_id ON users (tenant_id, id);

-- Preserve existing audit history by nullifying actor references that do not match tenant.
UPDATE audit_logs al
SET actor_user_id = NULL
WHERE actor_user_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM users u
    WHERE u.id = al.actor_user_id
      AND u.tenant_id = al.tenant_id
  );

ALTER TABLE audit_logs
    DROP CONSTRAINT IF EXISTS audit_logs_actor_user_id_fkey;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'audit_logs_tenant_actor_user_fkey'
          AND conrelid = 'audit_logs'::regclass
    ) THEN
        ALTER TABLE audit_logs
            ADD CONSTRAINT audit_logs_tenant_actor_user_fkey
            FOREIGN KEY (tenant_id, actor_user_id)
            REFERENCES users (tenant_id, id)
            ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_actor_user_id
    ON audit_logs (tenant_id, actor_user_id)
    WHERE actor_user_id IS NOT NULL;
