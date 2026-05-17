-- 028_hl7_adt_outbound: outbox for HL7 v2 ADT messages.
--
-- ADT (Admission, Discharge, Transfer) is the standard HL7 envelope HBYS
-- systems use to notify downstream consumers (PACS, lab LIS, billing,
-- regional HIE) about patient encounters. We persist every generated
-- message + dispatch attempt for replay + audit. Worker drains pending
-- rows; mock dispatcher logs (no network), real dispatcher will send
-- over MLLP / TCP socket once a peer is configured.

BEGIN;

CREATE TYPE hl7_adt_event AS ENUM (
    'A01',  -- Admit / visit notification
    'A02',  -- Transfer (yatak / servis değişimi)
    'A03',  -- Discharge / taburcu
    'A04',  -- Register a patient (outpatient kabul)
    'A08'   -- Update patient information
);

CREATE TYPE hl7_outbound_status AS ENUM (
    'pending',
    'in_flight',
    'sent',
    'failed',
    'dead'
);

CREATE TABLE hl7_outbound_message (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id         UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    -- HL7 message control id (MSH-10) — UUID-derived; globally unique.
    message_control_id TEXT NOT NULL UNIQUE,
    event_type        hl7_adt_event NOT NULL,
    patient_id        UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    admission_id      UUID REFERENCES admission(id) ON DELETE SET NULL,
    -- Pre-formatted HL7 pipe-bar message (wire format). Generated at
    -- enqueue time so retries replay the exact bytes the event captured.
    raw_message       TEXT NOT NULL,
    status            hl7_outbound_status NOT NULL DEFAULT 'pending',
    retry_count       INTEGER NOT NULL DEFAULT 0,
    next_retry_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error        TEXT,
    -- ACK message body (MSA segment) when the peer accepts.
    ack_raw           TEXT,
    sent_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hl7_outbound_due ON hl7_outbound_message(next_retry_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX idx_hl7_outbound_branch_status ON hl7_outbound_message(branch_id, status, created_at DESC);
CREATE INDEX idx_hl7_outbound_admission ON hl7_outbound_message(admission_id) WHERE admission_id IS NOT NULL;

CREATE TRIGGER trg_hl7_outbound_updated_at BEFORE UPDATE ON hl7_outbound_message
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
