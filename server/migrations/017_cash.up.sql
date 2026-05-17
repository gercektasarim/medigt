-- 017_cash: cash register sessions + per-session movement log.
--
-- One cashier may have at most one open register at a time (enforced by
-- partial UNIQUE index). Every income / expense / refund inserts a row in
-- cash_movement; the expected closing balance is opening + sum(income -
-- expense - refund). On close, the cashier counts cash and enters the
-- declared_balance; variance is auto-computed.
--
-- The payment + invoice tables that build on top of this come in slice 2.

BEGIN;

CREATE TYPE cash_register_status AS ENUM (
    'open',
    'closed'
);

CREATE TYPE payment_method AS ENUM (
    'cash',         -- nakit
    'card',         -- kredi / banka kartı
    'transfer',     -- havale / EFT
    'mobile',       -- mobil ödeme
    'other'         -- mahsup, avans, kupon, vb.
);

CREATE TYPE cash_movement_kind AS ENUM (
    'opening',      -- açılış bakiyesi
    'income',       -- tahsilat
    'expense',      -- gider
    'refund',       -- iade
    'closing',      -- kapanış kaydı (audit)
    'transfer_in',  -- başka kasadan
    'transfer_out'  -- başka kasaya
);

-- ---------- Cash register (kasa oturumu) ----------

CREATE SEQUENCE cash_register_no_seq START 1000 INCREMENT 1;

CREATE TABLE cash_register (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    register_no     TEXT NOT NULL,
    cashier_user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    cashier_name    TEXT NOT NULL,             -- snapshot at open time
    status          cash_register_status NOT NULL DEFAULT 'open',

    opening_balance NUMERIC(14,2) NOT NULL DEFAULT 0,
    declared_balance NUMERIC(14,2),            -- countted on close
    -- Variance is computed from declared - expected; we don't store the
    -- expected because it's a sum over the movement table — Z report
    -- query computes it on read.

    notes           TEXT,

    opened_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, register_no)
);

-- At most one open register per cashier.
CREATE UNIQUE INDEX idx_cash_one_open_per_user ON cash_register(cashier_user_id)
    WHERE status = 'open';

CREATE INDEX idx_cash_branch_status ON cash_register(branch_id, status, opened_at DESC);

CREATE TRIGGER trg_cash_register_updated_at BEFORE UPDATE ON cash_register
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Cash movement (per-session audit log) ----------

CREATE SEQUENCE cash_movement_no_seq START 100000 INCREMENT 1;

CREATE TABLE cash_movement (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    cash_register_id UUID NOT NULL REFERENCES cash_register(id) ON DELETE RESTRICT,
    movement_no     TEXT NOT NULL,
    kind            cash_movement_kind NOT NULL,
    method          payment_method NOT NULL,
    amount          NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    -- Source-document linkage (invoice_id, payment_id, refund_id, etc.)
    reference_type  TEXT,
    reference_id    UUID,
    counterparty    TEXT,         -- hasta adı / tedarikçi / personel (snapshot)
    description     TEXT,
    performed_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    performed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, movement_no)
);

CREATE INDEX idx_cash_movement_register ON cash_movement(cash_register_id, performed_at);
CREATE INDEX idx_cash_movement_branch_kind ON cash_movement(branch_id, kind, performed_at DESC);
CREATE INDEX idx_cash_movement_reference ON cash_movement(reference_type, reference_id) WHERE reference_id IS NOT NULL;

COMMIT;
