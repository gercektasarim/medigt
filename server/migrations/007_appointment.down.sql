BEGIN;
DROP TRIGGER IF EXISTS trg_appointment_updated_at ON appointment;
DROP TABLE IF EXISTS appointment;
DROP TYPE IF EXISTS visit_kind;
DROP TYPE IF EXISTS appointment_status;
COMMIT;
