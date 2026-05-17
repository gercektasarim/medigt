-- 008_clinical: the clinical encounter layer.
--
-- visit            — one outpatient/follow-up/emergency encounter
-- diagnosis        — ICD-10 codes attached to a visit (primary + secondaries)
-- prescription     — header (status: draft/signed/sent_to_sgk/dispensed/...)
-- prescription_item — individual drugs, free-text for now
-- vital_signs      — time-series measurements within a visit
--
-- A visit can be opened directly (walk-in) or linked to an appointment
-- (appointment_id UNIQUE — one visit per appointment).

BEGIN;

CREATE TYPE visit_status AS ENUM ('in_progress', 'completed', 'cancelled');

CREATE TYPE encounter_type AS ENUM (
    'outpatient',
    'emergency',
    'follow_up',
    'consultation',
    'control',
    'admission'
);

CREATE TABLE visit (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    patient_id      UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    doctor_id       UUID REFERENCES doctor(id) ON DELETE SET NULL,

    -- One visit per appointment, max. UNIQUE handles the link both ways.
    appointment_id  UUID UNIQUE REFERENCES appointment(id) ON DELETE SET NULL,

    encounter_type  encounter_type NOT NULL DEFAULT 'outpatient',
    status          visit_status NOT NULL DEFAULT 'in_progress',

    chief_complaint            TEXT,    -- hasta şikayeti (kısa)
    history_of_present_illness TEXT,    -- hikaye (anamnez ana metin)
    examination_findings       TEXT,    -- fizik muayene
    treatment_plan             TEXT,    -- tedavi planı
    notes                      TEXT,    -- ek not

    opened_by_user_id  UUID REFERENCES app_user(id) ON DELETE SET NULL,
    started_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at           TIMESTAMPTZ,

    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_visit_branch ON visit(branch_id);
CREATE INDEX idx_visit_patient ON visit(patient_id);
CREATE INDEX idx_visit_doctor ON visit(doctor_id) WHERE doctor_id IS NOT NULL;
CREATE INDEX idx_visit_branch_status ON visit(branch_id, status);
CREATE INDEX idx_visit_started ON visit(branch_id, started_at DESC);

CREATE TRIGGER trg_visit_updated_at BEFORE UPDATE ON visit
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Diagnosis ----------

CREATE TYPE diagnosis_kind AS ENUM (
    'primary',
    'secondary',
    'provisional',
    'differential',
    'ruled_out'
);

CREATE TABLE diagnosis (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    visit_id    UUID NOT NULL REFERENCES visit(id) ON DELETE CASCADE,
    -- Snapshot the ICD-10 code + title at write time so the diagnosis stays
    -- readable even if the catalog row is later edited or deleted.
    icd10_code  TEXT NOT NULL,
    icd10_title TEXT NOT NULL,
    kind        diagnosis_kind NOT NULL DEFAULT 'primary',
    notes       TEXT,
    created_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_diagnosis_visit ON diagnosis(visit_id);
CREATE INDEX idx_diagnosis_code ON diagnosis(icd10_code);

-- ---------- Prescription ----------

CREATE TYPE prescription_status AS ENUM (
    'draft',
    'signed',
    'sent_to_sgk',
    'dispensed',
    'cancelled'
);

-- 8-digit zero-padded prescription numbers. Global like patient MRN for now;
-- per-org/per-branch counters can layer on later if a hospital needs them.
CREATE SEQUENCE prescription_no_seq START 100000 INCREMENT 1;

CREATE TABLE prescription (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    visit_id            UUID NOT NULL REFERENCES visit(id) ON DELETE CASCADE,
    patient_id          UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    doctor_id           UUID REFERENCES doctor(id) ON DELETE SET NULL,

    prescription_no     TEXT NOT NULL,
    e_prescription_no   TEXT,                       -- SGK e-reçete numarası
    status              prescription_status NOT NULL DEFAULT 'draft',
    notes               TEXT,

    signed_at           TIMESTAMPTZ,
    sent_to_sgk_at      TIMESTAMPTZ,
    dispensed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, prescription_no)
);

CREATE INDEX idx_prescription_visit ON prescription(visit_id);
CREATE INDEX idx_prescription_patient ON prescription(patient_id);
CREATE INDEX idx_prescription_status ON prescription(organization_id, status);

CREATE TRIGGER trg_prescription_updated_at BEFORE UPDATE ON prescription
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE prescription_item (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prescription_id UUID NOT NULL REFERENCES prescription(id) ON DELETE CASCADE,

    medication_name TEXT NOT NULL,           -- free-text; medication catalog layer later
    dosage          TEXT,                    -- "500 mg", "10 mg/ml"
    frequency       TEXT,                    -- "günde 2 kez", "8 saatte bir"
    duration_days   INTEGER CHECK (duration_days IS NULL OR duration_days > 0),
    quantity        TEXT,                    -- "1 kutu", "30 tablet"
    instructions    TEXT,                    -- "yemeklerden sonra"

    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_prescription_item_rx ON prescription_item(prescription_id, sort_order);

-- ---------- Vital signs ----------

CREATE TABLE vital_signs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    patient_id          UUID NOT NULL REFERENCES patient(id) ON DELETE CASCADE,
    visit_id            UUID REFERENCES visit(id) ON DELETE CASCADE,

    measured_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    measured_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,

    systolic_bp         INTEGER CHECK (systolic_bp IS NULL OR systolic_bp BETWEEN 30 AND 300),
    diastolic_bp        INTEGER CHECK (diastolic_bp IS NULL OR diastolic_bp BETWEEN 10 AND 250),
    pulse               INTEGER CHECK (pulse IS NULL OR pulse BETWEEN 20 AND 250),
    temperature_c       NUMERIC(4,1) CHECK (temperature_c IS NULL OR temperature_c BETWEEN 25 AND 45),
    spo2_percent        INTEGER CHECK (spo2_percent IS NULL OR spo2_percent BETWEEN 30 AND 100),
    respiration         INTEGER CHECK (respiration IS NULL OR respiration BETWEEN 4 AND 80),
    weight_kg           NUMERIC(5,2) CHECK (weight_kg IS NULL OR weight_kg BETWEEN 0.1 AND 400),
    height_cm           NUMERIC(5,1) CHECK (height_cm IS NULL OR height_cm BETWEEN 20 AND 280),
    pain_score          INTEGER CHECK (pain_score IS NULL OR pain_score BETWEEN 0 AND 10),

    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vitals_patient ON vital_signs(patient_id, measured_at DESC);
CREATE INDEX idx_vitals_visit ON vital_signs(visit_id) WHERE visit_id IS NOT NULL;

COMMIT;
