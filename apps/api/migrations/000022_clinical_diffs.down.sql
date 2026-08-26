CREATE OR REPLACE FUNCTION validate_clinical_ai_run_source()
RETURNS trigger AS $$
BEGIN
    IF NEW.source_type = 'formulation_snapshot' AND NOT EXISTS (SELECT 1 FROM clinical_formulation_snapshots WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND status='approved' AND (NEW.source_version IS NULL OR version=NEW.source_version)) THEN RAISE EXCEPTION 'formulation snapshot source must be approved and tenant-local' USING ERRCODE='23503';
    ELSIF NEW.source_type = 'formulation_anchor' AND NOT EXISTS (SELECT 1 FROM clinical_formulation_anchors a JOIN clinical_formulation_snapshots s ON s.tenant_id=a.tenant_id AND s.id=a.snapshot_id WHERE a.tenant_id=NEW.tenant_id AND a.id=NEW.source_id AND s.status='approved' AND (NEW.source_version IS NULL OR s.version=NEW.source_version)) THEN RAISE EXCEPTION 'formulation anchor source must belong to an approved tenant-local snapshot' USING ERRCODE='23503';
    ELSIF NEW.source_type = 'session_report' AND NOT EXISTS (SELECT 1 FROM session_reports WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND status='approved' AND (NEW.source_version IS NULL OR version=NEW.source_version)) THEN RAISE EXCEPTION 'session report source must be approved and tenant-local' USING ERRCODE='23503'; END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
ALTER TABLE clinical_ai_run_sources DROP CONSTRAINT clinical_ai_run_sources_source_type_check;
ALTER TABLE clinical_ai_run_sources ADD CONSTRAINT clinical_ai_run_sources_source_type_check CHECK(source_type IN ('formulation_snapshot','formulation_anchor','session_report'));
DROP TABLE IF EXISTS clinical_longitudinal_transitions;
DROP TRIGGER IF EXISTS clinical_diff_operations_no_delete ON clinical_diff_operations;
DROP TABLE IF EXISTS clinical_diff_operations;
DROP TRIGGER IF EXISTS clinical_diffs_no_delete ON clinical_diffs;
DROP TABLE IF EXISTS clinical_diffs;
DROP TABLE IF EXISTS clinical_longitudinal_heads;
DROP INDEX IF EXISTS session_reports_tenant_id_session_unique;
