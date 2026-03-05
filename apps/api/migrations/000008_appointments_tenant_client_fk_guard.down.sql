-- No-op down migration.
-- This guard migration only enforces the same tenant/client FK state already established in previous migrations.
SELECT 1;
