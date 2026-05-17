BEGIN;
DROP TRIGGER IF EXISTS trg_institution_updated_at ON external_institution;
DROP TABLE IF EXISTS external_institution;
COMMIT;
