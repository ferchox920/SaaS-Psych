DROP INDEX IF EXISTS clinical_ai_suggestions_tenant_run;
ALTER TABLE clinical_ai_suggestions DROP CONSTRAINT IF EXISTS clinical_ai_suggestions_tenant_run_fkey;
ALTER TABLE clinical_ai_suggestions DROP COLUMN IF EXISTS ai_run_id;
DROP TABLE IF EXISTS clinical_ai_run_sources;
DROP FUNCTION IF EXISTS validate_clinical_ai_run_source();
DROP TABLE IF EXISTS clinical_ai_runs;
DROP INDEX IF EXISTS idx_clinical_sessions_tenant_id_client_id;
DROP INDEX IF EXISTS idx_appointments_tenant_id_client_id;
