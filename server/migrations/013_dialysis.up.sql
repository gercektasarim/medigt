-- 013_dialysis: dialysis machines + sessions.
--
-- A dialysis_session row carries both the schedule (machine + time +
-- program) and the actual record (pre/post weight, BP, ultrafiltration,
-- complications). Chronic patients have many sessions per week so we
-- index heavily on (branch_id, scheduled_at) and (patient_id, scheduled_at).
--
-- States: scheduled → in_progress → completed (terminal); cancelled at any
-- non-terminal point.

BEGIN;

CREATE TYPE dialysis_status AS ENUM (
    'scheduled',
    'in_progress',
    'completed',
    'cancelled'
);

CREATE TYPE dialysis_modality AS ENUM (
    'hemodialysis',         -- HD
    'hemodiafiltration',    -- HDF
    'peritoneal'            -- PD
);

CREATE TYPE vascular_access_type AS ENUM (
    'av_fistula',           -- A-V fistül
    'av_graft',             -- greft
    'central_catheter',     -- santral kateter
    'peritoneal_catheter',  -- PD kateteri
    'other'
);

-- ---------- Dialysis machines ----------

CREATE TABLE dialysis_machine (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    code            TEXT NOT NULL,             -- "HD-01"
    name            TEXT NOT NULL,             -- "Fresenius 4008S #1"
    manufacturer    TEXT,
    model           TEXT,
    location        TEXT,                      -- "Salon A, 1. yatak"
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (branch_id, code)
);

CREATE INDEX idx_dialysis_machine_branch ON dialysis_machine(branch_id, is_active);

CREATE TRIGGER trg_dialysis_machine_updated_at BEFORE UPDATE ON dialysis_machine
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Dialysis sessions ----------

CREATE SEQUENCE dialysis_session_no_seq START 100000 INCREMENT 1;

CREATE TABLE dialysis_session (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id           UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    session_no          TEXT NOT NULL,

    patient_id          UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    machine_id          UUID REFERENCES dialysis_machine(id) ON DELETE SET NULL,
    -- Optional link to inpatient admission (in-hospital dialysis).
    admission_id        UUID REFERENCES admission(id) ON DELETE SET NULL,
    -- Nurse / technician who ran the session.
    primary_nurse_id    UUID REFERENCES nurse(id) ON DELETE SET NULL,
    -- Supervising nephrologist.
    supervisor_doctor_id UUID REFERENCES doctor(id) ON DELETE SET NULL,

    status              dialysis_status NOT NULL DEFAULT 'scheduled',
    modality            dialysis_modality NOT NULL DEFAULT 'hemodialysis',
    vascular_access     vascular_access_type NOT NULL DEFAULT 'av_fistula',

    scheduled_at        TIMESTAMPTZ NOT NULL,
    duration_minutes    INTEGER NOT NULL DEFAULT 240 CHECK (duration_minutes > 0),

    -- Pre-session
    pre_weight_kg       NUMERIC(5,2),
    pre_systolic_bp     INTEGER,
    pre_diastolic_bp    INTEGER,
    dry_weight_kg       NUMERIC(5,2),

    -- Prescription
    dialyzer_type       TEXT,              -- "Polyflux 17L"
    anticoagulant       TEXT,              -- "Heparin 2000 IU bolus + 1000 IU/h"
    ultrafiltration_target_ml INTEGER,
    blood_flow_rate     INTEGER,           -- ml/min
    dialysate_flow_rate INTEGER,           -- ml/min

    -- Actuals
    started_at          TIMESTAMPTZ,
    ended_at            TIMESTAMPTZ,
    post_weight_kg      NUMERIC(5,2),
    post_systolic_bp    INTEGER,
    post_diastolic_bp   INTEGER,
    actual_ultrafiltration_ml INTEGER,
    complications       TEXT,              -- hipotansiyon, kramp, vb.
    session_notes       TEXT,

    cancelled_at        TIMESTAMPTZ,
    cancelled_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    cancellation_reason TEXT,

    created_by_user_id  UUID REFERENCES app_user(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, session_no)
);

CREATE INDEX idx_dialysis_session_branch_scheduled ON dialysis_session(branch_id, scheduled_at);
CREATE INDEX idx_dialysis_session_status ON dialysis_session(branch_id, status);
CREATE INDEX idx_dialysis_session_patient ON dialysis_session(patient_id, scheduled_at);
CREATE INDEX idx_dialysis_session_machine ON dialysis_session(machine_id, scheduled_at) WHERE machine_id IS NOT NULL;

CREATE TRIGGER trg_dialysis_session_updated_at BEFORE UPDATE ON dialysis_session
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
