-- 004_service_catalog: hospital service catalog (procedures, exams, items
-- that can appear on an invoice) + per-institution price book.

BEGIN;

-- ---------- Hizmet kategori (consultation / lab / imaging / surgery / ...) ----------

CREATE TYPE service_category AS ENUM (
    'consultation',     -- Muayene
    'lab',              -- Laboratuvar tetkikleri
    'imaging',          -- Radyoloji / görüntüleme
    'procedure',        -- Girişimsel işlem
    'surgery',          -- Ameliyat
    'inpatient',        -- Yatış / yatak hizmeti
    'medication',       -- İlaç (sarf değil — eczane çıkışı)
    'supply',           -- Sarf malzeme
    'package',          -- Paket hizmet (örn. check-up paketi)
    'other'
);

-- ---------- Hizmet katalogu ----------

CREATE TABLE service_catalog (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    code            TEXT NOT NULL,              -- internal kısa kod
    sut_code        TEXT,                       -- SGK SUT kodu (örn. 520.011)
    name            TEXT NOT NULL,
    category        service_category NOT NULL,
    description     TEXT,
    unit            TEXT NOT NULL DEFAULT 'adet', -- adet, seans, gün, kutu, vs
    vat_rate        NUMERIC(5,2) NOT NULL DEFAULT 10.00, -- KDV %
    base_price      NUMERIC(12,2),              -- "etiket" fiyat; institution bazlı override service_price'ta
    requires_doctor BOOLEAN NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, code)
);

CREATE INDEX idx_service_org ON service_catalog(organization_id);
CREATE INDEX idx_service_active ON service_catalog(organization_id, is_active);
CREATE INDEX idx_service_category ON service_catalog(organization_id, category);
CREATE INDEX idx_service_sut ON service_catalog(sut_code) WHERE sut_code IS NOT NULL;

CREATE TRIGGER trg_service_catalog_updated_at BEFORE UPDATE ON service_catalog
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Kurum bazlı fiyat ----------
-- Bir hizmet birden fazla kuruma satılır; her kurum için farklı fiyat olabilir.
-- Tarih aralığı destekli — SUT güncellemeleri için yeni satır eklenir, eskisi kapatılır.

CREATE TABLE service_price (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id          UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    service_catalog_id       UUID NOT NULL REFERENCES service_catalog(id) ON DELETE CASCADE,
    external_institution_id  UUID REFERENCES external_institution(id) ON DELETE CASCADE,
    -- NULL institution = varsayılan/cepten ödeme (oop) fiyatı
    price                    NUMERIC(12,2) NOT NULL,
    currency                 TEXT NOT NULL DEFAULT 'TRY',
    valid_from               DATE NOT NULL DEFAULT CURRENT_DATE,
    valid_to                 DATE,
    notes                    TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_service_price_lookup ON service_price(service_catalog_id, external_institution_id, valid_from DESC);
CREATE INDEX idx_service_price_org ON service_price(organization_id);

CREATE TRIGGER trg_service_price_updated_at BEFORE UPDATE ON service_price
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
