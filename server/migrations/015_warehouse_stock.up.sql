-- 015_warehouse_stock: branch-scoped warehouses + per-lot medication stock +
-- audit-trail of stock movements.
--
-- medication_stock is the live inventory denormalisation (one row per
-- warehouse × medication × lot × expiry). stock_movement is the append-only
-- audit log; every stock change must insert a movement row in the same
-- transaction that updates the live row.
--
-- FIFO/FEFO logic lives in service-layer code (find lots with earliest
-- expiry, decrement until the requested quantity is satisfied).

BEGIN;

CREATE TYPE warehouse_kind AS ENUM (
    'pharmacy',         -- eczane deposu
    'general',          -- genel depo
    'central',          -- merkez depo
    'ward',             -- servis dolabı
    'operating_room',   -- ameliyathane stok
    'other'
);

CREATE TYPE stock_movement_kind AS ENUM (
    'receive',          -- giriş (irsaliye / mal alımı)
    'issue',            -- çıkış (reçete / hasta verme)
    'transfer_out',     -- başka depoya çıkış
    'transfer_in',      -- başka depodan giriş
    'adjust',           -- manuel düzeltme
    'expire',           -- son kullanım tarihi geçti
    'return'            -- iade
);

-- ---------- Warehouse ----------

CREATE TABLE warehouse (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    code            TEXT NOT NULL,
    name            TEXT NOT NULL,
    kind            warehouse_kind NOT NULL DEFAULT 'pharmacy',
    location        TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (branch_id, code)
);

CREATE INDEX idx_warehouse_branch ON warehouse(branch_id, is_active);

CREATE TRIGGER trg_warehouse_updated_at BEFORE UPDATE ON warehouse
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Live stock (per warehouse × medication × lot × expiry) ----------

CREATE TABLE medication_stock (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    warehouse_id    UUID NOT NULL REFERENCES warehouse(id) ON DELETE RESTRICT,
    medication_id   UUID NOT NULL REFERENCES medication(id) ON DELETE RESTRICT,
    lot_no          TEXT NOT NULL DEFAULT '',
    expiry_date     DATE,                            -- NULL allowed for non-dated items
    quantity        NUMERIC(14,3) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    last_movement_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- NULLS NOT DISTINCT (PG 15+): two NULL expiry_date rows for the same
    -- warehouse+medication+lot collapse into one bucket so ON CONFLICT works.
    UNIQUE NULLS NOT DISTINCT (warehouse_id, medication_id, lot_no, expiry_date)
);

CREATE INDEX idx_stock_warehouse_med ON medication_stock(warehouse_id, medication_id);
CREATE INDEX idx_stock_branch_med ON medication_stock(branch_id, medication_id);
CREATE INDEX idx_stock_expiry ON medication_stock(branch_id, expiry_date) WHERE expiry_date IS NOT NULL AND quantity > 0;

CREATE TRIGGER trg_medication_stock_updated_at BEFORE UPDATE ON medication_stock
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Stock movements (append-only audit + actuals) ----------

CREATE SEQUENCE stock_movement_no_seq START 100000 INCREMENT 1;

CREATE TABLE stock_movement (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    movement_no     TEXT NOT NULL,
    warehouse_id    UUID NOT NULL REFERENCES warehouse(id) ON DELETE RESTRICT,
    medication_id   UUID NOT NULL REFERENCES medication(id) ON DELETE RESTRICT,
    lot_no          TEXT NOT NULL DEFAULT '',
    expiry_date     DATE,
    kind            stock_movement_kind NOT NULL,
    quantity        NUMERIC(14,3) NOT NULL CHECK (quantity > 0),  -- always positive; kind determines direction
    unit_price      NUMERIC(12,4),  -- for receive (alış birim fiyatı)
    -- Free-form linkage to source document (purchase_order, prescription, vs.)
    reference_type  TEXT,
    reference_id    UUID,
    counterparty    TEXT,            -- tedarikçi adı / hasta adı (snapshot)
    notes           TEXT,
    performed_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    performed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, movement_no)
);

CREATE INDEX idx_movement_warehouse_med ON stock_movement(warehouse_id, medication_id, performed_at DESC);
CREATE INDEX idx_movement_branch_kind ON stock_movement(branch_id, kind, performed_at DESC);
CREATE INDEX idx_movement_reference ON stock_movement(reference_type, reference_id) WHERE reference_id IS NOT NULL;

COMMIT;
