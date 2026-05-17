BEGIN;

DROP TABLE IF EXISTS medula_eraport;
DROP TYPE IF EXISTS medula_eraport_status;
DROP TYPE IF EXISTS medula_eraport_kind;
DROP TABLE IF EXISTS medula_referral;
DROP TYPE IF EXISTS medula_referral_status;
DROP TABLE IF EXISTS medula_invoice_submission;
DROP TYPE IF EXISTS medula_submit_status;

ALTER TABLE medula_provision
    DROP COLUMN IF EXISTS closed_at,
    DROP COLUMN IF EXISTS cancellation_reason,
    DROP COLUMN IF EXISTS cancelled_at;

COMMIT;
