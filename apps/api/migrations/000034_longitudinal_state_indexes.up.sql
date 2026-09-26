CREATE INDEX clinical_events_approved_recent
ON clinical_events(tenant_id, client_id, observed_at DESC, id DESC)
WHERE approval_status = 'approved';

CREATE INDEX clinical_processes_approved_state
ON clinical_processes(
    tenant_id,
    client_id,
    (CASE clinical_status WHEN 'active' THEN 1 WHEN 'observing' THEN 2 WHEN 'stabilized' THEN 3 ELSE 4 END),
    updated_at DESC,
    id DESC
)
WHERE approval_status = 'approved';

CREATE INDEX clinical_hypotheses_approved_unassigned_state
ON clinical_hypotheses(tenant_id, client_id, updated_at DESC, id DESC)
WHERE approval_status = 'approved' AND process_id IS NULL;

CREATE INDEX clinical_diffs_open_state
ON clinical_diffs(tenant_id, client_id, created_at DESC, id DESC)
WHERE status IN ('draft', 'pending_review', 'partially_reviewed', 'approved');
