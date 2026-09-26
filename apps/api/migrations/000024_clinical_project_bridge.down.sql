DROP INDEX IF EXISTS clinical_diffs_external_proposal_unique;
ALTER TABLE clinical_diffs DROP CONSTRAINT IF EXISTS clinical_diffs_single_proposal_source;
ALTER TABLE clinical_diffs DROP CONSTRAINT IF EXISTS clinical_diffs_external_proposal_fkey;
ALTER TABLE clinical_diffs DROP COLUMN IF EXISTS source_external_proposal_id;
DROP TABLE IF EXISTS clinical_external_proposals;
DROP TABLE IF EXISTS clinical_project_export_sources;
DROP TABLE IF EXISTS clinical_project_exports;
DROP FUNCTION IF EXISTS validate_clinical_project_source();
DROP FUNCTION IF EXISTS protect_clinical_project_provenance();
