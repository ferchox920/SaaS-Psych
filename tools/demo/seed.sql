-- Opt-in, entirely fictional portfolio fixture. Run only against the local
-- sessionflow database after migrations; never include in production migrations.
BEGIN;

DO $$ BEGIN
  IF current_database() <> 'sessionflow' THEN
    RAISE EXCEPTION 'demo seed requires the local sessionflow database';
  END IF;
END $$;

-- The historical migration creates these two users, then migration 000029
-- invalidates their known passwords. Restore them only in this opt-in seed.
INSERT INTO users (id, tenant_id, email, password_hash) VALUES
 ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa','11111111-1111-1111-1111-111111111111','owner@tenant-a.local',crypt('ChangeMe123!',gen_salt('bf'))),
 ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb','22222222-2222-2222-2222-222222222222','member@tenant-b.local',crypt('ChangeMe123!',gen_salt('bf')))
ON CONFLICT (tenant_id,email) DO UPDATE SET password_hash=EXCLUDED.password_hash;

INSERT INTO users (id, tenant_id, email, password_hash) VALUES
 ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab','11111111-1111-1111-1111-111111111111','therapist@tenant-a.local',crypt('ChangeMe123!',gen_salt('bf'))),
 ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaac','11111111-1111-1111-1111-111111111111','unassigned@tenant-a.local',crypt('ChangeMe123!',gen_salt('bf')))
ON CONFLICT (tenant_id,email) DO NOTHING;

INSERT INTO user_roles (id,tenant_id,user_id,role) VALUES
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee1','11111111-1111-1111-1111-111111111111','aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab','member'),
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee2','11111111-1111-1111-1111-111111111111','aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaac','member')
ON CONFLICT (tenant_id,user_id,role) DO NOTHING;

INSERT INTO clients (id,tenant_id,fullname,contact,notes_public) VALUES
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11','11111111-1111-1111-1111-111111111111','Paciente Ficticia Aurora','aurora@example.invalid','Ficha sintética exclusiva de demo.'),
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee22','22222222-2222-2222-2222-222222222222','Paciente Ficticio Río','rio@example.invalid','Ficha sintética de otro tenant.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO client_clinical_assignments (id,tenant_id,client_id,user_id,relationship,granted_by_user_id) VALUES
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee31','11111111-1111-1111-1111-111111111111','eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11','aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa','treating','aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'),
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee32','11111111-1111-1111-1111-111111111111','eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11','aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab','treating','aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'),
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee33','22222222-2222-2222-2222-222222222222','eeeeeeee-eeee-eeee-eeee-eeeeeeeeee22','bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb','treating','bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb')
ON CONFLICT (tenant_id,client_id,user_id,relationship) WHERE ends_at IS NULL DO NOTHING;

INSERT INTO appointments (id,tenant_id,client_id,starts_at,ends_at,status,location) VALUES
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee41','11111111-1111-1111-1111-111111111111','eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11',date_trunc('day',now()) + interval '2 days 10 hours',date_trunc('day',now()) + interval '2 days 11 hours','scheduled','Videollamada ficticia'),
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee42','11111111-1111-1111-1111-111111111111','eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11',date_trunc('week',now()) - interval '5 days' + interval '10 hours',date_trunc('week',now()) - interval '5 days' + interval '11 hours','completed','Consultorio ficticio'),
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee43','22222222-2222-2222-2222-222222222222','eeeeeeee-eeee-eeee-eeee-eeeeeeeeee22',date_trunc('day',now()) + interval '3 days 14 hours',date_trunc('day',now()) + interval '3 days 15 hours','scheduled','Sala ficticia')
ON CONFLICT (id) DO NOTHING;

INSERT INTO clinical_sessions (id,tenant_id,client_id,appointment_id,therapist_user_id,status,started_at,ended_at) VALUES
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee51','11111111-1111-1111-1111-111111111111','eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11','eeeeeeee-eeee-eeee-eeee-eeeeeeeeee42','aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab','completed',date_trunc('week',now()) - interval '5 days' + interval '10 hours',date_trunc('week',now()) - interval '5 days' + interval '11 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO clinical_consent_grants (id,tenant_id,client_id,scope,definition_version,granted_by_user_id) VALUES
 ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeee61','11111111-1111-1111-1111-111111111111','eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11','LOCAL_AI_PROCESSING',1,'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab')
ON CONFLICT (tenant_id,client_id,scope) WHERE status='granted' DO NOTHING;

COMMIT;
