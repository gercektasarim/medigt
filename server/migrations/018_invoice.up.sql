-- 018_invoice: invoices + line items + payment ledger with multi-invoice
-- allocations.
--
-- An invoice has a status machine: draft → finalized → paid (terminal) or
-- cancelled. Total / paid_total are denormalised on the invoice row and
-- maintained transactionally by the service layer (payment alloc / refund);
-- the audit truth lives in payment_allocation rows.

BEGIN;

CREATE TYPE invoice_status AS ENUM (
    'draft',
    'finalized',
    'paid',
    'cancelled'
);

-- ---------- Invoice header ----------

CREATE SEQUENCE invoice_no_seq START 100000 INCREMENT 1;

CREATE TABLE invoice (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    invoice_no      TEXT NOT NULL,

    patient_id      UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    -- Optional payer institution (SGK, özel sigorta, kurum); NULL = self-pay.
    institution_id  UUID REFERENCES external_institution(id) ON DELETE SET NULL,
    -- Optional links to source documents.
    visit_id        UUID REFERENCES visit(id) ON DELETE SET NULL,
    admission_id    UUID REFERENCES admission(id) ON DELETE SET NULL,

    status          invoice_status NOT NULL DEFAULT 'draft',

    -- Totals (denormalised, maintained by service layer)
    subtotal        NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    discount_total  NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (discount_total >= 0),
    tax_total       NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (tax_total >= 0),
    total           NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (total >= 0),
    paid_total      NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (paid_total >= 0),
    -- Convenience computed column (subject to floating-point variance — read
    -- only, callers can also derive locally).
    balance_due     NUMERIC(14,2) GENERATED ALWAYS AS (total - paid_total) STORED,

    issued_at       TIMESTAMPTZ,
    cancelled_at    TIMESTAMPTZ,
    notes           TEXT,

    created_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, invoice_no)
);

CREATE INDEX idx_invoice_branch_status ON invoice(branch_id, status, created_at DESC);
CREATE INDEX idx_invoice_patient ON invoice(patient_id, created_at DESC);
CREATE INDEX idx_invoice_institution ON invoice(institution_id) WHERE institution_id IS NOT NULL;

CREATE TRIGGER trg_invoice_updated_at BEFORE UPDATE ON invoice
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Invoice line item ----------

CREATE TABLE invoice_item (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id      UUID NOT NULL REFERENCES invoice(id) ON DELETE CASCADE,
    -- Snapshot of catalog row at issue time (so historical reads are stable).
    service_id      UUID REFERENCES service_catalog(id) ON DELETE SET NULL,
    code            TEXT NOT NULL,        -- snapshot code
    name            TEXT NOT NULL,        -- snapshot name
    -- Optional cross-links to clinical sources (for traceability).
    visit_id        UUID REFERENCES visit(id) ON DELETE SET NULL,
    lab_order_id    UUID REFERENCES lab_order(id) ON DELETE SET NULL,
    radiology_order_id UUID REFERENCES radiology_order(id) ON DELETE SET NULL,
    surgery_id      UUID REFERENCES surgery(id) ON DELETE SET NULL,
    -- Performing doctor for hakediş later.
    doctor_id       UUID REFERENCES doctor(id) ON DELETE SET NULL,

    quantity        NUMERIC(10,3) NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price      NUMERIC(14,4) NOT NULL CHECK (unit_price >= 0),
    discount_pct    NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (discount_pct >= 0 AND discount_pct <= 100),
    vat_rate        NUMERIC(5,2) NOT NULL DEFAULT 10 CHECK (vat_rate >= 0 AND vat_rate <= 100),
    -- Pre-tax line (computed in service): qty * unit_price * (1 - discount/100)
    line_subtotal   NUMERIC(14,2) NOT NULL,
    line_tax        NUMERIC(14,2) NOT NULL,
    line_total      NUMERIC(14,2) NOT NULL,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoice_item_invoice ON invoice_item(invoice_id, sort_order);
CREATE INDEX idx_invoice_item_service ON invoice_item(service_id) WHERE service_id IS NOT NULL;
CREATE INDEX idx_invoice_item_doctor ON invoice_item(doctor_id) WHERE doctor_id IS NOT NULL;
CREATE INDEX idx_invoice_item_visit ON invoice_item(visit_id) WHERE visit_id IS NOT NULL;

-- ---------- Payment + allocation ----------

CREATE SEQUENCE payment_no_seq START 100000 INCREMENT 1;

CREATE TABLE payment (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    payment_no      TEXT NOT NULL,
    patient_id      UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    method          payment_method NOT NULL,
    amount          NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    -- Optional cash register session if method='cash' (so kasa Z report
    -- includes the payment).
    cash_register_id UUID REFERENCES cash_register(id) ON DELETE SET NULL,
    -- Audit-trail back to the cash_movement row created when method='cash'.
    cash_movement_id UUID REFERENCES cash_movement(id) ON DELETE SET NULL,
    reference       TEXT,         -- pos auth code, havale ref, etc.
    notes           TEXT,
    received_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, payment_no)
);

CREATE INDEX idx_payment_branch_at ON payment(branch_id, received_at DESC);
CREATE INDEX idx_payment_patient ON payment(patient_id, received_at DESC);
CREATE INDEX idx_payment_cash_register ON payment(cash_register_id) WHERE cash_register_id IS NOT NULL;

-- A payment can cover multiple invoices (taksitli ödeme / mahsuplaşma);
-- an invoice can be paid by multiple payments (kısmi ödeme).
CREATE TABLE payment_allocation (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    payment_id      UUID NOT NULL REFERENCES payment(id) ON DELETE CASCADE,
    invoice_id      UUID NOT NULL REFERENCES invoice(id) ON DELETE RESTRICT,
    amount          NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (payment_id, invoice_id)
);

CREATE INDEX idx_alloc_invoice ON payment_allocation(invoice_id);
CREATE INDEX idx_alloc_payment ON payment_allocation(payment_id);

COMMIT;
