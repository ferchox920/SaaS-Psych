-- Migration 000002 historically installed known demo passwords in every
-- environment. Invalidate only those untouched passwords, preserving any
-- account whose password was changed by an operator.
WITH rotated AS (
    UPDATE users
       SET password_hash = crypt(gen_random_uuid()::text, gen_salt('bf')),
           updated_at = NOW()
     WHERE (id, tenant_id, email) IN (
         ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid, '11111111-1111-1111-1111-111111111111'::uuid, 'owner@tenant-a.local'),
         ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid, '22222222-2222-2222-2222-222222222222'::uuid, 'member@tenant-b.local')
     )
       AND password_hash = crypt('ChangeMe123!', password_hash)
     RETURNING tenant_id, id
)
UPDATE refresh_tokens rt
   SET revoked_at = NOW()
  FROM rotated r
 WHERE rt.tenant_id = r.tenant_id
   AND rt.user_id = r.id
   AND rt.revoked_at IS NULL;
