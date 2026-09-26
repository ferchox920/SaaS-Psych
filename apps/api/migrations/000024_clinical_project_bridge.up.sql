CREATE TABLE clinical_project_exports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    generated_by_user_id UUID NOT NULL,
    state_version BIGINT NOT NULL CHECK(state_version >= 0),
    schema_version TEXT NOT NULL CHECK(length(btrim(schema_version)) > 0),
    privacy_mode TEXT NOT NULL DEFAULT 'minimized' CHECK(privacy_mode IN ('minimized')),
    artifact_json JSONB NOT NULL CHECK(jsonb_typeof(artifact_json) = 'object'),
    local_snapshot_json JSONB NOT NULL CHECK(jsonb_typeof(local_snapshot_json) = 'object'),
    artifact_markdown TEXT NOT NULL,
    content_hash TEXT NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    marked_used_by_user_id UUID,
    marked_used_at TIMESTAMPTZ,
    CONSTRAINT clinical_project_exports_client_fkey FOREIGN KEY(tenant_id,client_id) REFERENCES clients(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_project_exports_generator_fkey FOREIGN KEY(tenant_id,generated_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_project_exports_used_by_fkey FOREIGN KEY(tenant_id,marked_used_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_project_exports_tenant_id_unique UNIQUE(tenant_id,id),
    CONSTRAINT clinical_project_exports_tenant_client_unique UNIQUE(tenant_id,id,client_id)
);
CREATE INDEX clinical_project_exports_client_created ON clinical_project_exports(tenant_id,client_id,generated_at DESC,id DESC);
CREATE TRIGGER clinical_project_exports_no_delete BEFORE DELETE ON clinical_project_exports FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE clinical_project_export_sources (
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    export_id UUID NOT NULL,
    export_ref TEXT NOT NULL CHECK(length(btrim(export_ref)) > 0),
    entity_type TEXT NOT NULL CHECK(entity_type IN ('process','evidence','event','hypothesis','target','goal','indicator','rationale','gira','session_report','formulation')),
    entity_id UUID NOT NULL,
    entity_version INTEGER NOT NULL CHECK(entity_version > 0),
    PRIMARY KEY(tenant_id,export_id,export_ref),
    CONSTRAINT clinical_project_export_sources_export_fkey FOREIGN KEY(tenant_id,export_id,client_id) REFERENCES clinical_project_exports(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_project_export_sources_entity_unique UNIQUE(tenant_id,export_id,entity_type,entity_id)
);
CREATE INDEX clinical_project_export_sources_lookup ON clinical_project_export_sources(tenant_id,export_id,entity_type,entity_id);
CREATE TRIGGER clinical_project_export_sources_no_delete BEFORE DELETE ON clinical_project_export_sources FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE clinical_external_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    source_export_id UUID NOT NULL,
    source_type TEXT NOT NULL DEFAULT 'manual_external_ai' CHECK(source_type = 'manual_external_ai'),
    provider_name TEXT,
    model_name TEXT,
    surface TEXT,
    project_context_version TEXT,
    provenance_assertion TEXT NOT NULL DEFAULT 'user_supplied' CHECK(provenance_assertion = 'user_supplied'),
    content_hash TEXT NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
    imported_by_user_id UUID NOT NULL,
    proposal_json JSONB NOT NULL CHECK(jsonb_typeof(proposal_json) = 'object'),
    imported_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_external_proposals_export_fkey FOREIGN KEY(tenant_id,source_export_id,client_id) REFERENCES clinical_project_exports(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_external_proposals_importer_fkey FOREIGN KEY(tenant_id,imported_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_external_proposals_tenant_id_unique UNIQUE(tenant_id,id),
    CONSTRAINT clinical_external_proposals_tenant_client_unique UNIQUE(tenant_id,id,client_id),
    CONSTRAINT clinical_external_proposals_idempotency UNIQUE(tenant_id,source_export_id,content_hash)
);
CREATE TRIGGER clinical_external_proposals_no_delete BEFORE DELETE ON clinical_external_proposals FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

ALTER TABLE clinical_diffs ADD COLUMN source_external_proposal_id UUID;
ALTER TABLE clinical_diffs ADD CONSTRAINT clinical_diffs_external_proposal_fkey FOREIGN KEY(tenant_id,source_external_proposal_id,client_id) REFERENCES clinical_external_proposals(tenant_id,id,client_id) ON DELETE RESTRICT;
ALTER TABLE clinical_diffs ADD CONSTRAINT clinical_diffs_single_proposal_source CHECK (NOT (source_ai_run_id IS NOT NULL AND source_external_proposal_id IS NOT NULL));
CREATE UNIQUE INDEX clinical_diffs_external_proposal_unique ON clinical_diffs(tenant_id,source_external_proposal_id) WHERE source_external_proposal_id IS NOT NULL;

CREATE FUNCTION protect_clinical_project_provenance() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'Clinical project provenance is immutable' USING ERRCODE='23514';
END;
$$;
CREATE TRIGGER clinical_project_exports_immutable BEFORE UPDATE ON clinical_project_exports FOR EACH ROW EXECUTE FUNCTION protect_clinical_project_provenance();
CREATE TRIGGER clinical_project_export_sources_immutable BEFORE UPDATE ON clinical_project_export_sources FOR EACH ROW EXECUTE FUNCTION protect_clinical_project_provenance();
CREATE TRIGGER clinical_external_proposals_immutable BEFORE UPDATE ON clinical_external_proposals FOR EACH ROW EXECUTE FUNCTION protect_clinical_project_provenance();

-- Polymorphic source references are checked against a closed table allow-list.
-- Their exact entity version and tenant/client ownership must exist at export time.
CREATE FUNCTION validate_clinical_project_source() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE source_table TEXT; version_column TEXT := 'version'; valid_source BOOLEAN;
BEGIN
    source_table := CASE NEW.entity_type
        WHEN 'process' THEN 'clinical_processes' WHEN 'evidence' THEN 'clinical_evidence'
        WHEN 'event' THEN 'clinical_events' WHEN 'hypothesis' THEN 'clinical_hypotheses'
        WHEN 'target' THEN 'clinical_targets' WHEN 'goal' THEN 'clinical_goals'
        WHEN 'indicator' THEN 'goal_indicators' WHEN 'rationale' THEN 'therapeutic_rationales'
        WHEN 'gira' THEN 'giras' ELSE NULL END;
    IF source_table IS NULL THEN
        RAISE EXCEPTION 'Unsupported project source type' USING ERRCODE='23514';
    END IF;
    IF NEW.entity_type='gira' THEN version_column := 'entity_version'; END IF;
    EXECUTE format('SELECT EXISTS(SELECT 1 FROM %I WHERE tenant_id=$1 AND client_id=$2 AND id=$3 AND %I=$4)',source_table,version_column)
        INTO valid_source USING NEW.tenant_id,NEW.client_id,NEW.entity_id,NEW.entity_version;
    IF NOT valid_source THEN
        RAISE EXCEPTION 'Project source must match tenant, client and version' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER clinical_project_export_sources_validate BEFORE INSERT ON clinical_project_export_sources FOR EACH ROW EXECUTE FUNCTION validate_clinical_project_source();
