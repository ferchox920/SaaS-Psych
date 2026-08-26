CREATE TABLE clinical_processes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, client_id UUID NOT NULL,
    title TEXT NOT NULL CHECK(length(btrim(title))>0), description TEXT NOT NULL CHECK(length(btrim(description))>0),
    approval_status TEXT NOT NULL DEFAULT 'proposed' CHECK(approval_status IN ('proposed','approved','rejected')),
    clinical_status TEXT NOT NULL DEFAULT 'observing' CHECK(clinical_status IN ('observing','active','stabilized','closed')),
    version INTEGER NOT NULL DEFAULT 1 CHECK(version>0), created_by_user_id UUID, created_from_ai_run_id UUID,
    approved_by_user_id UUID, approved_at TIMESTAMPTZ, opened_at TIMESTAMPTZ, closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_processes_approval_check CHECK((approval_status='proposed' AND approved_by_user_id IS NULL AND approved_at IS NULL) OR (approval_status='approved' AND approved_by_user_id IS NOT NULL AND approved_at IS NOT NULL) OR (approval_status='rejected' AND approved_at IS NOT NULL)),
    CONSTRAINT clinical_processes_status_time_check CHECK((clinical_status='closed' AND closed_at IS NOT NULL) OR clinical_status<>'closed'),
    CONSTRAINT clinical_processes_provenance_check CHECK(created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL),
    CONSTRAINT clinical_processes_client_fkey FOREIGN KEY(tenant_id,client_id) REFERENCES clients(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_processes_creator_fkey FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_processes_approver_fkey FOREIGN KEY(tenant_id,approved_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_processes_run_fkey FOREIGN KEY(tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_processes_tenant_id_client_unique UNIQUE(tenant_id,id,client_id), CONSTRAINT clinical_processes_tenant_id_unique UNIQUE(tenant_id,id)
);
CREATE INDEX clinical_processes_client_status ON clinical_processes(tenant_id,client_id,approval_status,clinical_status,updated_at DESC,id DESC);
CREATE TRIGGER clinical_processes_no_delete BEFORE DELETE ON clinical_processes FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE clinical_process_events (
    tenant_id UUID NOT NULL, client_id UUID NOT NULL, process_id UUID NOT NULL, event_id UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(tenant_id,process_id,event_id),
    CONSTRAINT clinical_process_events_process_fkey FOREIGN KEY(tenant_id,process_id,client_id) REFERENCES clinical_processes(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_process_events_event_fkey FOREIGN KEY(tenant_id,event_id,client_id) REFERENCES clinical_events(tenant_id,id,client_id) ON DELETE RESTRICT
);
CREATE INDEX clinical_process_events_event ON clinical_process_events(tenant_id,event_id,process_id);

CREATE TABLE clinical_hypotheses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, client_id UUID NOT NULL, process_id UUID,
    statement TEXT NOT NULL CHECK(length(btrim(statement))>0), hypothesis_type TEXT,
    approval_status TEXT NOT NULL DEFAULT 'proposed' CHECK(approval_status IN ('proposed','approved','rejected')),
    clinical_status TEXT NOT NULL DEFAULT 'active' CHECK(clinical_status IN ('active','weakened','superseded','retired')),
    confidence_level TEXT NOT NULL CHECK(confidence_level IN ('red','yellow','green')),
    version INTEGER NOT NULL DEFAULT 1 CHECK(version>0), created_by_user_id UUID, created_from_ai_run_id UUID,
    approved_by_user_id UUID, approved_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_hypotheses_approval_check CHECK((approval_status='proposed' AND approved_by_user_id IS NULL AND approved_at IS NULL) OR (approval_status='approved' AND approved_by_user_id IS NOT NULL AND approved_at IS NOT NULL) OR (approval_status='rejected' AND approved_at IS NOT NULL)),
    CONSTRAINT clinical_hypotheses_provenance_check CHECK(created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL),
    CONSTRAINT clinical_hypotheses_client_fkey FOREIGN KEY(tenant_id,client_id) REFERENCES clients(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_hypotheses_process_fkey FOREIGN KEY(tenant_id,process_id,client_id) REFERENCES clinical_processes(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_hypotheses_creator_fkey FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_hypotheses_approver_fkey FOREIGN KEY(tenant_id,approved_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_hypotheses_run_fkey FOREIGN KEY(tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_hypotheses_tenant_id_client_unique UNIQUE(tenant_id,id,client_id), CONSTRAINT clinical_hypotheses_tenant_id_unique UNIQUE(tenant_id,id)
);
CREATE INDEX clinical_hypotheses_client_status ON clinical_hypotheses(tenant_id,client_id,approval_status,clinical_status,updated_at DESC,id DESC);
CREATE TRIGGER clinical_hypotheses_no_delete BEFORE DELETE ON clinical_hypotheses FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE clinical_hypothesis_evidence (
    tenant_id UUID NOT NULL, client_id UUID NOT NULL, hypothesis_id UUID NOT NULL, evidence_id UUID NOT NULL,
    relation_type TEXT NOT NULL CHECK(relation_type IN ('supporting','contradicting')), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(tenant_id,hypothesis_id,evidence_id),
    CONSTRAINT clinical_hypothesis_evidence_hypothesis_fkey FOREIGN KEY(tenant_id,hypothesis_id,client_id) REFERENCES clinical_hypotheses(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_hypothesis_evidence_evidence_fkey FOREIGN KEY(tenant_id,evidence_id,client_id) REFERENCES clinical_evidence(tenant_id,id,client_id) ON DELETE RESTRICT
);
CREATE INDEX clinical_hypothesis_evidence_evidence ON clinical_hypothesis_evidence(tenant_id,evidence_id,hypothesis_id);

CREATE OR REPLACE FUNCTION ensure_approved_hypothesis_has_support()
RETURNS trigger AS $$
DECLARE checked_tenant UUID; checked_hypothesis UUID; approval TEXT;
BEGIN
  checked_tenant:=COALESCE(NEW.tenant_id,OLD.tenant_id);
  IF TG_TABLE_NAME='clinical_hypotheses' THEN checked_hypothesis:=COALESCE(NEW.id,OLD.id);
  ELSE checked_hypothesis:=COALESCE(NEW.hypothesis_id,OLD.hypothesis_id); END IF;
  SELECT approval_status INTO approval FROM clinical_hypotheses WHERE tenant_id=checked_tenant AND id=checked_hypothesis;
  IF approval='approved' AND NOT EXISTS(SELECT 1 FROM clinical_hypothesis_evidence WHERE tenant_id=checked_tenant AND hypothesis_id=checked_hypothesis AND relation_type='supporting') THEN
    RAISE EXCEPTION 'approved clinical hypothesis requires supporting evidence' USING ERRCODE='23514';
  END IF;
  RETURN COALESCE(NEW,OLD);
END;
$$ LANGUAGE plpgsql;
CREATE CONSTRAINT TRIGGER clinical_hypotheses_require_support AFTER INSERT OR UPDATE OF approval_status ON clinical_hypotheses DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_approved_hypothesis_has_support();
CREATE CONSTRAINT TRIGGER clinical_hypothesis_evidence_preserve_support AFTER DELETE OR UPDATE OF relation_type ON clinical_hypothesis_evidence DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_approved_hypothesis_has_support();
