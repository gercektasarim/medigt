-- 019_hakedis: doctor commission rules + computed earnings view.
--
-- A doctor has zero or more commission rules; each rule covers either a
-- specific service_category or all categories (category IS NULL).
-- Earnings are computed on read by joining paid invoice_items to the
-- most specific applicable rule for the doctor.

BEGIN;

CREATE TABLE doctor_commission_rule (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    doctor_id        UUID NOT NULL REFERENCES doctor(id) ON DELETE CASCADE,
    -- NULL = applies to all categories not covered by a more specific rule.
    category         service_category,
    commission_pct   NUMERIC(5,2) NOT NULL CHECK (commission_pct >= 0 AND commission_pct <= 100),
    valid_from       DATE NOT NULL DEFAULT CURRENT_DATE,
    valid_to         DATE,
    notes            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- One active rule per (doctor, category, valid_from) — the valid_to is
    -- managed at write time when a new rule starts.
    UNIQUE (doctor_id, category, valid_from)
);

CREATE INDEX idx_commission_rule_doctor ON doctor_commission_rule(doctor_id, valid_from);

CREATE TRIGGER trg_commission_rule_updated_at BEFORE UPDATE ON doctor_commission_rule
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
