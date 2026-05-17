-- 027_enabiz: outbox + receipt log for e-Nabız (Sağlık Bakanlığı PHR).
--
-- Every clinical event we want pushed to e-Nabız becomes a row here in
-- `pending` state. The worker drains them with exponential backoff
-- (30s → 2m → 10m → 1h → 6h), giving up at retry_count=5 (status=dead).
-- On success we keep the receipt_id so reconciliation reports can match
-- our visit_id back to Bakanlık's record.
--
-- This is the SAME pattern as medula_outgoing_message; we keep it
-- separate so e-Nabız outages don't drag SGK provisioning down (and
-- vice versa) — the workers are independent.

BEGIN;

CREATE TYPE enabiz_message_status AS ENUM (
    'pending',   -- waiting for first attempt
    'in_flight', -- currently being processed
    'sent',      -- delivered + receipt received
    'failed',    -- last attempt errored, will retry
    'dead'       -- retry budget exhausted, admin attention needed
);

CREATE TYPE enabiz_resource_kind AS ENUM (
    'Encounter',          -- muayene / yatış
    'Observation',        -- vital signs, lab result
    'Condition',          -- ICD-10 tanı
    'MedicationRequest',  -- reçete
    'DiagnosticReport'    -- lab + radyoloji raporu
);

CREATE TABLE enabiz_message (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id         UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    patient_id        UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    -- patient TC at insert time — KVKK requires last-4 only in audit_log,
    -- but the outbox needs the full TC to address Bakanlık by national
    -- identifier. The column lives behind FIELD_ENCRYPTION_KEY at rest
    -- (encryption-at-rest happens at the DB layer, not in app code).
    patient_tc        TEXT NOT NULL,

    kind              enabiz_resource_kind NOT NULL,
    -- The FHIR resource serialized as JSON. We store it pre-formatted so
    -- retries don't need to re-derive it from upstream tables (which may
    -- have shifted in the meantime).
    resource_json     JSONB NOT NULL,

    -- What in our DB this message refers to (e.g. visit_id, lab_order_id).
    -- Used for reconciliation reports.
    source_table      TEXT,
    source_id         UUID,

    status            enabiz_message_status NOT NULL DEFAULT 'pending',
    retry_count       INTEGER NOT NULL DEFAULT 0,
    next_retry_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error        TEXT,

    -- Bakanlık'tan dönen iz: receipt_id + son raw cevap.
    receipt_id        TEXT,
    last_response     JSONB,

    queued_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Hot path: the worker polls "pending and ready to retry, ordered by
-- next_retry_at". FOR UPDATE SKIP LOCKED in the worker query stays
-- O(N) on this index.
CREATE INDEX idx_enabiz_pending_due ON enabiz_message(next_retry_at)
    WHERE status IN ('pending', 'failed');

CREATE INDEX idx_enabiz_branch_status ON enabiz_message(branch_id, status, created_at DESC);
CREATE INDEX idx_enabiz_patient ON enabiz_message(patient_id, created_at DESC);
CREATE INDEX idx_enabiz_source ON enabiz_message(source_table, source_id)
    WHERE source_id IS NOT NULL;

CREATE TRIGGER trg_enabiz_message_updated_at BEFORE UPDATE ON enabiz_message
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
