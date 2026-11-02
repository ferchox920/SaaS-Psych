DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM clinical_manual_source_contents) THEN
    RAISE EXCEPTION 'Refusing to discard imported source content; preserve or restore its backup before schema rollback';
  END IF;
END $$;
ALTER TABLE clinical_manual_sources DROP CONSTRAINT manual_current_revision_tenant;
ALTER TABLE clinical_manual_sources DROP COLUMN current_revision_id;
DROP TABLE clinical_manual_source_contents;
ALTER TABLE clinical_manual_source_revisions DROP CONSTRAINT manual_revision_tenant_identity;
ALTER TABLE clinical_manual_source_revisions DROP CONSTRAINT manual_revision_source_identity;
-- Refuse rollback if newer source kinds remain; never silently delete history.
ALTER TABLE clinical_manual_sources DROP CONSTRAINT clinical_manual_sources_source_kind_check;
ALTER TABLE clinical_manual_sources ADD CONSTRAINT clinical_manual_sources_source_kind_check CHECK (source_kind='chatgpt_project');
