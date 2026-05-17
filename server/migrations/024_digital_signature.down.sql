BEGIN;

ALTER TABLE prescription DROP COLUMN IF EXISTS digital_signature_id;

DROP TABLE IF EXISTS digital_signature;
DROP TYPE IF EXISTS signature_status;
DROP TYPE IF EXISTS signature_provider;

COMMIT;
