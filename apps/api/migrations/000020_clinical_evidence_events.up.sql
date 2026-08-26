CREATE UNIQUE INDEX IF NOT EXISTS clinical_ai_runs_tenant_id_client_unique
    ON clinical_ai_runs (tenant_id, id, client_id);

CREATE TABLE clinical_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('session_report')),
    source_id UUID NOT NULL,
    source_version INTEGER NOT NULL CHECK (source_version > 0),
    source_item_id TEXT NOT NULL CHECK (length(btrim(source_item_id)) > 0),
    epistemic_type TEXT NOT NULL CHECK (epistemic_type IN ('patient_report','therapist_observation','measurement','documented_fact','inference')),
    statement TEXT NOT NULL CHECK (length(btrim(statement)) > 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','superseded','invalidated')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by_user_id UUID,
    created_from_ai_run_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_evidence_provenance_check CHECK (created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL),
    CONSTRAINT clinical_evidence_tenant_client_fkey FOREIGN KEY (tenant_id,client_id) REFERENCES clients(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_evidence_tenant_creator_fkey FOREIGN KEY (tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_evidence_tenant_run_client_fkey FOREIGN KEY (tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_evidence_tenant_id_client_unique UNIQUE (tenant_id,id,client_id),
    CONSTRAINT clinical_evidence_tenant_id_unique UNIQUE (tenant_id,id)
);
CREATE UNIQUE INDEX clinical_evidence_one_active_source
    ON clinical_evidence(tenant_id,client_id,source_type,source_id,source_version,source_item_id)
    WHERE status='active';
CREATE INDEX clinical_evidence_client_status ON clinical_evidence(tenant_id,client_id,status,created_at DESC,id DESC);

CREATE OR REPLACE FUNCTION validate_clinical_evidence_source()
RETURNS trigger AS $$
DECLARE source_statement TEXT;
BEGIN
    SELECT CASE
        WHEN item->>'statement' IS NOT NULL THEN item->>'statement'
        ELSE item->>'description'
    END INTO source_statement
    FROM session_reports r
    JOIN clinical_sessions s ON s.tenant_id=r.tenant_id AND s.id=r.clinical_session_id
    CROSS JOIN LATERAL jsonb_array_elements(
        COALESCE(r.report_json->'facts','[]'::jsonb) ||
        COALESCE(r.report_json->'relevant_changes','[]'::jsonb) ||
        COALESCE(r.report_json->'patient_responses','[]'::jsonb) ||
        COALESCE(r.report_json->'affective_nodes','[]'::jsonb)
    ) item
    WHERE r.tenant_id=NEW.tenant_id AND r.id=NEW.source_id AND r.version=NEW.source_version
      AND r.status IN ('approved','superseded') AND s.client_id=NEW.client_id
      AND item->>'id'=NEW.source_item_id
    LIMIT 1;
    IF source_statement IS NULL THEN
        RAISE EXCEPTION 'clinical evidence source must be an eligible item in a tenant-local approved report' USING ERRCODE='23503';
    END IF;
    IF btrim(regexp_replace(NEW.statement, '\s+', ' ', 'g')) <> btrim(regexp_replace(source_statement, '\s+', ' ', 'g')) THEN
        RAISE EXCEPTION 'clinical evidence statement must preserve source item content' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER clinical_evidence_validate_source
BEFORE INSERT ON clinical_evidence FOR EACH ROW EXECUTE FUNCTION validate_clinical_evidence_source();

CREATE OR REPLACE FUNCTION prevent_clinical_evidence_provenance_update()
RETURNS trigger AS $$
BEGIN
  IF ROW(NEW.tenant_id,NEW.client_id,NEW.source_type,NEW.source_id,NEW.source_version,NEW.source_item_id,NEW.epistemic_type,NEW.statement,NEW.created_by_user_id,NEW.created_from_ai_run_id,NEW.created_at)
     IS DISTINCT FROM ROW(OLD.tenant_id,OLD.client_id,OLD.source_type,OLD.source_id,OLD.source_version,OLD.source_item_id,OLD.epistemic_type,OLD.statement,OLD.created_by_user_id,OLD.created_from_ai_run_id,OLD.created_at) THEN
    RAISE EXCEPTION 'clinical evidence provenance is immutable' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER clinical_evidence_immutable_provenance BEFORE UPDATE ON clinical_evidence FOR EACH ROW EXECUTE FUNCTION prevent_clinical_evidence_provenance_update();

CREATE OR REPLACE FUNCTION prevent_clinical_longitudinal_delete()
RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'longitudinal clinical records cannot be physically deleted' USING ERRCODE='23514'; END; $$ LANGUAGE plpgsql;
CREATE TRIGGER clinical_evidence_no_delete BEFORE DELETE ON clinical_evidence FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE clinical_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('reported_change','behavior','decision','affective_shift','relationship_event','symptom_change','coping_strategy','therapeutic_response','safety_event','context_change','other')),
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    description TEXT NOT NULL CHECK (length(btrim(description)) > 0),
    occurred_at TIMESTAMPTZ,
    observed_at TIMESTAMPTZ NOT NULL,
    approval_status TEXT NOT NULL DEFAULT 'proposed' CHECK (approval_status IN ('proposed','approved','rejected')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by_user_id UUID,
    created_from_ai_run_id UUID,
    approved_by_user_id UUID,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_events_approval_check CHECK (
      (approval_status='proposed' AND approved_by_user_id IS NULL AND approved_at IS NULL) OR
      (approval_status='approved' AND approved_by_user_id IS NOT NULL AND approved_at IS NOT NULL) OR
      (approval_status='rejected' AND approved_at IS NOT NULL)
    ),
    CONSTRAINT clinical_events_provenance_check CHECK (created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL),
    CONSTRAINT clinical_events_tenant_client_fkey FOREIGN KEY (tenant_id,client_id) REFERENCES clients(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_events_tenant_creator_fkey FOREIGN KEY (tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_events_tenant_approver_fkey FOREIGN KEY (tenant_id,approved_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_events_tenant_run_client_fkey FOREIGN KEY (tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_events_tenant_id_client_unique UNIQUE (tenant_id,id,client_id),
    CONSTRAINT clinical_events_tenant_id_unique UNIQUE (tenant_id,id)
);
CREATE INDEX clinical_events_client_observed ON clinical_events(tenant_id,client_id,observed_at DESC,id DESC);
CREATE TRIGGER clinical_events_no_delete BEFORE DELETE ON clinical_events FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE clinical_event_evidence (
    tenant_id UUID NOT NULL,
    client_id UUID NOT NULL,
    event_id UUID NOT NULL,
    evidence_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id,event_id,evidence_id),
    CONSTRAINT clinical_event_evidence_event_fkey FOREIGN KEY (tenant_id,event_id,client_id) REFERENCES clinical_events(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_event_evidence_evidence_fkey FOREIGN KEY (tenant_id,evidence_id,client_id) REFERENCES clinical_evidence(tenant_id,id,client_id) ON DELETE RESTRICT
);
CREATE INDEX clinical_event_evidence_evidence ON clinical_event_evidence(tenant_id,evidence_id,event_id);

CREATE OR REPLACE FUNCTION ensure_approved_event_has_evidence()
RETURNS trigger AS $$
DECLARE checked_tenant UUID; checked_event UUID; event_status TEXT;
BEGIN
    IF TG_TABLE_NAME='clinical_events' THEN
      checked_tenant := COALESCE(NEW.tenant_id,OLD.tenant_id);
      checked_event := COALESCE(NEW.id,OLD.id);
    ELSE
      checked_tenant := COALESCE(NEW.tenant_id,OLD.tenant_id);
      checked_event := COALESCE(NEW.event_id,OLD.event_id);
    END IF;
    SELECT approval_status INTO event_status FROM clinical_events WHERE tenant_id=checked_tenant AND id=checked_event;
    IF event_status='approved' AND NOT EXISTS (SELECT 1 FROM clinical_event_evidence WHERE tenant_id=checked_tenant AND event_id=checked_event) THEN
      RAISE EXCEPTION 'approved clinical event requires evidence' USING ERRCODE='23514';
    END IF;
    RETURN COALESCE(NEW,OLD);
END;
$$ LANGUAGE plpgsql;
CREATE CONSTRAINT TRIGGER clinical_events_require_evidence
AFTER INSERT OR UPDATE OF approval_status ON clinical_events DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION ensure_approved_event_has_evidence();
CREATE CONSTRAINT TRIGGER clinical_event_evidence_preserve_requirement
AFTER DELETE ON clinical_event_evidence DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION ensure_approved_event_has_evidence();
