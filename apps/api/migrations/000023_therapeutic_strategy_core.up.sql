CREATE TABLE therapeutic_approach_definitions (
	 id UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    slug TEXT NOT NULL CHECK(length(btrim(slug))>0), version INTEGER NOT NULL CHECK(version>0),
    name TEXT NOT NULL CHECK(length(btrim(name))>0), description TEXT NOT NULL CHECK(length(btrim(description))>0),
    target_domains TEXT[] NOT NULL DEFAULT '{}', core_mechanisms TEXT[] NOT NULL DEFAULT '{}', intervention_families TEXT[] NOT NULL DEFAULT '{}',
    progress_signals TEXT[] NOT NULL DEFAULT '{}', limitations TEXT[] NOT NULL DEFAULT '{}', cautions TEXT[] NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN('active','deprecated')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(slug,version)
);
CREATE INDEX therapeutic_approaches_status ON therapeutic_approach_definitions(status,slug,version DESC);
CREATE TRIGGER therapeutic_approaches_no_delete BEFORE DELETE ON therapeutic_approach_definitions FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE therapeutic_technique_definitions (
	 id UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    slug TEXT NOT NULL CHECK(length(btrim(slug))>0), version INTEGER NOT NULL CHECK(version>0),
    name TEXT NOT NULL CHECK(length(btrim(name))>0), approach_slug TEXT NOT NULL, approach_version INTEGER NOT NULL,
    description TEXT NOT NULL CHECK(length(btrim(description))>0), target_domains TEXT[] NOT NULL DEFAULT '{}',
    mechanism TEXT NOT NULL CHECK(length(btrim(mechanism))>0), indications TEXT[] NOT NULL DEFAULT '{}', cautions TEXT[] NOT NULL DEFAULT '{}',
    limits TEXT[] NOT NULL DEFAULT '{}', expected_signals TEXT[] NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN('active','deprecated')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(slug,version),
    CONSTRAINT therapeutic_techniques_approach_fkey FOREIGN KEY(approach_slug,approach_version) REFERENCES therapeutic_approach_definitions(slug,version) ON DELETE RESTRICT,
    CONSTRAINT therapeutic_techniques_compat_unique UNIQUE(slug,version,approach_slug,approach_version)
);
CREATE INDEX therapeutic_techniques_approach ON therapeutic_technique_definitions(approach_slug,approach_version,status,slug,version DESC);
CREATE TRIGGER therapeutic_techniques_no_delete BEFORE DELETE ON therapeutic_technique_definitions FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

INSERT INTO therapeutic_approach_definitions(slug,version,name,description,target_domains,core_mechanisms,intervention_families,progress_signals,limitations,cautions) VALUES
('cbt',1,'TCC / CBT','Modelo cognitivo-conductual orientado a relaciones funcionales entre situación, interpretación, emoción, conducta y consecuencia.',ARRAY['behavioral_pattern','cognitive_pattern','skill_deficit'],ARRAY['maintenance_cycle','avoidance','safety_behavior','rules','beliefs'],ARRAY['behavioral_experiment','exposure','skills_training','cognitive_reappraisal'],ARRAY['reduced_avoidance','new_behavior','belief_flexibility'],ARRAY['No presume que todo malestar deriva de cogniciones distorsionadas.'],ARRAY['Graduar exposición y evaluar seguridad clínica.']),
('act',1,'ACT','Modelo contextual orientado a flexibilidad psicológica y acción guiada por valores aun con malestar presente.',ARRAY['experiential_avoidance','meaning_value_conflict','emotional_regulation'],ARRAY['acceptance','defusion','values','committed_action','present_moment','self_as_context'],ARRAY['values_clarification','defusion_practice','acceptance_practice','committed_action'],ARRAY['willingness','values_consistent_action','reduced_experiential_control'],ARRAY['No usa aceptación como resignación ni exige eliminar síntomas.'],ARRAY['No convertir valores en prescripciones del terapeuta.']),
('logotherapy',1,'Logoterapia','Abordaje de libertad situada, responsabilidad, significado, valores, culpa, finitud y actitud ante límites irreversibles.',ARRAY['meaning_value_conflict','cognitive_pattern'],ARRAY['freedom_within_limits','responsibility_without_omnipotence','meaning','self_transcendence'],ARRAY['meaning_clarification','responsibility_clarification','attitudinal_work'],ARRAY['differentiated_responsibility','meaningful_action'],ARRAY['No impone un sentido prefabricado.'],ARRAY['Evitar moralizar culpa o sufrimiento.']),
('systemic',1,'Sistémico','Abordaje de patrones relacionales, circularidad, reglas, roles, feedback, alianzas y contexto.',ARRAY['interpersonal_pattern','environmental_context'],ARRAY['circularity','relational_rules','roles','feedback','interactional_maintenance'],ARRAY['pattern_mapping','relational_reframing','interaction_experiment'],ARRAY['changed_interaction','new_feedback_pattern'],ARRAY['No reduce a la persona a su sistema.'],ARRAY['Considerar poder, seguridad y contexto sociocultural.']),
('gestalt_phenomenological',1,'Gestalt / Fenomenológico','Exploración descriptiva de experiencia presente, contacto, awareness, cuerpo y polaridades.',ARRAY['emotional_regulation','interpersonal_pattern','meaning_value_conflict'],ARRAY['present_experience','contact','awareness','body_experience','polarities'],ARRAY['phenomenological_exploration','awareness_experiment','chair_work_family'],ARRAY['increased_awareness','contact_quality'],ARRAY['No interpreta experiencia como hecho sin contraste.'],ARRAY['Graduar experimentos y respetar límites.']),
('motivational_interviewing',1,'Entrevista Motivacional','Estilo colaborativo para explorar ambivalencia, change talk, sustain talk, readiness, autonomía y discrepancia.',ARRAY['behavioral_pattern','meaning_value_conflict'],ARRAY['ambivalence','change_talk','sustain_talk','readiness','autonomy','discrepancy'],ARRAY['evocation','decisional_balance_family','readiness_exploration'],ARRAY['change_talk','autonomous_commitment'],ARRAY['No sustituye intervención específica cuando ya existe decisión y capacidad.'],ARRAY['Evitar persuasión encubierta.']);

INSERT INTO therapeutic_technique_definitions(slug,version,name,approach_slug,approach_version,description,target_domains,mechanism,indications,cautions,limits,expected_signals) VALUES
('behavioral_experiment',1,'Experimento conductual','cbt',1,'Familia de pruebas colaborativas de predicciones mediante conducta observable.',ARRAY['behavioral_pattern','cognitive_pattern'],'Contrasta predicciones y ciclos de mantenimiento con experiencia planificada.',ARRAY['predicción identificable','condiciones seguras'],ARRAY['No diseñar como prueba de obediencia.'],ARRAY['No demuestra universalmente una creencia.'],ARRAY['nueva evidencia experiencial','revisión de predicción']),
('graded_exposure',1,'Exposición graduada','cbt',1,'Acercamiento gradual y planificado a situaciones evitadas cuando la evitación mantiene el problema.',ARRAY['experiential_avoidance','behavioral_pattern'],'Reduce mantenimiento por evitación y amplía aprendizaje de seguridad/tolerancia.',ARRAY['evitación funcionalmente evaluada'],ARRAY['Evaluar riesgo real y consentimiento.'],ARRAY['No usar ante peligro actual.'],ARRAY['mayor aproximación','menor conducta de seguridad']),
('values_committed_action',1,'Acción comprometida guiada por valores','act',1,'Familia de acciones pequeñas elegidas en dirección a valores.',ARRAY['meaning_value_conflict','experiential_avoidance'],'Aumenta conducta flexible aun con malestar presente.',ARRAY['valor elegido','acción posible'],ARRAY['No imponer valores.'],ARRAY['No equivale a eliminar culpa o ansiedad.'],ARRAY['acción consistente con valores']),
('defusion_practice',1,'Práctica de defusión','act',1,'Ejercicios para modificar la relación con pensamientos sin exigir su desaparición.',ARRAY['cognitive_pattern','experiential_avoidance'],'Reduce literalidad y dominio conductual de reglas internas.',ARRAY['fusión identificada'],ARRAY['Adaptar lenguaje y experiencia.'],ARRAY['No invalida contenido factual.'],ARRAY['mayor distancia funcional']),
('responsibility_clarification',1,'Clarificación de responsabilidad y control','logotherapy',1,'Explora libertad situada, responsabilidad y límites de control.',ARRAY['meaning_value_conflict','cognitive_pattern'],'Diferencia responsabilidad posible de omnipotencia y culpa totalizante.',ARRAY['confusión responsabilidad-control'],ARRAY['Evitar moralización.'],ARRAY['No prescribe sentido.'],ARRAY['responsabilidad diferenciada']),
('interaction_pattern_mapping',1,'Mapeo de patrón interactivo','systemic',1,'Describe secuencias circulares y feedback sin asignar causalidad lineal simplista.',ARRAY['interpersonal_pattern','environmental_context'],'Hace visibles contingencias relacionales mantenedoras.',ARRAY['secuencia relacional observable'],ARRAY['Considerar asimetrías de poder.'],ARRAY['No atribuye culpa equivalente.'],ARRAY['descripción circular compartida']),
('phenomenological_exploration',1,'Exploración fenomenológica','gestalt_phenomenological',1,'Atención descriptiva a experiencia presente, cuerpo y contacto.',ARRAY['emotional_regulation','interpersonal_pattern'],'Amplía awareness y discriminación experiencial.',ARRAY['capacidad de observación presente'],ARRAY['Graduar intensidad.'],ARRAY['No convierte sensación en interpretación factual.'],ARRAY['mayor discriminación experiencial']),
('ambivalence_exploration',1,'Exploración de ambivalencia','motivational_interviewing',1,'Evoca argumentos propios de cambio y mantenimiento respetando autonomía.',ARRAY['behavioral_pattern','meaning_value_conflict'],'Clarifica ambivalencia y disposición sin persuasión directiva.',ARRAY['ambivalencia relevante'],ARRAY['Evitar confrontación argumentativa.'],ARRAY['No sustituye decisión informada.'],ARRAY['change talk','decisión autónoma']);

CREATE TABLE clinical_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, client_id UUID NOT NULL, process_id UUID NOT NULL,
    title TEXT NOT NULL CHECK(length(btrim(title))>0), description TEXT NOT NULL CHECK(length(btrim(description))>0),
    target_type TEXT NOT NULL CHECK(target_type IN('behavioral_pattern','cognitive_pattern','emotional_regulation','experiential_avoidance','interpersonal_pattern','meaning_value_conflict','skill_deficit','environmental_context','other')),
    approval_status TEXT NOT NULL DEFAULT 'proposed' CHECK(approval_status IN('proposed','approved','rejected')),
    clinical_status TEXT NOT NULL DEFAULT 'active' CHECK(clinical_status IN('active','monitoring','resolved','retired')),
    version INTEGER NOT NULL DEFAULT 1 CHECK(version>0), created_by_user_id UUID, created_from_ai_run_id UUID, approved_by_user_id UUID, approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_targets_approval_check CHECK((approval_status='proposed' AND approved_by_user_id IS NULL AND approved_at IS NULL) OR (approval_status='approved' AND approved_by_user_id IS NOT NULL AND approved_at IS NOT NULL) OR (approval_status='rejected' AND approved_at IS NOT NULL)),
    CONSTRAINT clinical_targets_provenance_check CHECK(created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL),
    CONSTRAINT clinical_targets_client_fkey FOREIGN KEY(tenant_id,client_id) REFERENCES clients(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_targets_process_fkey FOREIGN KEY(tenant_id,process_id,client_id) REFERENCES clinical_processes(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_targets_creator_fkey FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_targets_approver_fkey FOREIGN KEY(tenant_id,approved_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    CONSTRAINT clinical_targets_run_fkey FOREIGN KEY(tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,
    CONSTRAINT clinical_targets_tenant_id_client_unique UNIQUE(tenant_id,id,client_id), CONSTRAINT clinical_targets_tenant_id_unique UNIQUE(tenant_id,id)
);
CREATE INDEX clinical_targets_client_process ON clinical_targets(tenant_id,client_id,process_id,approval_status,clinical_status,updated_at DESC,id);
CREATE TRIGGER clinical_targets_no_delete BEFORE DELETE ON clinical_targets FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();

CREATE TABLE clinical_target_evidence(tenant_id UUID NOT NULL,client_id UUID NOT NULL,target_id UUID NOT NULL,evidence_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,target_id,evidence_id),FOREIGN KEY(tenant_id,target_id,client_id) REFERENCES clinical_targets(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,evidence_id,client_id) REFERENCES clinical_evidence(tenant_id,id,client_id) ON DELETE RESTRICT);
CREATE TABLE clinical_target_hypotheses(tenant_id UUID NOT NULL,client_id UUID NOT NULL,target_id UUID NOT NULL,hypothesis_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,target_id,hypothesis_id),FOREIGN KEY(tenant_id,target_id,client_id) REFERENCES clinical_targets(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,hypothesis_id,client_id) REFERENCES clinical_hypotheses(tenant_id,id,client_id) ON DELETE RESTRICT);
CREATE TABLE clinical_target_events(tenant_id UUID NOT NULL,client_id UUID NOT NULL,target_id UUID NOT NULL,event_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,target_id,event_id),FOREIGN KEY(tenant_id,target_id,client_id) REFERENCES clinical_targets(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,event_id,client_id) REFERENCES clinical_events(tenant_id,id,client_id) ON DELETE RESTRICT);

CREATE OR REPLACE FUNCTION ensure_approved_target_grounding() RETURNS trigger AS $$
DECLARE t UUID; i UUID; a TEXT; BEGIN t:=COALESCE(NEW.tenant_id,OLD.tenant_id); IF TG_TABLE_NAME='clinical_targets' THEN i:=COALESCE(NEW.id,OLD.id); ELSE i:=COALESCE(NEW.target_id,OLD.target_id); END IF;
SELECT approval_status INTO a FROM clinical_targets WHERE tenant_id=t AND id=i;
IF a='approved' AND NOT EXISTS(SELECT 1 FROM clinical_target_evidence WHERE tenant_id=t AND target_id=i) AND NOT EXISTS(SELECT 1 FROM clinical_target_hypotheses WHERE tenant_id=t AND target_id=i) AND NOT EXISTS(SELECT 1 FROM clinical_target_events WHERE tenant_id=t AND target_id=i) THEN RAISE EXCEPTION 'approved target requires clinical grounding' USING ERRCODE='23514'; END IF; RETURN COALESCE(NEW,OLD); END; $$ LANGUAGE plpgsql;
CREATE CONSTRAINT TRIGGER clinical_targets_require_grounding AFTER INSERT OR UPDATE OF approval_status ON clinical_targets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_approved_target_grounding();
CREATE CONSTRAINT TRIGGER clinical_target_evidence_preserve_grounding AFTER DELETE ON clinical_target_evidence DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_approved_target_grounding();
CREATE CONSTRAINT TRIGGER clinical_target_hypotheses_preserve_grounding AFTER DELETE ON clinical_target_hypotheses DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_approved_target_grounding();
CREATE CONSTRAINT TRIGGER clinical_target_events_preserve_grounding AFTER DELETE ON clinical_target_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_approved_target_grounding();

CREATE TABLE clinical_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),tenant_id UUID NOT NULL,client_id UUID NOT NULL,process_id UUID NOT NULL,
    title TEXT NOT NULL CHECK(length(btrim(title))>0),description TEXT NOT NULL CHECK(length(btrim(description))>0),
    goal_type TEXT NOT NULL CHECK(goal_type IN('understanding','behavior_change','skill_acquisition','emotional_regulation','meaning_reconstruction','relationship_change','maintenance','relapse_prevention','other')),
    priority TEXT NOT NULL CHECK(priority IN('low','medium','high')),
    approval_status TEXT NOT NULL DEFAULT 'proposed' CHECK(approval_status IN('proposed','approved','rejected')),
    clinical_status TEXT NOT NULL DEFAULT 'planned' CHECK(clinical_status IN('planned','active','progressing','achieved','paused','abandoned')),
    version INTEGER NOT NULL DEFAULT 1 CHECK(version>0),created_by_user_id UUID,created_from_ai_run_id UUID,approved_by_user_id UUID,approved_at TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,achieved_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clinical_goals_approval_check CHECK((approval_status='proposed' AND approved_by_user_id IS NULL AND approved_at IS NULL) OR (approval_status='approved' AND approved_by_user_id IS NOT NULL AND approved_at IS NOT NULL) OR (approval_status='rejected' AND approved_at IS NOT NULL)),
    CONSTRAINT clinical_goals_provenance_check CHECK(created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL),
    FOREIGN KEY(tenant_id,client_id) REFERENCES clients(tenant_id,id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,process_id,client_id) REFERENCES clinical_processes(tenant_id,id,client_id) ON DELETE RESTRICT,
    FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,approved_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    FOREIGN KEY(tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,
    UNIQUE(tenant_id,id,client_id),UNIQUE(tenant_id,id)
);
CREATE INDEX clinical_goals_client_process ON clinical_goals(tenant_id,client_id,process_id,approval_status,clinical_status,priority,id);
CREATE TRIGGER clinical_goals_no_delete BEFORE DELETE ON clinical_goals FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();
CREATE TABLE clinical_goal_targets(tenant_id UUID NOT NULL,client_id UUID NOT NULL,goal_id UUID NOT NULL,target_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,goal_id,target_id),FOREIGN KEY(tenant_id,goal_id,client_id) REFERENCES clinical_goals(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,target_id,client_id) REFERENCES clinical_targets(tenant_id,id,client_id) ON DELETE RESTRICT);
CREATE OR REPLACE FUNCTION ensure_approved_goal_target() RETURNS trigger AS $$ DECLARE t UUID;i UUID;a TEXT;BEGIN t:=COALESCE(NEW.tenant_id,OLD.tenant_id);IF TG_TABLE_NAME='clinical_goals' THEN i:=COALESCE(NEW.id,OLD.id);ELSE i:=COALESCE(NEW.goal_id,OLD.goal_id);END IF;SELECT approval_status INTO a FROM clinical_goals WHERE tenant_id=t AND id=i;IF a='approved' AND NOT EXISTS(SELECT 1 FROM clinical_goal_targets WHERE tenant_id=t AND goal_id=i) THEN RAISE EXCEPTION 'approved goal requires target' USING ERRCODE='23514';END IF;RETURN COALESCE(NEW,OLD);END;$$ LANGUAGE plpgsql;
CREATE CONSTRAINT TRIGGER clinical_goals_require_target AFTER INSERT OR UPDATE OF approval_status ON clinical_goals DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_approved_goal_target();
CREATE CONSTRAINT TRIGGER clinical_goal_targets_preserve_requirement AFTER DELETE ON clinical_goal_targets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_approved_goal_target();

CREATE TABLE clinical_goal_indicators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),tenant_id UUID NOT NULL,client_id UUID NOT NULL,goal_id UUID NOT NULL,
    description TEXT NOT NULL CHECK(length(btrim(description))>0),indicator_type TEXT NOT NULL CHECK(indicator_type IN('qualitative','behavioral','self_report','frequency','scale','other')),
    measurement_method TEXT,baseline TEXT,target_value TEXT,status TEXT NOT NULL DEFAULT 'active' CHECK(status IN('active','inactive','retired')),
    version INTEGER NOT NULL DEFAULT 1 CHECK(version>0),created_by_user_id UUID,created_from_ai_run_id UUID,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY(tenant_id,goal_id,client_id) REFERENCES clinical_goals(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    FOREIGN KEY(tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,UNIQUE(tenant_id,id,client_id),UNIQUE(tenant_id,id),
    CHECK(created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL)
);
CREATE INDEX clinical_goal_indicators_goal ON clinical_goal_indicators(tenant_id,client_id,goal_id,status,id);
CREATE TRIGGER clinical_goal_indicators_no_delete BEFORE DELETE ON clinical_goal_indicators FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();
CREATE TABLE clinical_indicator_evidence(tenant_id UUID NOT NULL,client_id UUID NOT NULL,indicator_id UUID NOT NULL,evidence_id UUID NOT NULL,relation_type TEXT NOT NULL CHECK(relation_type IN('supports_progress','supports_regression','neutral')),created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,indicator_id,evidence_id),FOREIGN KEY(tenant_id,indicator_id,client_id) REFERENCES clinical_goal_indicators(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,evidence_id,client_id) REFERENCES clinical_evidence(tenant_id,id,client_id) ON DELETE RESTRICT);
CREATE TABLE clinical_indicator_events(tenant_id UUID NOT NULL,client_id UUID NOT NULL,indicator_id UUID NOT NULL,event_id UUID NOT NULL,relation_type TEXT NOT NULL CHECK(relation_type IN('supports_progress','supports_regression','neutral')),created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,indicator_id,event_id),FOREIGN KEY(tenant_id,indicator_id,client_id) REFERENCES clinical_goal_indicators(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,event_id,client_id) REFERENCES clinical_events(tenant_id,id,client_id) ON DELETE RESTRICT);

CREATE TABLE therapeutic_rationales (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),tenant_id UUID NOT NULL,client_id UUID NOT NULL,process_id UUID NOT NULL,target_id UUID NOT NULL,goal_id UUID NOT NULL,
    approach_slug TEXT NOT NULL,approach_version INTEGER NOT NULL,technique_slug TEXT,technique_version INTEGER,
    rationale TEXT NOT NULL CHECK(length(btrim(rationale))>0),expected_effect TEXT NOT NULL CHECK(length(btrim(expected_effect))>0),
    grounding_status TEXT NOT NULL CHECK(grounding_status IN('grounded','insufficient_evidence','exploration_needed')),
    approval_status TEXT NOT NULL DEFAULT 'proposed' CHECK(approval_status IN('proposed','approved','rejected')),version INTEGER NOT NULL DEFAULT 1 CHECK(version>0),
    created_by_user_id UUID,created_from_ai_run_id UUID,approved_by_user_id UUID,approved_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK((technique_slug IS NULL AND technique_version IS NULL) OR (technique_slug IS NOT NULL AND technique_version IS NOT NULL)),
    CHECK(approval_status<>'approved' OR grounding_status='grounded'),
    CHECK((approval_status='proposed' AND approved_by_user_id IS NULL AND approved_at IS NULL) OR (approval_status='approved' AND approved_by_user_id IS NOT NULL AND approved_at IS NOT NULL) OR (approval_status='rejected' AND approved_at IS NOT NULL)),
    CHECK(created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL),
    FOREIGN KEY(tenant_id,process_id,client_id) REFERENCES clinical_processes(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,target_id,client_id) REFERENCES clinical_targets(tenant_id,id,client_id) ON DELETE RESTRICT,
    FOREIGN KEY(tenant_id,goal_id,client_id) REFERENCES clinical_goals(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(approach_slug,approach_version) REFERENCES therapeutic_approach_definitions(slug,version) ON DELETE RESTRICT,
    FOREIGN KEY(technique_slug,technique_version,approach_slug,approach_version) REFERENCES therapeutic_technique_definitions(slug,version,approach_slug,approach_version) ON DELETE RESTRICT,
    FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,approved_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    FOREIGN KEY(tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,UNIQUE(tenant_id,id,client_id),UNIQUE(tenant_id,id)
);
CREATE INDEX therapeutic_rationales_chain ON therapeutic_rationales(tenant_id,client_id,process_id,target_id,goal_id,approval_status,approach_slug,approach_version,id);
CREATE TRIGGER therapeutic_rationales_no_delete BEFORE DELETE ON therapeutic_rationales FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();
CREATE TABLE therapeutic_rationale_evidence(tenant_id UUID NOT NULL,client_id UUID NOT NULL,rationale_id UUID NOT NULL,evidence_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,rationale_id,evidence_id),FOREIGN KEY(tenant_id,rationale_id,client_id) REFERENCES therapeutic_rationales(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,evidence_id,client_id) REFERENCES clinical_evidence(tenant_id,id,client_id) ON DELETE RESTRICT);
CREATE TABLE therapeutic_rationale_hypotheses(tenant_id UUID NOT NULL,client_id UUID NOT NULL,rationale_id UUID NOT NULL,hypothesis_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,rationale_id,hypothesis_id),FOREIGN KEY(tenant_id,rationale_id,client_id) REFERENCES therapeutic_rationales(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,hypothesis_id,client_id) REFERENCES clinical_hypotheses(tenant_id,id,client_id) ON DELETE RESTRICT);

CREATE TABLE giras (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),tenant_id UUID NOT NULL,client_id UUID NOT NULL,process_id UUID NOT NULL,gira_version INTEGER NOT NULL CHECK(gira_version>0),
    title TEXT NOT NULL CHECK(length(btrim(title))>0),summary TEXT NOT NULL CHECK(length(btrim(summary))>0),approval_status TEXT NOT NULL DEFAULT 'proposed' CHECK(approval_status IN('proposed','approved','rejected')),
    clinical_status TEXT NOT NULL DEFAULT 'planned' CHECK(clinical_status IN('planned','active','paused','completed','superseded')),entity_version INTEGER NOT NULL DEFAULT 1 CHECK(entity_version>0),
    created_by_user_id UUID,created_from_ai_run_id UUID,approved_by_user_id UUID,approved_at TIMESTAMPTZ,activated_at TIMESTAMPTZ,completed_at TIMESTAMPTZ,supersedes_gira_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK((approval_status='proposed' AND approved_by_user_id IS NULL AND approved_at IS NULL) OR (approval_status='approved' AND approved_by_user_id IS NOT NULL AND approved_at IS NOT NULL) OR (approval_status='rejected' AND approved_at IS NOT NULL)),
    CHECK(created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL),
    FOREIGN KEY(tenant_id,process_id,client_id) REFERENCES clinical_processes(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,supersedes_gira_id,client_id) REFERENCES giras(tenant_id,id,client_id) ON DELETE RESTRICT,
    FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,approved_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,
    FOREIGN KEY(tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,
    UNIQUE(tenant_id,id,client_id),UNIQUE(tenant_id,id),UNIQUE(tenant_id,process_id,gira_version)
);
CREATE UNIQUE INDEX giras_one_active_process ON giras(tenant_id,client_id,process_id) WHERE approval_status='approved' AND clinical_status='active';
CREATE INDEX giras_client_process ON giras(tenant_id,client_id,process_id,gira_version DESC,id);
CREATE TRIGGER giras_no_delete BEFORE DELETE ON giras FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();
CREATE TABLE gira_targets(tenant_id UUID NOT NULL,client_id UUID NOT NULL,gira_id UUID NOT NULL,target_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,gira_id,target_id),FOREIGN KEY(tenant_id,gira_id,client_id) REFERENCES giras(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,target_id,client_id) REFERENCES clinical_targets(tenant_id,id,client_id) ON DELETE RESTRICT);
CREATE TABLE gira_goals(tenant_id UUID NOT NULL,client_id UUID NOT NULL,gira_id UUID NOT NULL,goal_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,gira_id,goal_id),FOREIGN KEY(tenant_id,gira_id,client_id) REFERENCES giras(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,goal_id,client_id) REFERENCES clinical_goals(tenant_id,id,client_id) ON DELETE RESTRICT);
CREATE TABLE gira_rationales(tenant_id UUID NOT NULL,client_id UUID NOT NULL,gira_id UUID NOT NULL,rationale_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,gira_id,rationale_id),FOREIGN KEY(tenant_id,gira_id,client_id) REFERENCES giras(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,rationale_id,client_id) REFERENCES therapeutic_rationales(tenant_id,id,client_id) ON DELETE RESTRICT);

CREATE TABLE gira_phases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),tenant_id UUID NOT NULL,client_id UUID NOT NULL,gira_id UUID NOT NULL,position INTEGER NOT NULL CHECK(position>0),
    title TEXT NOT NULL CHECK(length(btrim(title))>0),description TEXT NOT NULL CHECK(length(btrim(description))>0),clinical_status TEXT NOT NULL DEFAULT 'planned' CHECK(clinical_status IN('planned','active','completed','paused','skipped')),
    entry_criteria TEXT,exit_criteria TEXT,version INTEGER NOT NULL DEFAULT 1 CHECK(version>0),created_by_user_id UUID,created_from_ai_run_id UUID,activated_at TIMESTAMPTZ,completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK(created_by_user_id IS NOT NULL OR created_from_ai_run_id IS NOT NULL),FOREIGN KEY(tenant_id,gira_id,client_id) REFERENCES giras(tenant_id,id,client_id) ON DELETE RESTRICT,
    FOREIGN KEY(tenant_id,created_by_user_id) REFERENCES users(tenant_id,id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,created_from_ai_run_id,client_id) REFERENCES clinical_ai_runs(tenant_id,id,client_id) ON DELETE RESTRICT,
    UNIQUE(tenant_id,id,client_id),UNIQUE(tenant_id,id),UNIQUE(tenant_id,gira_id,position)
);
CREATE INDEX gira_phases_order ON gira_phases(tenant_id,client_id,gira_id,position,id);
CREATE TRIGGER gira_phases_no_delete BEFORE DELETE ON gira_phases FOR EACH ROW EXECUTE FUNCTION prevent_clinical_longitudinal_delete();
CREATE TABLE gira_phase_goals(tenant_id UUID NOT NULL,client_id UUID NOT NULL,phase_id UUID NOT NULL,goal_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,phase_id,goal_id),FOREIGN KEY(tenant_id,phase_id,client_id) REFERENCES gira_phases(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,goal_id,client_id) REFERENCES clinical_goals(tenant_id,id,client_id) ON DELETE RESTRICT);
CREATE TABLE gira_phase_rationales(tenant_id UUID NOT NULL,client_id UUID NOT NULL,phase_id UUID NOT NULL,rationale_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,phase_id,rationale_id),FOREIGN KEY(tenant_id,phase_id,client_id) REFERENCES gira_phases(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,rationale_id,client_id) REFERENCES therapeutic_rationales(tenant_id,id,client_id) ON DELETE RESTRICT);
CREATE TABLE gira_phase_indicators(tenant_id UUID NOT NULL,client_id UUID NOT NULL,phase_id UUID NOT NULL,indicator_id UUID NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(tenant_id,phase_id,indicator_id),FOREIGN KEY(tenant_id,phase_id,client_id) REFERENCES gira_phases(tenant_id,id,client_id) ON DELETE RESTRICT,FOREIGN KEY(tenant_id,indicator_id,client_id) REFERENCES clinical_goal_indicators(tenant_id,id,client_id) ON DELETE RESTRICT);

ALTER TABLE clinical_diff_operations DROP CONSTRAINT clinical_diff_operations_operation_type_check;
ALTER TABLE clinical_diff_operations ADD CONSTRAINT clinical_diff_operations_operation_type_check CHECK(operation_type IN(
'create_evidence','invalidate_evidence','create_event','approve_event','link_event_process','create_process','update_process','close_process','reopen_process','create_hypothesis','update_hypothesis','link_supporting_evidence','link_contradicting_evidence','strengthen_hypothesis','weaken_hypothesis','retire_hypothesis',
'create_target','update_target','resolve_target','retire_target','create_goal','update_goal','activate_goal','pause_goal','achieve_goal','abandon_goal','create_goal_indicator','update_goal_indicator','link_indicator_evidence','link_indicator_event','create_therapeutic_rationale','update_therapeutic_rationale','create_gira','supersede_gira','activate_gira','pause_gira','complete_gira','create_gira_phase','update_gira_phase','activate_gira_phase','complete_gira_phase','pause_gira_phase'));
ALTER TABLE clinical_longitudinal_transitions DROP CONSTRAINT clinical_longitudinal_transitions_entity_type_check;
ALTER TABLE clinical_longitudinal_transitions ADD CONSTRAINT clinical_longitudinal_transitions_entity_type_check CHECK(entity_type IN('evidence','event','process','hypothesis','target','goal','goal_indicator','therapeutic_rationale','gira','gira_phase'));

ALTER TABLE clinical_ai_run_sources DROP CONSTRAINT clinical_ai_run_sources_source_type_check;
ALTER TABLE clinical_ai_run_sources ADD CONSTRAINT clinical_ai_run_sources_source_type_check CHECK(source_type IN('formulation_snapshot','formulation_anchor','session_report','clinical_evidence','clinical_event','clinical_process','clinical_hypothesis','clinical_target','clinical_goal','goal_indicator','therapeutic_rationale','gira','gira_phase','therapeutic_approach_definition','therapeutic_technique_definition'));
CREATE OR REPLACE FUNCTION validate_clinical_ai_run_source() RETURNS trigger AS $$ DECLARE run_client UUID; BEGIN
SELECT client_id INTO run_client FROM clinical_ai_runs WHERE tenant_id=NEW.tenant_id AND id=NEW.ai_run_id;
IF NEW.source_type='formulation_snapshot' AND NOT EXISTS(SELECT 1 FROM clinical_formulation_snapshots WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND status='approved' AND (NEW.source_version IS NULL OR version=NEW.source_version)) THEN RAISE EXCEPTION 'invalid formulation source' USING ERRCODE='23503';
ELSIF NEW.source_type='formulation_anchor' AND NOT EXISTS(SELECT 1 FROM clinical_formulation_anchors a JOIN clinical_formulation_snapshots s ON s.tenant_id=a.tenant_id AND s.id=a.snapshot_id WHERE a.tenant_id=NEW.tenant_id AND a.id=NEW.source_id AND s.status='approved' AND (NEW.source_version IS NULL OR s.version=NEW.source_version)) THEN RAISE EXCEPTION 'invalid anchor source' USING ERRCODE='23503';
ELSIF NEW.source_type='session_report' AND NOT EXISTS(SELECT 1 FROM session_reports r JOIN clinical_sessions s ON s.tenant_id=r.tenant_id AND s.id=r.clinical_session_id WHERE r.tenant_id=NEW.tenant_id AND r.id=NEW.source_id AND r.status='approved' AND s.client_id=run_client AND (NEW.source_version IS NULL OR r.version=NEW.source_version)) THEN RAISE EXCEPTION 'invalid report source' USING ERRCODE='23503';
ELSIF NEW.source_type='clinical_evidence' AND NOT EXISTS(SELECT 1 FROM clinical_evidence WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND status='active' AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid evidence source' USING ERRCODE='23503';
ELSIF NEW.source_type='clinical_event' AND NOT EXISTS(SELECT 1 FROM clinical_events WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid event source' USING ERRCODE='23503';
ELSIF NEW.source_type='clinical_process' AND NOT EXISTS(SELECT 1 FROM clinical_processes WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid process source' USING ERRCODE='23503';
ELSIF NEW.source_type='clinical_hypothesis' AND NOT EXISTS(SELECT 1 FROM clinical_hypotheses WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid hypothesis source' USING ERRCODE='23503';
ELSIF NEW.source_type='clinical_target' AND NOT EXISTS(SELECT 1 FROM clinical_targets WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid target source' USING ERRCODE='23503';
ELSIF NEW.source_type='clinical_goal' AND NOT EXISTS(SELECT 1 FROM clinical_goals WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid goal source' USING ERRCODE='23503';
ELSIF NEW.source_type='goal_indicator' AND NOT EXISTS(SELECT 1 FROM clinical_goal_indicators WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid indicator source' USING ERRCODE='23503';
ELSIF NEW.source_type='therapeutic_rationale' AND NOT EXISTS(SELECT 1 FROM therapeutic_rationales WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid rationale source' USING ERRCODE='23503';
ELSIF NEW.source_type='gira' AND NOT EXISTS(SELECT 1 FROM giras WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND approval_status='approved' AND entity_version=NEW.source_version) THEN RAISE EXCEPTION 'invalid gira source' USING ERRCODE='23503';
ELSIF NEW.source_type='gira_phase' AND NOT EXISTS(SELECT 1 FROM gira_phases WHERE tenant_id=NEW.tenant_id AND id=NEW.source_id AND client_id=run_client AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid phase source' USING ERRCODE='23503';
ELSIF NEW.source_type='therapeutic_approach_definition' AND NOT EXISTS(SELECT 1 FROM therapeutic_approach_definitions WHERE id=NEW.source_id AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid approach definition source' USING ERRCODE='23503';
ELSIF NEW.source_type='therapeutic_technique_definition' AND NOT EXISTS(SELECT 1 FROM therapeutic_technique_definitions WHERE id=NEW.source_id AND version=NEW.source_version) THEN RAISE EXCEPTION 'invalid technique definition source' USING ERRCODE='23503'; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql;
