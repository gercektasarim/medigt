-- 022_finance_extensions: patient cari hesap (avans) + iade + taksit planı.
--
-- Mali invariant'lar:
--   - patient_account_entry append-only; balance = SUM(direction * amount)
--   - refund: invoice.paid_total düşürür, cash kasa hareketi yaratır (nakit ise)
--     veya avansa kredi olarak yazılır (to_advance=true)
--   - installment_plan: bilgi katmanı; her installment 1+ payment ile ödenebilir
--
-- Tüm bu mutasyonlar service layer'da transactional, çift-girişli muhasebe
-- mantığıyla yazılır (servis dosyaları için bkz. internal/service/).

BEGIN;

-- ---------- Patient account (hasta cari hesabı) ----------
--
-- Burada özet bir 'balance' kolonu DENORMALİZE etmiyoruz. Her okuma için
-- SUM(direction * amount) yapılır. İlerde performans gerekirse cache
-- eklenebilir; doğruluk ledger'da.

CREATE TYPE patient_account_entry_kind AS ENUM (
    'advance_in',         -- avans alındı (kasaya nakit girdi → cariye kredi)
    'advance_use',        -- avans faturaya uygulandı (cariden düşer)
    'advance_refund',     -- avans iade edildi (cariden çıktı, kasadan nakit çıktı)
    'refund_to_advance'   -- fatura iadesi avansa yazıldı (cariye kredi)
);

CREATE TABLE patient_account_entry (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id        UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    patient_id       UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    kind             patient_account_entry_kind NOT NULL,
    amount           NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    -- Yön: +1 = avans kredi (hasta lehine), -1 = avans çıkış / kullanım
    direction        INT NOT NULL CHECK (direction IN (1, -1)),
    -- Source linkler (denetim izi)
    payment_id       UUID REFERENCES payment(id) ON DELETE SET NULL,
    invoice_id       UUID REFERENCES invoice(id) ON DELETE SET NULL,
    cash_movement_id UUID REFERENCES cash_movement(id) ON DELETE SET NULL,
    -- refund_id sonradan tablo ekleninceye kadar boş
    refund_id        UUID,
    notes            TEXT,
    performed_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    performed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pae_patient_at ON patient_account_entry(patient_id, performed_at DESC);
CREATE INDEX idx_pae_branch_at ON patient_account_entry(branch_id, performed_at DESC);

-- ---------- Refund (fatura iadesi) ----------

CREATE SEQUENCE refund_no_seq START 100000 INCREMENT 1;

CREATE TABLE refund (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id         UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    refund_no         TEXT NOT NULL,
    patient_id        UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    -- Kaynak: orijinal ödeme veya fatura (en az biri zorunlu)
    payment_id        UUID REFERENCES payment(id) ON DELETE SET NULL,
    invoice_id        UUID REFERENCES invoice(id) ON DELETE SET NULL,
    amount            NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    method            payment_method NOT NULL,
    -- Nakit ise kasa zorunlu
    cash_register_id  UUID REFERENCES cash_register(id),
    cash_movement_id  UUID REFERENCES cash_movement(id),
    -- Veya cari avansa kredi yazılır
    to_advance        BOOLEAN NOT NULL DEFAULT FALSE,
    reason            TEXT,
    performed_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    performed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, refund_no),
    -- En az bir kaynak link gerekli
    CHECK (payment_id IS NOT NULL OR invoice_id IS NOT NULL),
    -- Nakit iade için kasa zorunlu (to_advance true ise istisna)
    CHECK ((method <> 'cash') OR to_advance OR cash_register_id IS NOT NULL)
);

CREATE INDEX idx_refund_branch_at ON refund(branch_id, performed_at DESC);
CREATE INDEX idx_refund_patient ON refund(patient_id, performed_at DESC);
CREATE INDEX idx_refund_invoice ON refund(invoice_id) WHERE invoice_id IS NOT NULL;
CREATE INDEX idx_refund_payment ON refund(payment_id) WHERE payment_id IS NOT NULL;

-- ---------- Installment (taksit planı) ----------

CREATE TYPE installment_plan_status AS ENUM ('active', 'completed', 'cancelled');

CREATE TABLE installment_plan (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id         UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    invoice_id        UUID NOT NULL REFERENCES invoice(id) ON DELETE RESTRICT,
    total_amount      NUMERIC(14,2) NOT NULL CHECK (total_amount > 0),
    num_installments  INT NOT NULL CHECK (num_installments > 0),
    status            installment_plan_status NOT NULL DEFAULT 'active',
    notes             TEXT,
    created_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Bir faturaya bir plan
    UNIQUE (invoice_id)
);

CREATE INDEX idx_installment_plan_branch ON installment_plan(branch_id, status);

CREATE TRIGGER trg_installment_plan_updated_at BEFORE UPDATE ON installment_plan
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TYPE installment_status AS ENUM ('pending', 'paid', 'partial', 'overdue', 'cancelled');

CREATE TABLE installment (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id     UUID NOT NULL REFERENCES installment_plan(id) ON DELETE CASCADE,
    seq         INT NOT NULL CHECK (seq > 0),
    due_date    DATE NOT NULL,
    amount      NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    paid_amount NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (paid_amount >= 0),
    status      installment_status NOT NULL DEFAULT 'pending',
    paid_at     TIMESTAMPTZ,
    -- Bu taksiti kapatan en son payment (kısmi ödemelerde null kalabilir)
    payment_id  UUID REFERENCES payment(id) ON DELETE SET NULL,
    notes       TEXT,
    UNIQUE (plan_id, seq)
);

CREATE INDEX idx_installment_due ON installment(due_date, status);
CREATE INDEX idx_installment_plan_seq ON installment(plan_id, seq);

COMMIT;
