CREATE TABLE client_profiles (
 tenant_id uuid NOT NULL,
 client_id uuid NOT NULL,
 profile jsonb NOT NULL,
 revision integer NOT NULL CHECK (revision > 0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (tenant_id, client_id),
 FOREIGN KEY (tenant_id, client_id) REFERENCES clients(tenant_id, id) ON DELETE RESTRICT
);
