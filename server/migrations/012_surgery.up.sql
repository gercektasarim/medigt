-- 012_surgery: operating rooms + planned/performed surgeries.
--
-- A surgery row carries both the schedule (when + where + who) and the
-- post-op record (op note + actual times). The surgical team lives as
-- JSONB for the first slice; if hospitals need per-member earnings later
-- it gets normalised into surgery_team_member.
--
-- States: scheduled → in_progress → completed (terminal); cancelled at
-- any non-terminal point.

BEGIN;

CREATE TYPE surgery_status AS ENUM (
    'scheduled',
    'in_progress',
    'completed',
    'cancelled'
);

CREATE TYPE surgery_priority AS ENUM (
    'elective',     -- elektif
    'urgent',       -- aciliyet
    'emergency'     -- acil (anında)
);

CREATE TYPE anesthesia_type AS ENUM (
    'general',
    'regional',
    'spinal',
    'epidural',
    'local',
    'sedation',
    'none'
);

-- ---------- Operating rooms ----------

CREATE TABLE operating_room (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    code            TEXT NOT NULL,
    name            TEXT NOT NULL,
    floor           TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (branch_id, code)
);

CREATE INDEX idx_or_branch ON operating_room(branch_id, is_active);

CREATE TRIGGER trg_or_updated_at BEFORE UPDATE ON operating_room
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Surgeries ----------

CREATE SEQUENCE surgery_no_seq START 100000 INCREMENT 1;

CREATE TABLE surgery (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id           UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    surgery_no          TEXT NOT NULL,

    patient_id          UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    operating_room_id   UUID NOT NULL REFERENCES operating_room(id) ON DELETE RESTRICT,
    primary_surgeon_id  UUID REFERENCES doctor(id) ON DELETE SET NULL,
    -- Optional link to the inpatient admission this surgery belongs to.
    admission_id        UUID REFERENCES admission(id) ON DELETE SET NULL,

    status              surgery_status NOT NULL DEFAULT 'scheduled',
    priority            surgery_priority NOT NULL DEFAULT 'elective',

    procedure_name      TEXT NOT NULL,    -- "Laparoskopik kolesistektomi"
    procedure_codes     JSONB NOT NULL DEFAULT '[]'::JSONB,  -- ["SUT:401.020", ...]
    indication          TEXT,             -- klinik gerekçe
    anesthesia_type     anesthesia_type NOT NULL DEFAULT 'general',

    scheduled_at        TIMESTAMPTZ NOT NULL,
    estimated_minutes   INTEGER NOT NULL DEFAULT 60 CHECK (estimated_minutes > 0),

    -- Surgical team — JSONB of {staff_member_id, role, name} entries.
    -- Roles: primary_surgeon, assistant, anesthesiologist, scrub_nurse,
    -- circulating_nurse, technician.
    team                JSONB NOT NULL DEFAULT '[]'::JSONB,

    -- Actuals
    started_at          TIMESTAMPTZ,
    ended_at            TIMESTAMPTZ,
    op_note             TEXT,             -- post-op record (free text)
    complications       TEXT,
    blood_loss_ml       INTEGER,
    specimen_sent       BOOLEAN NOT NULL DEFAULT FALSE,

    cancelled_at        TIMESTAMPTZ,
    cancelled_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    cancellation_reason TEXT,

    created_by_user_id  UUID REFERENCES app_user(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, surgery_no)
);

CREATE INDEX idx_surgery_branch_scheduled ON surgery(branch_id, scheduled_at);
CREATE INDEX idx_surgery_status ON surgery(branch_id, status);
CREATE INDEX idx_surgery_or ON surgery(operating_room_id, scheduled_at);
CREATE INDEX idx_surgery_patient ON surgery(patient_id);
CREATE INDEX idx_surgery_surgeon ON surgery(primary_surgeon_id) WHERE primary_surgeon_id IS NOT NULL;

CREATE TRIGGER trg_surgery_updated_at BEFORE UPDATE ON surgery
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
