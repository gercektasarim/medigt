BEGIN;

DROP TABLE IF EXISTS dialysis_session;
DROP SEQUENCE IF EXISTS dialysis_session_no_seq;
DROP TABLE IF EXISTS dialysis_machine;

DROP TYPE IF EXISTS vascular_access_type;
DROP TYPE IF EXISTS dialysis_modality;
DROP TYPE IF EXISTS dialysis_status;

COMMIT;
