-- No-op down migration.
-- 000010 is forward-only hardening to enforce tenant-safe user_roles references.
SELECT 1;
