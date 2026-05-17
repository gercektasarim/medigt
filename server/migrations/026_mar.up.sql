-- 026_mar: Medication Administration Record — yatan hasta için ilaç
-- siparişleri (medication_order) ve fiilen verilen dozların kaydı
-- (medication_administration). 5 doğru protokolü (doğru hasta, ilaç, doz,
-- yol, zaman) verme anında barkod taramayla doğrulanır.

BEGIN;

CREATE TYPE medication_order_status AS ENUM (
    'active',
    'on_hold',
    'completed',
    'cancelled',
    'expired'
);

CREATE TYPE medication_route AS ENUM (
    'oral',
    'iv',
    'im',
    'sc',
    'topical',
    'inhalation',
    'rectal',
    'sublingual',
    'intranasal',
    'ophthalmic',
    'otic',
    'other'
);

CREATE TYPE administration_status AS ENUM (
    'given',       -- ilaç verildi
    'refused',     -- hasta reddetti
    'withheld',    -- doktor isteği / NPO / tıbbi sebep
    'missed',      -- süre geçti, verilemedi
    'wrong_time'   -- başka zamanda verildi (deviation)
);

-- ---------- Medication order (doktor ilaç emri) ----------

CREATE SEQUENCE medication_order_no_seq START 1000 INCREMENT 1;

CREATE TABLE medication_order (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id           UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    order_no            TEXT NOT NULL,

    admission_id        UUID NOT NULL REFERENCES admission(id) ON DELETE CASCADE,
    patient_id          UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    medication_id       UUID NOT NULL REFERENCES medication(id) ON DELETE RESTRICT,
    ordering_doctor_id  UUID REFERENCES doctor(id) ON DELETE SET NULL,

    dose_amount         NUMERIC(10,3) NOT NULL CHECK (dose_amount > 0),
    dose_unit           TEXT NOT NULL,                -- mg, mL, tab, IU, ...
    route               medication_route NOT NULL,
    -- Free-text frequency mnemonic (Q8H, BID, TID, QHS, PRN, ...).
    frequency           TEXT NOT NULL,
    -- For non-PRN orders, the scheduler turns frequency + scheduled_times
    -- into individual due slots. Free-form for V1; can switch to JSONB
    -- structured rule later.
    scheduled_times     TIME[] NOT NULL DEFAULT ARRAY[]::TIME[],
    -- PRN (pro re nata) — verilmesi hastanın isteğine/şikayetine bağlı.
    is_prn              BOOLEAN NOT NULL DEFAULT FALSE,
    prn_reason          TEXT,                         -- "ağrı için", "ateş için"

    starts_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at             TIMESTAMPTZ,                  -- NULL = ongoing

    instructions        TEXT,
    status              medication_order_status NOT NULL DEFAULT 'active',

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, order_no),
    CHECK ((is_prn = TRUE AND scheduled_times = ARRAY[]::TIME[])
        OR is_prn = FALSE),
    CHECK (ends_at IS NULL OR ends_at > starts_at)
);

CREATE INDEX idx_med_order_admission ON medication_order(admission_id, status);
CREATE INDEX idx_med_order_patient ON medication_order(patient_id, status);
CREATE INDEX idx_med_order_branch ON medication_order(branch_id, status, starts_at DESC);

CREATE TRIGGER trg_medication_order_updated_at BEFORE UPDATE ON medication_order
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Medication administration (verilen doz kayıt) ----------

CREATE TABLE medication_administration (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id             UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id                   UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    medication_order_id         UUID NOT NULL REFERENCES medication_order(id) ON DELETE CASCADE,
    admission_id                UUID NOT NULL REFERENCES admission(id) ON DELETE CASCADE,
    patient_id                  UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,

    -- The scheduled slot this fulfills (the calendar tick: e.g. 08:00 today).
    -- For PRN orders this is NULL — verme talep üzerine olur.
    scheduled_at                TIMESTAMPTZ,
    administered_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    status                      administration_status NOT NULL DEFAULT 'given',

    -- 5 doğru audit trail. Required for status='given'.
    five_rights_checked         BOOLEAN NOT NULL DEFAULT FALSE,
    patient_barcode_scanned     TEXT,
    medication_barcode_scanned  TEXT,

    -- Actual dose given (verilen tutar emirden sapmış olabilir; CYA için).
    dose_amount                 NUMERIC(10,3),
    dose_unit                   TEXT,
    route                       medication_route,

    notes                       TEXT,

    performed_by_user_id        UUID REFERENCES app_user(id) ON DELETE SET NULL,
    -- Controlled-substance double-check (örn. opioid çekilmesi).
    witnessed_by_user_id        UUID REFERENCES app_user(id) ON DELETE SET NULL,

    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Either the 5-rights check is true (and 'given'), or status is one of
    -- the non-given reasons. We don't insist on barcodes — some MED carts
    -- are not yet barcoded; the nurse can confirm manually.
    CHECK (
        (status = 'given' AND five_rights_checked = TRUE)
        OR status <> 'given'
    )
);

CREATE INDEX idx_med_admin_order ON medication_administration(medication_order_id, administered_at DESC);
CREATE INDEX idx_med_admin_admission ON medication_administration(admission_id, administered_at DESC);
CREATE INDEX idx_med_admin_patient ON medication_administration(patient_id, administered_at DESC);
CREATE INDEX idx_med_admin_scheduled ON medication_administration(scheduled_at) WHERE scheduled_at IS NOT NULL;

COMMIT;
