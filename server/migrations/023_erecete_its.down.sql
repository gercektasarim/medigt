BEGIN;

ALTER TABLE prescription_dispense
    DROP COLUMN IF EXISTS karekod,
    DROP COLUMN IF EXISTS its_error,
    DROP COLUMN IF EXISTS its_response,
    DROP COLUMN IF EXISTS its_notified_at,
    DROP COLUMN IF EXISTS its_status;
DROP TYPE IF EXISTS its_notify_status;

ALTER TABLE prescription
    DROP COLUMN IF EXISTS e_prescription_error,
    DROP COLUMN IF EXISTS e_prescription_response,
    DROP COLUMN IF EXISTS e_prescription_submitted_at,
    DROP COLUMN IF EXISTS e_prescription_status;
DROP TYPE IF EXISTS e_prescription_status;

COMMIT;
