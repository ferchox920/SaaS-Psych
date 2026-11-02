-- Raw imported material is versioned separately from approved clinical memory.
ALTER TABLE clinical_manual_sources DROP CONSTRAINT clinical_manual_sources_source_kind_check;
ALTER TABLE clinical_manual_sources ADD CONSTRAINT clinical_manual_sources_source_kind_check
  CHECK (source_kind IN ('chatgpt_project','project_instructions','desktop_recording','phone_recording','local_transcript','comparison'));
ALTER TABLE clinical_manual_source_revisions ADD CONSTRAINT manual_revision_tenant_identity UNIQUE (tenant_id,id,client_id);
ALTER TABLE clinical_manual_source_revisions ADD CONSTRAINT manual_revision_source_identity UNIQUE (tenant_id,id,source_id,client_id);
CREATE TABLE clinical_manual_source_contents (
  tenant_id UUID NOT NULL,
  client_id UUID NOT NULL,
  revision_id UUID NOT NULL,
  payload JSONB NOT NULL CHECK (jsonb_typeof(payload)='object' AND octet_length(payload::text)<=1048576),
  PRIMARY KEY (tenant_id,revision_id),
  FOREIGN KEY (tenant_id,revision_id,client_id) REFERENCES clinical_manual_source_revisions(tenant_id,id,client_id) ON DELETE RESTRICT
);
ALTER TABLE clinical_manual_sources ADD COLUMN current_revision_id UUID;
ALTER TABLE clinical_manual_sources ADD CONSTRAINT manual_current_revision_tenant
  FOREIGN KEY (tenant_id,current_revision_id,id,client_id) REFERENCES clinical_manual_source_revisions(tenant_id,id,source_id,client_id) ON DELETE RESTRICT;
