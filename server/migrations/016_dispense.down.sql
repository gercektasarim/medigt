BEGIN;

DROP TABLE IF EXISTS prescription_dispense;

ALTER TABLE prescription_item
    DROP COLUMN IF EXISTS dispense_quantity,
    DROP COLUMN IF EXISTS medication_id;

COMMIT;
