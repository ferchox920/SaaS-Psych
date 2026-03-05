CREATE UNIQUE INDEX IF NOT EXISTS idx_clients_tenant_id_id ON clients (tenant_id, id);

ALTER TABLE appointments
    DROP CONSTRAINT IF EXISTS appointments_client_id_fkey;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'appointments_tenant_client_fkey'
          AND conrelid = 'appointments'::regclass
    ) THEN
        ALTER TABLE appointments
            ADD CONSTRAINT appointments_tenant_client_fkey
            FOREIGN KEY (tenant_id, client_id)
            REFERENCES clients (tenant_id, id)
            ON DELETE CASCADE;
    END IF;
END $$;
