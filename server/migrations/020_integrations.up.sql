-- 020_integrations: MERNIS verification log + Medula provision + outbox.
--
-- KVKK requirement (CLAUDE.md): TC kimlik no in audit details = last 4
-- digits only. Full TC is captured in encrypted blobs at app layer.
--
-- The Medula integration uses the outbox pattern: synchronous user
-- requests get 202 Accepted; a background worker polls pending messages
-- and runs the SOAP call. Test environment is mock today; replace the
-- client when SGK certification arrives.

BEGIN;

-- ---------- MERNIS verification log ----------

CREATE TABLE mernis_verification_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID REFERENCES branch(id) ON DELETE SET NULL,
    -- KVKK: only the last 4 digits are stored in clear; full TC lives
    -- inside the encrypted blob.
    tc_last4        TEXT NOT NULL CHECK (length(tc_last4) = 4),
    tc_enc          BYTEA,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    birth_year      INTEGER NOT NULL CHECK (birth_year BETWEEN 1900 AND 2100),
    verified        BOOLEAN NOT NULL,
    response_code   TEXT,                       -- NVI response code or simulated
    error_message   TEXT,
    requested_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    requested_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    response_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mernis_log_org_at ON mernis_verification_log(organization_id, requested_at DESC);

-- ---------- Medula provision ----------

CREATE TYPE medula_provision_status AS ENUM (
    'pending',         -- outbox'a düştü, worker'ı bekliyor
    'in_progress',     -- worker SOAP çağrısı yapıyor
    'completed',       -- başarıyla provizyon alındı (takip_no var)
    'failed',          -- SGK reddi (response code var)
    'cancelled'        -- iptal edildi
);

CREATE TABLE medula_provision (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    patient_id      UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    institution_id  UUID REFERENCES external_institution(id) ON DELETE SET NULL,

    -- SGK takip numarası (başarı durumunda gelir).
    takip_no        TEXT,
    provision_type  TEXT NOT NULL DEFAULT 'normal',  -- normal | acil | yatis
    branch_code     TEXT,                            -- SGK branş kodu

    status          medula_provision_status NOT NULL DEFAULT 'pending',

    -- SGK request / response payload (XML or JSON). In real impl these would
    -- be encrypted at rest.
    request_payload  JSONB NOT NULL DEFAULT '{}'::JSONB,
    response_payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    response_code    TEXT,
    error_message    TEXT,

    requested_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    requested_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_medula_provision_branch_status ON medula_provision(branch_id, status, requested_at DESC);
CREATE INDEX idx_medula_provision_patient ON medula_provision(patient_id, requested_at DESC);

CREATE TRIGGER trg_medula_provision_updated_at BEFORE UPDATE ON medula_provision
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Medula outbox ----------
--
-- One row per outgoing SGK message. The worker selects FOR UPDATE SKIP
-- LOCKED to allow horizontal worker scaling. retry_count + next_retry_at
-- drive exponential backoff; status 'dead' is the terminal failure state
-- after max retries.

CREATE TYPE medula_outbox_status AS ENUM (
    'pending',
    'in_progress',
    'sent',
    'failed',
    'dead'
);

CREATE TABLE medula_outgoing_message (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID REFERENCES branch(id) ON DELETE SET NULL,
    message_type    TEXT NOT NULL,             -- provision_request | invoice_submit | ...
    -- Foreign key to whichever domain row drives this message (e.g.
    -- medula_provision.id when message_type='provision_request').
    target_table    TEXT NOT NULL,
    target_id       UUID NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}'::JSONB,
    status          medula_outbox_status NOT NULL DEFAULT 'pending',
    retry_count     INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    next_retry_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error      TEXT,
    sent_at         TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbox_pending ON medula_outgoing_message(status, next_retry_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX idx_outbox_target ON medula_outgoing_message(target_table, target_id);

CREATE TRIGGER trg_outbox_updated_at BEFORE UPDATE ON medula_outgoing_message
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
