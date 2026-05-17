BEGIN;

DROP TABLE IF EXISTS cash_movement;
DROP SEQUENCE IF EXISTS cash_movement_no_seq;
DROP TABLE IF EXISTS cash_register;
DROP SEQUENCE IF EXISTS cash_register_no_seq;

DROP TYPE IF EXISTS cash_movement_kind;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS cash_register_status;

COMMIT;
