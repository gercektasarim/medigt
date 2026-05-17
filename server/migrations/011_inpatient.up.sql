-- 011_inpatient: wards, beds, admissions, bed transfers.
--
-- Admission = a single inpatient stay. Lifecycle:
--   active → discharged (terminal)
-- Bed status transitions are driven by the admission service:
--   on admit:    bed.free        → occupied
--   on transfer: from.occupied   → cleaning, to.free → occupied
--   on discharge: bed.occupied   → cleaning  (housekeeping later flips to free)
--
-- All mutations go through the service layer with SELECT … FOR UPDATE so
-- two clerks can't double-book the same bed.

BEGIN;

CREATE TYPE ward_kind AS ENUM (
    'general',      -- yatış servisi
    'icu',          -- yoğun bakım
    'ccu',          -- koroner yoğun bakım
    'pediatrics',   -- çocuk servisi
    'maternity',    -- doğum / kadın doğum
    'surgical',     -- cerrahi servisi
    'isolation',    -- izolasyon
    'observation'   -- gözlem
);

CREATE TYPE bed_kind AS ENUM (
    'standard',
    'icu',
    'isolation',
    'pediatric',
    'vip',
    'observation'
);

CREATE TYPE bed_status AS ENUM (
    'free',         -- boş, hazır
    'occupied',     -- hasta yatıyor
    'reserved',     -- bekleyen yatış için ayrıldı
    'cleaning',     -- temizleniyor / hazırlanıyor
    'blocked'       -- bakım / arıza
);

CREATE TYPE admission_kind AS ENUM (
    'planned',      -- planlı yatış
    'emergency',    -- acil
    'transfer_in',  -- başka kurumdan transfer
    'newborn'       -- doğum sonrası yenidoğan kaydı
);

CREATE TYPE admission_status AS ENUM (
    'active',
    'discharged'
);

CREATE TYPE discharge_kind AS ENUM (
    'home',                 -- evde takip
    'home_with_help',       -- evde hemşirelik / VHK
    'referred',             -- başka kuruma sevk
    'against_advice',       -- önerilere rağmen taburcu
    'left_without_notice',  -- haber vermeden ayrıldı
    'transferred',          -- iç transfer (genelde başka servis/branch)
    'expired'               -- ex
);

-- ---------- Ward (servis) ----------

CREATE TABLE ward (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    code            TEXT NOT NULL,        -- "DAH3", "ICU1"
    name            TEXT NOT NULL,
    kind            ward_kind NOT NULL DEFAULT 'general',
    floor           TEXT,
    capacity        INTEGER CHECK (capacity IS NULL OR capacity > 0),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (branch_id, code)
);

CREATE INDEX idx_ward_branch ON ward(branch_id, is_active);

CREATE TRIGGER trg_ward_updated_at BEFORE UPDATE ON ward
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Bed (yatak) ----------

CREATE TABLE bed (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ward_id     UUID NOT NULL REFERENCES ward(id) ON DELETE CASCADE,
    code        TEXT NOT NULL,
    kind        bed_kind NOT NULL DEFAULT 'standard',
    status      bed_status NOT NULL DEFAULT 'free',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (ward_id, code)
);

CREATE INDEX idx_bed_ward_status ON bed(ward_id, status);

CREATE TRIGGER trg_bed_updated_at BEFORE UPDATE ON bed
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Admission ----------

CREATE SEQUENCE admission_no_seq START 100000 INCREMENT 1;

CREATE TABLE admission (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id             UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    admission_no          TEXT NOT NULL,

    patient_id            UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    admitting_doctor_id   UUID REFERENCES doctor(id) ON DELETE SET NULL,
    ward_id               UUID NOT NULL REFERENCES ward(id) ON DELETE RESTRICT,
    bed_id                UUID REFERENCES bed(id) ON DELETE SET NULL,

    kind                  admission_kind NOT NULL DEFAULT 'planned',
    status                admission_status NOT NULL DEFAULT 'active',

    chief_complaint       TEXT,
    admission_diagnosis   TEXT,
    notes                 TEXT,

    admitted_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    admitted_by_user_id   UUID REFERENCES app_user(id) ON DELETE SET NULL,

    -- Discharge fields are embedded; population implies status='discharged'.
    discharged_at         TIMESTAMPTZ,
    discharge_kind        discharge_kind,
    discharge_summary     TEXT,
    discharged_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, admission_no),
    -- Discharged rows must have a stamp + kind + summary OR none of them.
    CHECK (
        (status = 'active' AND discharged_at IS NULL AND discharge_kind IS NULL)
        OR
        (status = 'discharged' AND discharged_at IS NOT NULL AND discharge_kind IS NOT NULL)
    )
);

CREATE INDEX idx_admission_branch_status ON admission(branch_id, status);
CREATE INDEX idx_admission_patient ON admission(patient_id);
CREATE INDEX idx_admission_ward ON admission(ward_id) WHERE status = 'active';
CREATE INDEX idx_admission_bed ON admission(bed_id) WHERE status = 'active';

-- Only one *active* admission per patient at any given time. Discharged rows
-- are unrestricted.
CREATE UNIQUE INDEX idx_admission_patient_active
    ON admission(patient_id) WHERE status = 'active';

CREATE TRIGGER trg_admission_updated_at BEFORE UPDATE ON admission
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Bed transfer audit ----------

CREATE TABLE bed_transfer (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id           UUID NOT NULL REFERENCES admission(id) ON DELETE CASCADE,
    from_bed_id            UUID REFERENCES bed(id) ON DELETE SET NULL,
    to_bed_id              UUID NOT NULL REFERENCES bed(id) ON DELETE RESTRICT,
    transferred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    transferred_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    reason                 TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bedtransfer_admission ON bed_transfer(admission_id, transferred_at DESC);

COMMIT;
