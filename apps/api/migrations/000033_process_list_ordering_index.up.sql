CREATE INDEX clinical_processes_client_status_page
ON clinical_processes (
    tenant_id,
    client_id,
    (CASE clinical_status WHEN 'active' THEN 1 WHEN 'observing' THEN 2 WHEN 'stabilized' THEN 3 ELSE 4 END),
    updated_at DESC,
    id DESC
);
