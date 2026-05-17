-- 021_medula_expansion: full SGK Medula service surface.
--
-- 14 servisin tamamı için tablo + ENUM iskeleti. Mock client bugün
-- deterministik cevap döner; sertifika gelince adapter swap edilir,
-- tablolar değişmez.
--
-- Yazma operasyonları (provision/invoice/referral/eraport) outbox üzerinden
-- async; sorgular sync RPC + isteğe bağlı cache (UI bekler).

BEGIN;

-- ---------- Provision cancel + close (mevcut tabloya ek alanlar) ----------

ALTER TABLE medula_provision
    ADD COLUMN cancelled_at TIMESTAMPTZ,
    ADD COLUMN cancellation_reason TEXT,
    ADD COLUMN closed_at TIMESTAMPTZ;

-- ---------- Invoice submission (fatura SGK'ya gönder) ----------

CREATE TYPE medula_submit_status AS ENUM (
    'pending',
    'in_progress',
    'submitted',
    'accepted',
    'rejected',
    'cancelled',
    'failed'
);

CREATE TABLE medula_invoice_submission (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    invoice_id      UUID NOT NULL REFERENCES invoice(id) ON DELETE RESTRICT,
    -- Provision the invoice was filed against (one invoice can roll up
    -- multiple provisions later — JSONB list).
    provision_id    UUID REFERENCES medula_provision(id) ON DELETE SET NULL,
    -- SGK assigns a batch number on accept; nullable until then.
    batch_no        TEXT,
    -- SGK invoice number (different from our internal invoice_no).
    sgk_invoice_no  TEXT,

    status          medula_submit_status NOT NULL DEFAULT 'pending',
    request_payload  JSONB NOT NULL DEFAULT '{}'::JSONB,
    response_payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    response_code    TEXT,
    error_message    TEXT,
    cancelled_at     TIMESTAMPTZ,
    cancellation_reason TEXT,

    requested_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    requested_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, invoice_id)
);

CREATE INDEX idx_medula_submit_branch_status ON medula_invoice_submission(branch_id, status, requested_at DESC);
CREATE INDEX idx_medula_submit_batch ON medula_invoice_submission(batch_no) WHERE batch_no IS NOT NULL;

CREATE TRIGGER trg_medula_submit_updated_at BEFORE UPDATE ON medula_invoice_submission
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Referral (sevk) ----------

CREATE TYPE medula_referral_status AS ENUM (
    'pending',
    'in_progress',
    'created',
    'rejected',
    'cancelled',
    'failed'
);

CREATE TABLE medula_referral (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    patient_id      UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    -- Sevk eden doktor
    referring_doctor_id UUID REFERENCES doctor(id) ON DELETE SET NULL,
    -- Hedef kurum (SGK koduyla) — özgür metin; tesis kataloğu V2'de
    target_provider_code TEXT NOT NULL,
    target_provider_name TEXT,
    target_branch_code   TEXT,           -- SGK branş kodu
    -- Klinik gerekçe
    reason          TEXT NOT NULL,
    diagnosis_icd10 TEXT,
    referral_type   TEXT NOT NULL DEFAULT 'normal',  -- normal / acil / kontrol

    status          medula_referral_status NOT NULL DEFAULT 'pending',
    -- SGK sevk numarası (oluştuktan sonra gelir)
    sevk_no         TEXT,

    request_payload  JSONB NOT NULL DEFAULT '{}'::JSONB,
    response_payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    response_code    TEXT,
    error_message    TEXT,

    requested_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    requested_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    cancelled_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_medula_referral_branch_status ON medula_referral(branch_id, status, requested_at DESC);
CREATE INDEX idx_medula_referral_patient ON medula_referral(patient_id, requested_at DESC);

CREATE TRIGGER trg_medula_referral_updated_at BEFORE UPDATE ON medula_referral
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- e-Rapor (kronik / yatış / iş göremezlik) ----------

CREATE TYPE medula_eraport_kind AS ENUM (
    'chronic_drug',        -- kronik ilaç raporu
    'inpatient',           -- yatış raporu
    'work_incapacity',     -- iş göremezlik
    'special_procedure'    -- özel girişimsel rapor
);

CREATE TYPE medula_eraport_status AS ENUM (
    'pending',
    'in_progress',
    'submitted',
    'approved',
    'rejected',
    'cancelled',
    'failed'
);

CREATE TABLE medula_eraport (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    patient_id      UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    doctor_id       UUID REFERENCES doctor(id) ON DELETE SET NULL,
    kind            medula_eraport_kind NOT NULL,

    -- ICD-10 tanılar, ilaç kodları (ATC) — JSONB liste
    diagnoses_icd10 JSONB NOT NULL DEFAULT '[]'::JSONB,
    drug_codes      JSONB NOT NULL DEFAULT '[]'::JSONB,
    valid_from      DATE NOT NULL,
    valid_to        DATE,
    -- Rapor metni (HL7/SGK formatı; mock'ta serbest text)
    report_text     TEXT,

    status          medula_eraport_status NOT NULL DEFAULT 'pending',
    eraport_no      TEXT,   -- SGK rapor numarası

    request_payload  JSONB NOT NULL DEFAULT '{}'::JSONB,
    response_payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    response_code    TEXT,
    error_message    TEXT,

    requested_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    requested_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    cancelled_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_medula_eraport_branch_status ON medula_eraport(branch_id, status, requested_at DESC);
CREATE INDEX idx_medula_eraport_patient ON medula_eraport(patient_id, requested_at DESC);

CREATE TRIGGER trg_medula_eraport_updated_at BEFORE UPDATE ON medula_eraport
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
