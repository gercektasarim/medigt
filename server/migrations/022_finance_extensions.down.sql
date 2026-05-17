BEGIN;

DROP TABLE IF EXISTS installment;
DROP TYPE IF EXISTS installment_status;
DROP TABLE IF EXISTS installment_plan;
DROP TYPE IF EXISTS installment_plan_status;
DROP TABLE IF EXISTS refund;
DROP SEQUENCE IF EXISTS refund_no_seq;
DROP TABLE IF EXISTS patient_account_entry;
DROP TYPE IF EXISTS patient_account_entry_kind;

COMMIT;
