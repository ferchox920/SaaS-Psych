CREATE UNIQUE INDEX IF NOT EXISTS session_reports_tenant_id_session_unique
    ON session_reports(tenant_id,id,clinical_session_id);

CREATE TABLE clinical_longitudinal_heads (
    tenant_id UUID NOT NULL, client_id UUID NOT NULL, revision BIGINT NOT NULL DEFAULT 0 CHECK(revision>=0), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(tenant_id,client_id), CONSTRAINT clinical_longitudinal_heads_client_fkey FOREIGN KEY(tenant_id,client_id) REFERENCES clients(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE clinical_diffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, client_id UUID NOT NULL,
    clinical_session_id UUID, source_session_report_id UUID, source_ai_run_id UUID,
    status TEXT NOT NULL CHECK(status IN ('draft','pending_review','partially_reviewed','approved','rejected','merged')),
    base_state_version BIGINT NOT NULL CHECK(base_state_version>=0), revision INTEGER NOT NULL DEFAULT 1 CHECK(revision>0),
    uncertainties_json JSONB NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(uncertainties_json)='array'),
    created_by_user_id UUID, reviewed_by_user_id UUID, reviewed_at TIMESTAMPTZ, merged_by_user_id UUID, merged_at TIMESTAMPTZ,
    merged_state_version BIGINT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_diffs_client_fkey FOREIGN KEY(tenant_id,client_id) REFERENCES clients(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_diffs_session_fkey FOREIGN KEY(tenant_id,clinical_session_id,client_id) REFERENCES clinical_sessions(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_diffs_report_fkey FOREIGN KEY(tenant_id,source_session_report_id,clinical_session_id) REFERENCES session_reports(tenant_id,id,clinical_session_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_diffs_run_fkey FOREIGN KEY(tenant_id,source_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_diffs_creator_fkey FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_diffs_reviewer_fkey FOREIGN KEY(tenant_id,reviewed_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_diffs_merger_fkey FOREIGN KEY(tenant_id,merged_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_diffs_tenant_id_client_unique UNIQUE(tenant_id,id,client_id), CONSTRAINT clinical_diffs_tenant_id_unique UNIQUE(tenant_id,id)
);
CREATE UNIQUE INDEX clinical_diffs_one_open_report_revision ON clinical_diffs(tenant_id,source_session_report_id,base_state_version) WHERE status IN ('draft','pending_review','partially_reviewed','approved');
CREATE INDEX clinical_diffs_client_created ON clinical_diffs(tenant_id,client_id,created_at DESC,id DESC);
CREATE TRIGGER clinical_diffs_no_delete BEFORE DELETE ON clinical_diffs FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE clinical_diff_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, client_id UUID NOT NULL, diff_id UUID NOT NULL,
    sequence INTEGER NOT NULL CHECK(sequence>0), operation_type TEXT NOT NULL CHECK(operation_type IN ('create_evidence','invalidate_evidence','create_event','approve_event','link_event_process','create_process','update_process','close_process','reopen_process','create_hypothesis','update_hypothesis','link_supporting_evidence','link_contradicting_evidence','strengthen_hypothesis','weaken_hypothesis','retire_hypothesis')),
    target_entity_id UUID NOT NULL, expected_entity_version INTEGER CHECK(expected_entity_version IS NULL OR expected_entity_version>0),
    original_proposal JSONB NOT NULL CHECK(jsonb_typeof(original_proposal)='object'), human_modification JSONB CHECK(human_modification IS NULL OR jsonb_typeof(human_modification)='object'),
    review_status TEXT NOT NULL DEFAULT 'pending' CHECK(review_status IN ('pending','approved','modified','rejected')),
    reviewed_by_user_id UUID, reviewed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_diff_operations_review_check CHECK((review_status='pending' AND reviewed_by_user_id IS NULL AND reviewed_at IS NULL AND human_modification IS NULL) OR (review_status='modified' AND reviewed_by_user_id IS NOT NULL AND reviewed_at IS NOT NULL AND human_modification IS NOT NULL) OR (review_status IN ('approved','rejected') AND reviewed_by_user_id IS NOT NULL AND reviewed_at IS NOT NULL AND human_modification IS NULL)),
    CONSTRAINT clinical_diff_operations_diff_fkey FOREIGN KEY(tenant_id,diff_id,client_id) REFERENCES clinical_diffs(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_diff_operations_reviewer_fkey FOREIGN KEY(tenant_id,reviewed_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_diff_operations_sequence_unique UNIQUE(tenant_id,diff_id,sequence), CONSTRAINT clinical_diff_operations_tenant_id_unique UNIQUE(tenant_id,id)
);
CREATE INDEX clinical_diff_operations_diff_order ON clinical_diff_operations(tenant_id,diff_id,sequence);
CREATE TRIGGER clinical_diff_operations_no_delete BEFORE DELETE ON clinical_diff_operations FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE clinical_longitudinal_transitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, client_id UUID NOT NULL, diff_id UUID NOT NULL, operation_id UUID NOT NULL,
    entity_type TEXT NOT NULL CHECK(entity_type IN ('evidence','event','process','hypothesis')), entity_id UUID NOT NULL, action TEXT NOT NULL,
    from_version INTEGER, to_version INTEGER NOT NULL CHECK(to_version>0), from_status TEXT, to_status TEXT,
    actor_user_id UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_longitudinal_transitions_diff_fkey FOREIGN KEY(tenant_id,diff_id,client_id) REFERENCES clinical_diffs(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_longitudinal_transitions_operation_fkey FOREIGN KEY(tenant_id,operation_id) REFERENCES clinical_diff_operations(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_longitudinal_transitions_actor_fkey FOREIGN KEY(tenant_id,actor_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX clinical_longitudinal_transitions_entity ON clinical_longitudinal_transitions(tenant_id,client_id,entity_type,entity_id,created_at,id);

ALTER TABLE clinical_ai_run_sources DROP CONSTRAINT clinical_ai_run_sources_source_type_check;
ALTER TABLE clinical_ai_run_sources ADD CONSTRAINT clinical_ai_run_sources_source_type_check CHECK(source_type IN ('formulation_snapshot','formulation_anchor','session_report','clinical_evidence','clinical_event','clinical_process','clinical_hypothesis'));

CREATE OR REPLACE FUNCTION validate_clinical_ai_run_source()
RETURNS trigger AS $$
DECLARE run_client UUID;
BEGIN
    SELECT client_id INTO run_client FROM clinical_ai_runs WHERE tenant_id=NEW.tenant_id AND id=NEW.ai_run_id;
    IF NEW.source_type = 'formulation_snapshot' AND NOT EXISTS (SELECT 1 FROM clinical_formulation_snapshots WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND status='approved' AND (NEW.source_version IS NULL OR version=NEW.source_version)) THEN RAISE EXCEPTION 'formulation snapshot source must be approved and tenant-local' USING ERRCODE='23503';
    ELSIF NEW.source_type = 'formulation_anchor' AND NOT EXISTS (SELECT 1 FROM clinical_formulation_anchors a JOIN clinical_formulation_snapshots s ON s.tenant_id=a.tenant_id AND s.id=a.snapshot_id WHERE a.tenant_id=NEW.tenant_id AND a.id=NEW.source_id AND s.status='approved' AND (NEW.source_version IS NULL OR s.version=NEW.source_version)) THEN RAISE EXCEPTION 'formulation anchor source must belong to an approved tenant-local snapshot' USING ERRCODE='23503';
    ELSIF NEW.source_type = 'session_report' AND NOT EXISTS (SELECT 1 FROM session_reports r JOIN clinical_sessions s ON s.tenant_id=r.tenant_id AND s.id=r.clinical_session_id WHERE r.tenant_id=NEW.tenant_id AND r.id=NEW.source_id AND r.status='approved' AND s.client_id=run_client AND (NEW.source_version IS NULL OR r.version=NEW.source_version)) THEN RAISE EXCEPTION 'session report source must be approved and tenant-local' USING ERRCODE='23503';
    ELSIF NEW.source_type = 'clinical_evidence' AND NOT EXISTS (SELECT 1 FROM clinical_evidence WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND status='active' AND version=NEW.source_version) THEN RAISE EXCEPTION 'clinical evidence source must be active and tenant-local' USING ERRCODE='23503';
    ELSIF NEW.source_type = 'clinical_event' AND NOT EXISTS (SELECT 1 FROM clinical_events WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND version=NEW.source_version) THEN RAISE EXCEPTION 'clinical event source must be approved and tenant-local' USING ERRCODE='23503';
    ELSIF NEW.source_type = 'clinical_process' AND NOT EXISTS (SELECT 1 FROM clinical_processes WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND version=NEW.source_version) THEN RAISE EXCEPTION 'clinical process source must be approved and tenant-local' USING ERRCODE='23503';
    ELSIF NEW.source_type = 'clinical_hypothesis' AND NOT EXISTS (SELECT 1 FROM clinical_hypotheses WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND version=NEW.source_version) THEN RAISE EXCEPTION 'clinical hypothesis source must be approved and tenant-local' USING ERRCODE='23503';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
