ALTER TABLE appointments
    DROP CONSTRAINT IF EXISTS appointments_tenant_client_fkey;

ALTER TABLE appointments
    ADD CONSTRAINT appointments_client_id_fkey
    FOREIGN KEY (client_id)
    REFERENCES clients (id)
    ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_clients_tenant_id_id;
