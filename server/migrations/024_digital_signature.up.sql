-- 024_digital_signature: TURKKEP / E-Tugra cloud e-imza entegrasyonu.
--
-- digital_signature satırı bir kullanıcının belirli bir dokümanı imzalama
-- oturumudur. Oturum başlatılır (TURKKEP'e Init call'u), kullanıcı mobil
-- uygulamasında onaylar (challenge_code), worker Poll ederek imza
-- zarfını (PKCS#7 / CAdES) alır.
--
-- Mali / klinik kritik dokümanlar (reçete, ameliyat notu, taburcu özeti,
-- e-rapor, fatura) bu satırla bağlanır — target_table + target_id ile.

BEGIN;

CREATE TYPE signature_provider AS ENUM (
    'turkkep',     -- TURKKEP cloud
    'eturkce',     -- E-Tugra (alternatif)
    'mock'         -- geliştirme
);

CREATE TYPE signature_status AS ENUM (
    'pending',     -- oturum başladı, kullanıcı henüz onaylamadı
    'in_progress', -- TURKKEP server işliyor
    'signed',      -- başarı: signed_envelope dolu
    'cancelled',
    'failed',
    'expired'      -- 15 dakika içinde onay gelmedi
);

CREATE TABLE digital_signature (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id           UUID REFERENCES branch(id) ON DELETE SET NULL,
    signer_user_id      UUID NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    signer_tc           TEXT NOT NULL,
    signer_full_name    TEXT NOT NULL,

    -- Hangi dokümanı imzalıyoruz
    target_table        TEXT NOT NULL,      -- prescription / surgery / discharge / invoice / medula_eraport / ...
    target_id           UUID NOT NULL,
    document_kind       TEXT NOT NULL,      -- prescription | discharge_summary | op_note | report | invoice | other
    document_hash       TEXT NOT NULL,      -- SHA-256 hex; tampering check için

    -- Provider session
    provider            signature_provider NOT NULL DEFAULT 'turkkep',
    session_id          TEXT,               -- TURKKEP / Tugra session id
    challenge_code      TEXT,               -- mock-only; gerçek provider'da kullanıcının cebine gelir

    -- Final çıktı
    signed_envelope     BYTEA,              -- PKCS#7 / CAdES paketi
    certificate_serial  TEXT,
    certificate_subject TEXT,

    status              signature_status NOT NULL DEFAULT 'pending',
    error_message       TEXT,
    initiated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    signed_at           TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '15 minutes'),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_signature_signer_status ON digital_signature(signer_user_id, status, initiated_at DESC);
CREATE INDEX idx_signature_target ON digital_signature(target_table, target_id);
CREATE INDEX idx_signature_pending ON digital_signature(status, expires_at) WHERE status IN ('pending', 'in_progress');

CREATE TRIGGER trg_signature_updated_at BEFORE UPDATE ON digital_signature
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Prescription bağlantısı ----------

ALTER TABLE prescription
    ADD COLUMN digital_signature_id UUID REFERENCES digital_signature(id) ON DELETE SET NULL;

CREATE INDEX idx_prescription_signature ON prescription(digital_signature_id) WHERE digital_signature_id IS NOT NULL;

COMMIT;
