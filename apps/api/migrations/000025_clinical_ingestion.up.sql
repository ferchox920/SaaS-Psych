CREATE TABLE clinical_consent_definitions (
 scope TEXT NOT NULL CHECK(scope IN ('LOCAL_AI_PROCESSING','AUDIO_RECORDING','LOCAL_TRANSCRIPTION','EXTERNAL_MANUAL_AI_PROCESSING')),
 version INTEGER NOT NULL CHECK(version>0), purpose TEXT NOT NULL,
 PRIMARY KEY(scope,version)
);
INSERT INTO clinical_consent_definitions VALUES
 ('LOCAL_AI_PROCESSING',1,'Local clinical assistance and post-session analysis'),
 ('AUDIO_RECORDING',1,'Local session audio capture and retention'),
 ('LOCAL_TRANSCRIPTION',1,'Local speech-to-text processing'),
 ('EXTERNAL_MANUAL_AI_PROCESSING',1,'Therapist-mediated external clinical context transfer');
CREATE TABLE clinical_consent_grants (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, client_id UUID NOT NULL,
 scope TEXT NOT NULL, definition_version INTEGER NOT NULL, metadata_version INTEGER NOT NULL DEFAULT 1 CHECK(metadata_version=1),
 status TEXT NOT NULL DEFAULT 'granted' CHECK(status IN ('granted','revoked')),
 granted_by_user_id UUID NOT NULL, granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 revoked_by_user_id UUID, revoked_at TIMESTAMPTZ,
 FOREIGN KEY(scope,definition_version) REFERENCES clinical_consent_definitions(scope,version),
 FOREIGN KEY(tenant_id,client_id) REFERENCES clients(tenant_id,id),
 FOREIGN KEY(tenant_id,granted_by_user_id) REFERENCES users(tenant_id,id),
 FOREIGN KEY(tenant_id,revoked_by_user_id) REFERENCES users(tenant_id,id),
 UNIQUE(tenant_id,id,client_id), UNIQUE(tenant_id,id,client_id,scope,definition_version),
 CHECK((status='granted' AND revoked_at IS NULL AND revoked_by_user_id IS NULL) OR (status='revoked' AND revoked_at IS NOT NULL AND revoked_by_user_id IS NOT NULL))
);
CREATE UNIQUE INDEX clinical_consent_one_active ON clinical_consent_grants(tenant_id,client_id,scope) WHERE status='granted';
CREATE TABLE clinical_session_artifacts (
 id UUID PRIMARY KEY, tenant_id UUID NOT NULL, client_id UUID NOT NULL, session_id UUID NOT NULL,
 kind TEXT NOT NULL DEFAULT 'audio' CHECK(kind='audio'), version INTEGER NOT NULL DEFAULT 1 CHECK(version=1),
 status TEXT NOT NULL DEFAULT 'uploading' CHECK(status IN ('uploading','available','deleting','deleted','missing')),
 media_type TEXT NOT NULL CHECK(media_type IN ('wav','webm','ogg','mp4','m4a')),
 storage_key TEXT NOT NULL, encryption_key_id TEXT NOT NULL,
 content_hash TEXT CHECK(content_hash ~ '^[0-9a-f]{64}$'), size_bytes BIGINT CHECK(size_bytes>0),
 consent_grant_id UUID NOT NULL, created_by_user_id UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 retention_until TIMESTAMPTZ NOT NULL, deletion_requested_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ,
 FOREIGN KEY(tenant_id,session_id,client_id) REFERENCES clinical_sessions(tenant_id,id,client_id),
 FOREIGN KEY(tenant_id,consent_grant_id,client_id) REFERENCES clinical_consent_grants(tenant_id,id,client_id),
 FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id),
 UNIQUE(tenant_id,id,client_id,session_id), UNIQUE(storage_key),
 CHECK(status<>'available' OR (content_hash IS NOT NULL AND size_bytes IS NOT NULL)),
 CHECK((status='deleted')=(deleted_at IS NOT NULL))
);
CREATE INDEX clinical_artifacts_retention ON clinical_session_artifacts(tenant_id,status,retention_until);
CREATE TABLE clinical_transcript_versions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, client_id UUID NOT NULL, session_id UUID NOT NULL,
 version INTEGER NOT NULL CHECK(version>0), parent_version_id UUID,
 source_artifact_id UUID NOT NULL, source_artifact_hash TEXT NOT NULL, source_artifact_version INTEGER NOT NULL DEFAULT 1,
 origin TEXT NOT NULL CHECK(origin IN ('machine','human_asr_correction')),
 status TEXT NOT NULL DEFAULT 'available' CHECK(status IN ('available','deleted')),
 transcript_text TEXT, segments_json JSONB, language TEXT NOT NULL,
 engine TEXT NOT NULL, model TEXT NOT NULL, engine_version TEXT NOT NULL, configuration_hash TEXT NOT NULL,
 content_hash TEXT NOT NULL CHECK(content_hash ~ '^[0-9a-f]{64}$'),
 created_by_user_id UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ,
 FOREIGN KEY(tenant_id,source_artifact_id,client_id,session_id) REFERENCES clinical_session_artifacts(tenant_id,id,client_id,session_id),
 FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id),
 UNIQUE(tenant_id,session_id,version), UNIQUE(tenant_id,id,client_id,session_id),
 FOREIGN KEY(tenant_id,parent_version_id,client_id,session_id) REFERENCES clinical_transcript_versions(tenant_id,id,client_id,session_id),
 CHECK((status='available' AND transcript_text IS NOT NULL AND segments_json IS NOT NULL AND deleted_at IS NULL) OR (status='deleted' AND transcript_text IS NULL AND segments_json IS NULL AND deleted_at IS NOT NULL))
);
CREATE TABLE clinical_ingestion_jobs (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, client_id UUID NOT NULL, session_id UUID NOT NULL,
 job_type TEXT NOT NULL CHECK(job_type IN ('transcribe_session_audio','analyze_session','artifact_cleanup')),
 artifact_id UUID, transcript_version_id UUID,
 status TEXT NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','succeeded','failed','cancelled')),
 attempt INTEGER NOT NULL DEFAULT 0 CHECK(attempt>=0), max_attempts INTEGER NOT NULL DEFAULT 3 CHECK(max_attempts BETWEEN 1 AND 5),
 available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), started_at TIMESTAMPTZ, completed_at TIMESTAMPTZ, cancelled_at TIMESTAMPTZ,
 lease_token UUID, lease_until TIMESTAMPTZ, error_code TEXT,
 idempotency_key TEXT NOT NULL, configuration_hash TEXT NOT NULL, language TEXT NOT NULL DEFAULT 'es',
 created_by_user_id UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 FOREIGN KEY(tenant_id,session_id,client_id) REFERENCES clinical_sessions(tenant_id,id,client_id),
 FOREIGN KEY(tenant_id,artifact_id,client_id,session_id) REFERENCES clinical_session_artifacts(tenant_id,id,client_id,session_id),
 FOREIGN KEY(tenant_id,transcript_version_id,client_id,session_id) REFERENCES clinical_transcript_versions(tenant_id,id,client_id,session_id),
 FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id),
 UNIQUE(tenant_id,id,client_id,session_id), UNIQUE(tenant_id,client_id,idempotency_key),
 CHECK((job_type IN ('transcribe_session_audio','artifact_cleanup') AND artifact_id IS NOT NULL AND transcript_version_id IS NULL) OR (job_type='analyze_session' AND artifact_id IS NULL AND transcript_version_id IS NOT NULL)),
 CHECK((status='running')=(lease_token IS NOT NULL AND lease_until IS NOT NULL))
);
CREATE INDEX clinical_ingestion_jobs_claim ON clinical_ingestion_jobs(available_at,created_at,id) WHERE status='queued';
CREATE INDEX clinical_ingestion_jobs_lease ON clinical_ingestion_jobs(lease_until) WHERE status='running';
CREATE TABLE clinical_ingestion_job_attempts (
 tenant_id UUID NOT NULL, client_id UUID NOT NULL, session_id UUID NOT NULL, job_id UUID NOT NULL,
 attempt INTEGER NOT NULL, lease_token UUID NOT NULL, started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 completed_at TIMESTAMPTZ, outcome TEXT NOT NULL DEFAULT 'running', error_code TEXT,
 PRIMARY KEY(tenant_id,job_id,attempt),
 FOREIGN KEY(tenant_id,job_id,client_id,session_id) REFERENCES clinical_ingestion_jobs(tenant_id,id,client_id,session_id)
);
CREATE TABLE clinical_ingestion_authorizations (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),tenant_id UUID NOT NULL,client_id UUID NOT NULL,
 grant_id UUID NOT NULL, scope TEXT NOT NULL, definition_version INTEGER NOT NULL,
 operation TEXT NOT NULL, entity_id UUID NOT NULL, authorized_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 FOREIGN KEY(tenant_id,grant_id,client_id,scope,definition_version) REFERENCES clinical_consent_grants(tenant_id,id,client_id,scope,definition_version),
 FOREIGN KEY(scope,definition_version) REFERENCES clinical_consent_definitions(scope,version)
);
CREATE INDEX clinical_ingestion_auth_source ON clinical_ingestion_authorizations(tenant_id,entity_id,authorized_at);
CREATE TABLE clinical_ingestion_results (
 tenant_id UUID NOT NULL,client_id UUID NOT NULL,session_id UUID NOT NULL,job_id UUID NOT NULL,
 transcript_version_id UUID, ai_run_id UUID, report_id UUID,
 PRIMARY KEY(tenant_id,job_id),
 FOREIGN KEY(tenant_id,job_id,client_id,session_id) REFERENCES clinical_ingestion_jobs(tenant_id,id,client_id,session_id),
 FOREIGN KEY(tenant_id,transcript_version_id,client_id,session_id) REFERENCES clinical_transcript_versions(tenant_id,id,client_id,session_id),
 FOREIGN KEY(tenant_id,ai_run_id) REFERENCES clinical_ai_runs(tenant_id,id),
 FOREIGN KEY(tenant_id,report_id) REFERENCES session_reports(tenant_id,id)
);
CREATE TABLE clinical_ingestion_run_attempts (
 tenant_id UUID NOT NULL,client_id UUID NOT NULL,session_id UUID NOT NULL,job_id UUID NOT NULL,attempt INTEGER NOT NULL,
 ai_run_id UUID NOT NULL, transcript_version_id UUID NOT NULL,
 PRIMARY KEY(tenant_id,job_id,attempt),
 FOREIGN KEY(tenant_id,job_id,attempt) REFERENCES clinical_ingestion_job_attempts(tenant_id,job_id,attempt),
 FOREIGN KEY(tenant_id,ai_run_id) REFERENCES clinical_ai_runs(tenant_id,id),
 FOREIGN KEY(tenant_id,transcript_version_id,client_id,session_id) REFERENCES clinical_transcript_versions(tenant_id,id,client_id,session_id)
);
CREATE TRIGGER clinical_consent_definitions_immutable BEFORE UPDATE OR DELETE ON clinical_consent_definitions FOR EACH ROW EXECUTE FUNCTION protect_clinical_project_provenance();
CREATE TRIGGER clinical_ingestion_authorizations_immutable BEFORE UPDATE OR DELETE ON clinical_ingestion_authorizations FOR EACH ROW EXECUTE FUNCTION protect_clinical_project_provenance();
CREATE TRIGGER clinical_consent_grants_no_delete BEFORE DELETE ON clinical_consent_grants FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();
CREATE FUNCTION protect_clinical_consent_grant() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF (to_jsonb(NEW)-ARRAY['status','revoked_by_user_id','revoked_at']) IS DISTINCT FROM
    (to_jsonb(OLD)-ARRAY['status','revoked_by_user_id','revoked_at']) OR OLD.status='revoked' OR NEW.status<>'revoked'
 THEN RAISE EXCEPTION 'consent grants are immutable except prospective revocation' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER clinical_consent_grant_immutable BEFORE UPDATE ON clinical_consent_grants FOR EACH ROW EXECUTE FUNCTION protect_clinical_consent_grant();
CREATE FUNCTION protect_clinical_transcript_version() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'transcript provenance cannot be hard deleted' USING ERRCODE='23514'; END IF;
 IF (to_jsonb(NEW)-ARRAY['status','transcript_text','segments_json','deleted_at']) IS DISTINCT FROM
    (to_jsonb(OLD)-ARRAY['status','transcript_text','segments_json','deleted_at']) OR OLD.status<>'available' OR NEW.status<>'deleted'
 THEN RAISE EXCEPTION 'transcript revisions require a new version' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER clinical_transcript_version_immutable BEFORE UPDATE OR DELETE ON clinical_transcript_versions FOR EACH ROW EXECUTE FUNCTION protect_clinical_transcript_version();
CREATE FUNCTION validate_clinical_ingestion_source() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF TG_TABLE_NAME='clinical_session_artifacts' THEN
  IF NEW.storage_key<>NEW.tenant_id::text||'/'||NEW.client_id::text||'/'||NEW.session_id::text||'/'||NEW.id::text
   OR NOT EXISTS(SELECT 1 FROM clinical_consent_grants WHERE tenant_id=NEW.tenant_id AND id=NEW.consent_grant_id AND client_id=NEW.client_id AND scope='AUDIO_RECORDING')
  THEN RAISE EXCEPTION 'invalid artifact identity or consent scope' USING ERRCODE='23514'; END IF;
 ELSE
  IF NOT EXISTS(SELECT 1 FROM clinical_session_artifacts WHERE tenant_id=NEW.tenant_id AND id=NEW.source_artifact_id AND client_id=NEW.client_id AND session_id=NEW.session_id AND content_hash=NEW.source_artifact_hash AND version=NEW.source_artifact_version)
  THEN RAISE EXCEPTION 'invalid transcript source provenance' USING ERRCODE='23514'; END IF;
  IF (NEW.origin='machine' AND NEW.parent_version_id IS NOT NULL) OR (NEW.origin='human_asr_correction' AND NEW.parent_version_id IS NULL)
  THEN RAISE EXCEPTION 'invalid transcript correction provenance' USING ERRCODE='23514'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER clinical_artifact_source_guard BEFORE INSERT ON clinical_session_artifacts FOR EACH ROW EXECUTE FUNCTION validate_clinical_ingestion_source();
CREATE TRIGGER clinical_transcript_source_guard BEFORE INSERT ON clinical_transcript_versions FOR EACH ROW EXECUTE FUNCTION validate_clinical_ingestion_source();
CREATE TRIGGER clinical_artifacts_no_delete BEFORE DELETE ON clinical_session_artifacts FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();
CREATE INDEX clinical_ingestion_runs_source ON clinical_ingestion_run_attempts(tenant_id,ai_run_id);
