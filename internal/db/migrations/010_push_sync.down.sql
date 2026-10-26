BEGIN;
DROP INDEX IF EXISTS idx_calendars_external_id;
ALTER TABLE calendars DROP COLUMN IF EXISTS external_id;
COMMIT;
