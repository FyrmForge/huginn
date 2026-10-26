BEGIN;
ALTER TABLE calendars ADD COLUMN external_id TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_calendars_external_id ON calendars(owner_id, external_id)
    WHERE external_id <> '' AND deleted_at IS NULL;
COMMIT;
