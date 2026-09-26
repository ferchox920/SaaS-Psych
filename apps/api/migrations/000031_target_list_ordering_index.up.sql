CREATE INDEX clinical_targets_client_status_page
ON clinical_targets (
    tenant_id,
    client_id,
    process_id,
    (CASE clinical_status WHEN 'active' THEN 1 WHEN 'monitoring' THEN 2 WHEN 'resolved' THEN 3 ELSE 4 END),
    updated_at DESC,
    id
);
