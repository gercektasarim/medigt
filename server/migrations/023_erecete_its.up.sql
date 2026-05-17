-- 023_erecete_its: e-Reçete (Sağlık Bakanlığı) + İTS (İlaç Takip Sistemi)
-- entegrasyon iskeleti. SGK Medula'dan ayrı iki adapter, aynı outbox tablosu.
--
-- Sertifika gelene kadar mock client'lar deterministik cevap dönüyor.
-- Production swap: internal/integration/erecete/ ve internal/integration/its/
-- altındaki Client interface'lerinin gerçek implementasyonlarını ekle.

BEGIN;

-- ---------- e-Reçete state ----------

CREATE TYPE e_prescription_status AS ENUM (
    'not_submitted',     -- imzalandı ama henüz gönderilmedi
    'queued',            -- outbox'ta beklemede
    'in_progress',       -- worker SOAP çağrısı yapıyor
    'submitted',         -- Sağlık Bakanlığı kabul etti, e_prescription_no var
    'rejected',          -- bakanlık reddetti
    'cancelled',         -- iptal edildi
    'failed'             -- max retry sonrası ölmüş
);

ALTER TABLE prescription
    ADD COLUMN e_prescription_status e_prescription_status NOT NULL DEFAULT 'not_submitted',
    ADD COLUMN e_prescription_submitted_at TIMESTAMPTZ,
    ADD COLUMN e_prescription_response JSONB NOT NULL DEFAULT '{}'::JSONB,
    ADD COLUMN e_prescription_error TEXT;

CREATE INDEX idx_prescription_erecete_status ON prescription(organization_id, e_prescription_status);

-- ---------- İTS notification state ----------
--
-- prescription_dispense satırı eczane dispense ettikten sonra İTS'e
-- karekod bildirimi yapar. its_status sürecin durumu, its_response
-- bakanlık cevabı. Outbox üzerinden gider.

CREATE TYPE its_notify_status AS ENUM (
    'pending',           -- worker bekliyor
    'in_progress',
    'notified',          -- bakanlık aldı
    'rejected',
    'failed'
);

ALTER TABLE prescription_dispense
    ADD COLUMN its_status its_notify_status NOT NULL DEFAULT 'pending',
    ADD COLUMN its_notified_at TIMESTAMPTZ,
    ADD COLUMN its_response JSONB NOT NULL DEFAULT '{}'::JSONB,
    ADD COLUMN its_error TEXT,
    -- İlacın karekodu (GTIN + lot + skt + seri); dispense sırasında set edilir
    ADD COLUMN karekod TEXT;

CREATE INDEX idx_dispense_its_status ON prescription_dispense(branch_id, its_status);

COMMIT;
