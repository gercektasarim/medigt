BEGIN;
DROP TRIGGER IF EXISTS trg_service_price_updated_at ON service_price;
DROP TRIGGER IF EXISTS trg_service_catalog_updated_at ON service_catalog;
DROP TABLE IF EXISTS service_price;
DROP TABLE IF EXISTS service_catalog;
DROP TYPE IF EXISTS service_category;
COMMIT;
