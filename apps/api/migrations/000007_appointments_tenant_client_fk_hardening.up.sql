CREATE UNIQUE INDEX IF NOT EXISTS idx_clients_tenant_id_id ON clients (tenant_id, id);

ALTER TABLE appointments
    DROP CONSTRAINT IF EXISTS appointments_client_id_fkey;

ALTER TABLE appointments
    ADD CONSTRAINT appointments_tenant_client_fkey
    FOREIGN KEY (tenant_id, client_id)
    REFERENCES clients (tenant_id, id)
    ON DELETE CASCADE;
