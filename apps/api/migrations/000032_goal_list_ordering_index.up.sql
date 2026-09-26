CREATE INDEX clinical_goals_client_priority_page
ON clinical_goals (
    tenant_id,
    client_id,
    process_id,
    (CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END),
    created_at,
    id
);
