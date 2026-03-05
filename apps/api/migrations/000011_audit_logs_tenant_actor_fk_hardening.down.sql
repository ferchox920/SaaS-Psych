-- No-op down migration.
-- 000011 is forward-only hardening to enforce tenant-safe audit actor references.
SELECT 1;
