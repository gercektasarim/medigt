-- 006_patient: core patient record + insurance lines + known allergies.
-- This is the spine of the clinical flow — appointments, visits, admissions,
-- prescriptions, lab orders and invoices all FK back here.

BEGIN;

CREATE TYPE patient_gender AS ENUM ('male', 'female', 'unknown');

CREATE TYPE patient_identifier_kind AS ENUM (
    'tc',                     -- T.C. Kimlik No
    'passport',               -- pasaport
    'foreigner_id',           -- YKN (yabancı kimlik no)
    'temporary_protection',   -- geçici koruma kimlik no
    'newborn'                 -- doğum sonrası geçici, TC verilmeden önce
);

CREATE TYPE blood_type AS ENUM (
    'A_pos','A_neg','B_pos','B_neg','AB_pos','AB_neg','O_pos','O_neg','unknown'
);

-- Per-org-but-globally-monotonic MRN. A real hospital usually wants
-- org-scoped counters; we'll layer that on top in a later slice if needed.
CREATE SEQUENCE patient_mrn_seq START 100000 INCREMENT 1;

CREATE TABLE patient (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,

    -- Medical Record Number: 8-digit zero-padded sequence value, formatted
    -- on insert by the service layer. UNIQUE per org.
    mrn                 TEXT NOT NULL,

    first_name          TEXT NOT NULL,
    last_name           TEXT NOT NULL,
    birth_date          DATE,
    gender              patient_gender NOT NULL DEFAULT 'unknown',
    blood_type          blood_type NOT NULL DEFAULT 'unknown',

    -- Primary identifier (most common case: TC kimlik no). For multiple
    -- identifiers per patient we'll add patient_identifier table later.
    identifier_kind     patient_identifier_kind,
    identifier_value    TEXT,
    mernis_verified_at  TIMESTAMPTZ,

    -- Primary contact embedded for fast access. Multi-contact rows live
    -- in patient_contact (added in a later slice).
    phone               TEXT,
    email               TEXT,
    address             TEXT,

    -- Caller / emergency contact in plain text — full relation model later.
    next_of_kin_name    TEXT,
    next_of_kin_phone   TEXT,

    notes               TEXT,
    is_deceased         BOOLEAN NOT NULL DEFAULT FALSE,
    deceased_at         DATE,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, mrn),
    -- One identifier value per kind per org (e.g. one TC per org).
    -- NULLS NOT DISTINCT so multiple NULL identifiers don't collide.
    UNIQUE NULLS NOT DISTINCT (organization_id, identifier_kind, identifier_value)
);

CREATE INDEX idx_patient_org ON patient(organization_id);
CREATE INDEX idx_patient_org_name ON patient(organization_id, last_name, first_name);
CREATE INDEX idx_patient_phone ON patient(organization_id, phone) WHERE phone IS NOT NULL;
CREATE INDEX idx_patient_identifier ON patient(organization_id, identifier_value) WHERE identifier_value IS NOT NULL;
-- Turkish-friendly full-text on name + identifier + phone for global search.
CREATE INDEX idx_patient_fts ON patient
    USING gin(to_tsvector('simple',
        first_name || ' ' || last_name || ' ' ||
        COALESCE(identifier_value, '') || ' ' ||
        COALESCE(phone, '') || ' ' ||
        COALESCE(mrn, '')
    ));

CREATE TRIGGER trg_patient_updated_at BEFORE UPDATE ON patient
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Sigorta poliçeleri ----------

CREATE TYPE insurance_status AS ENUM ('active', 'expired', 'cancelled', 'pending');

CREATE TABLE patient_insurance (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id          UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    patient_id               UUID NOT NULL REFERENCES patient(id) ON DELETE CASCADE,
    external_institution_id  UUID NOT NULL REFERENCES external_institution(id) ON DELETE RESTRICT,
    policy_no                TEXT,
    is_primary               BOOLEAN NOT NULL DEFAULT FALSE,
    status                   insurance_status NOT NULL DEFAULT 'active',
    valid_from               DATE,
    valid_to                 DATE,
    notes                    TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pat_ins_patient ON patient_insurance(patient_id);
CREATE INDEX idx_pat_ins_org ON patient_insurance(organization_id);

CREATE TRIGGER trg_patient_insurance_updated_at BEFORE UPDATE ON patient_insurance
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Bilinen alerjiler ----------

CREATE TYPE allergy_severity AS ENUM ('mild', 'moderate', 'severe', 'unknown');

CREATE TABLE patient_allergy (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    patient_id      UUID NOT NULL REFERENCES patient(id) ON DELETE CASCADE,
    substance       TEXT NOT NULL,
    reaction        TEXT,
    severity        allergy_severity NOT NULL DEFAULT 'unknown',
    onset_year      INTEGER,
    source          TEXT,        -- "patient_reported", "lab_test", etc — free-form for now
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (patient_id, substance)
);

CREATE INDEX idx_pat_allergy_patient ON patient_allergy(patient_id);

CREATE TRIGGER trg_patient_allergy_updated_at BEFORE UPDATE ON patient_allergy
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
