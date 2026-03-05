INSERT INTO tenants (id, name)
VALUES
    ('11111111-1111-1111-1111-111111111111', 'demo-tenant-a'),
    ('22222222-2222-2222-2222-222222222222', 'demo-tenant-b')
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, tenant_id, email, password_hash)
VALUES
    (
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        '11111111-1111-1111-1111-111111111111',
        'owner@tenant-a.local',
        crypt('ChangeMe123!', gen_salt('bf'))
    ),
    (
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        '22222222-2222-2222-2222-222222222222',
        'member@tenant-b.local',
        crypt('ChangeMe123!', gen_salt('bf'))
    )
ON CONFLICT (tenant_id, email) DO NOTHING;

INSERT INTO user_roles (id, tenant_id, user_id, role)
VALUES
    (
        'cccccccc-cccc-cccc-cccc-cccccccccccc',
        '11111111-1111-1111-1111-111111111111',
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        'owner'
    ),
    (
        'dddddddd-dddd-dddd-dddd-dddddddddddd',
        '22222222-2222-2222-2222-222222222222',
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        'member'
    )
ON CONFLICT (tenant_id, user_id, role) DO NOTHING;
