BEGIN;

DROP TABLE IF EXISTS payment_allocation;
DROP TABLE IF EXISTS payment;
DROP SEQUENCE IF EXISTS payment_no_seq;
DROP TABLE IF EXISTS invoice_item;
DROP TABLE IF EXISTS invoice;
DROP SEQUENCE IF EXISTS invoice_no_seq;

DROP TYPE IF EXISTS invoice_status;

COMMIT;
