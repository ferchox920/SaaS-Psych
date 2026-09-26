CREATE INDEX clinical_hypotheses_client_updated_page
ON clinical_hypotheses(tenant_id, client_id, updated_at DESC, id DESC);
