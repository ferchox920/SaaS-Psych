-- Credential invalidation cannot safely be reversed. Reinstalling a known
-- password on rollback would reintroduce the security defect.
SELECT 1;
