CREATE UNIQUE INDEX IF NOT EXISTS idx_users_tenant_id_id ON users (tenant_id, id);

ALTER TABLE user_roles
    DROP CONSTRAINT IF EXISTS user_roles_user_id_fkey;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_roles_tenant_user_fkey'
          AND conrelid = 'user_roles'::regclass
    ) THEN
        ALTER TABLE user_roles
            ADD CONSTRAINT user_roles_tenant_user_fkey
            FOREIGN KEY (tenant_id, user_id)
            REFERENCES users (tenant_id, id)
            ON DELETE CASCADE;
    END IF;
END $$;
