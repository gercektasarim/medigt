-- 007_appointment: scheduled patient visits. Branch-scoped — each shube
-- has its own calendar. State machine: scheduled -> arrived -> in_progress
-- -> completed; or no_show / cancelled at any non-terminal point.

BEGIN;

CREATE TYPE appointment_status AS ENUM (
    'scheduled',    -- planlandı, hasta gelmedi
    'arrived',      -- hasta geldi, beklemede
    'in_progress',  -- muayene içinde
    'completed',    -- tamamlandı
    'no_show',      -- hasta gelmedi
    'cancelled'     -- iptal
);

CREATE TYPE visit_kind AS ENUM (
    'outpatient',   -- poliklinik
    'follow_up',    -- kontrol
    'emergency',    -- acil
    'consultation', -- konsültasyon (başka doktor görüşü)
    'control'       -- kontrol muayenesi
);

CREATE TABLE appointment (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    patient_id      UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    -- Doctor + department are nullable so a receptionist can pre-book even
    -- before a doctor is assigned (rare but real for triage windows).
    doctor_id       UUID REFERENCES doctor(id) ON DELETE SET NULL,
    department_id   UUID REFERENCES department(id) ON DELETE SET NULL,

    scheduled_at        TIMESTAMPTZ NOT NULL,
    duration_minutes    INTEGER NOT NULL DEFAULT 20
        CHECK (duration_minutes BETWEEN 5 AND 480),

    status              appointment_status NOT NULL DEFAULT 'scheduled',
    kind                visit_kind NOT NULL DEFAULT 'outpatient',

    reason              TEXT,       -- hasta şikayeti / randevu notu
    notes               TEXT,       -- idari notlar

    created_by_user_id  UUID REFERENCES app_user(id) ON DELETE SET NULL,

    arrived_at          TIMESTAMPTZ,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    cancelled_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    cancellation_reason TEXT,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_appt_branch ON appointment(branch_id);
CREATE INDEX idx_appt_patient ON appointment(patient_id);
CREATE INDEX idx_appt_doctor ON appointment(doctor_id) WHERE doctor_id IS NOT NULL;
CREATE INDEX idx_appt_scheduled ON appointment(branch_id, scheduled_at);
CREATE INDEX idx_appt_status ON appointment(branch_id, status);

CREATE TRIGGER trg_appointment_updated_at BEFORE UPDATE ON appointment
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
