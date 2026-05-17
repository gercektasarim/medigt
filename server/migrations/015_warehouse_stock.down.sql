BEGIN;

DROP TABLE IF EXISTS stock_movement;
DROP SEQUENCE IF EXISTS stock_movement_no_seq;
DROP TABLE IF EXISTS medication_stock;
DROP TABLE IF EXISTS warehouse;

DROP TYPE IF EXISTS stock_movement_kind;
DROP TYPE IF EXISTS warehouse_kind;

COMMIT;
