BEGIN;

DROP TABLE IF EXISTS medication_administration;
DROP TABLE IF EXISTS medication_order;
DROP SEQUENCE IF EXISTS medication_order_no_seq;

DROP TYPE IF EXISTS administration_status;
DROP TYPE IF EXISTS medication_route;
DROP TYPE IF EXISTS medication_order_status;

COMMIT;
