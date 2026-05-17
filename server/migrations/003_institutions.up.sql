-- 003_institutions: External institutions (SGK, özel sigortalar, anlaşmalı
-- kurumlar) the hospital bills to or honors referrals from. Hospital-scoped.

BEGIN;

CREATE TABLE external_institution (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    code            TEXT NOT NULL,
    name            TEXT NOT NULL,
    kind            TEXT NOT NULL CHECK (kind IN (
        'sgk',                  -- SGK
        'private_insurance',    -- Özel sağlık sigortası
        'corporate',            -- Kurumsal anlaşma (TSK, Emniyet vs)
        'foreign_insurance',    -- Yabancı sigorta / turist sağlığı
        'oop',                  -- Cepten ödeme (out-of-pocket — kayıt için)
        'other'
    )),
    tax_id          TEXT,
    address         TEXT,
    phone           TEXT,
    email           TEXT,
    contract_no     TEXT,
    contract_starts_at DATE,
    contract_ends_at DATE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, code)
);

CREATE INDEX idx_institution_org ON external_institution(organization_id);
CREATE INDEX idx_institution_active ON external_institution(organization_id, is_active);

CREATE TRIGGER trg_institution_updated_at BEFORE UPDATE ON external_institution
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
