-- 014_medication: medication catalog (organization-scoped trade names with
-- ATC + form + strength + Turkish prescription class).
--
-- Each organization keeps its own catalog because hospital pharmacies build
-- around what they stock. ATC code is the cross-reference back to the
-- WHO classification.

BEGIN;

CREATE TYPE medication_form AS ENUM (
    'tablet',
    'capsule',
    'syrup',
    'injection',
    'ampoule',
    'cream',
    'ointment',
    'drops',
    'spray',
    'patch',
    'suppository',
    'solution',
    'powder',
    'other'
);

-- Türkiye reçete sınıfları
CREATE TYPE prescription_class AS ENUM (
    'otc',              -- reçetesiz
    'normal',           -- beyaz reçete (normal)
    'green',            -- yeşil reçete (psikotrop)
    'red',              -- kırmızı reçete (narkotik)
    'orange',           -- turuncu reçete (özel)
    'purple'            -- mor reçete (anabolik)
);

CREATE TABLE medication (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,

    -- Identifiers
    atc_code        TEXT,                -- ör. N02BE01 (parasetamol)
    barcode         TEXT,                -- GTIN / karekod base
    name            TEXT NOT NULL,       -- ticari ad ör. "Parol 500 mg tablet"
    generic_name    TEXT,                -- ör. "Parasetamol"

    -- Form & strength
    form            medication_form NOT NULL DEFAULT 'tablet',
    strength        TEXT,                -- "500 mg" / "10 mg/ml"
    pack_size       TEXT,                -- "20 tablet" / "100 ml şişe"

    -- Classification
    prescription_class prescription_class NOT NULL DEFAULT 'normal',
    requires_cold_chain BOOLEAN NOT NULL DEFAULT FALSE,
    is_controlled   BOOLEAN NOT NULL DEFAULT FALSE,  -- kontrole tabi

    -- Vendor
    manufacturer    TEXT,

    -- Pricing reference (catalog price; per-institution pricing comes later)
    list_price      NUMERIC(12,2),       -- liste fiyatı

    notes           TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, name)
);

CREATE INDEX idx_medication_org_name ON medication(organization_id, name);
CREATE INDEX idx_medication_atc ON medication(organization_id, atc_code) WHERE atc_code IS NOT NULL;
CREATE INDEX idx_medication_barcode ON medication(organization_id, barcode) WHERE barcode IS NOT NULL;
CREATE INDEX idx_medication_active ON medication(organization_id) WHERE is_active = TRUE;

CREATE TRIGGER trg_medication_updated_at BEFORE UPDATE ON medication
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
