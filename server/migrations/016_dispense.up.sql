-- 016_dispense: link prescriptions to the medication catalog and record per-lot
-- dispensations that decrement live stock through stock_movement.

BEGIN;

-- 1. Extend prescription_item with optional medication_id (eczane links the
--    doctor's free-text medication_name to a catalog row at dispense time)
--    and the canonical numeric dispense_quantity (units, not free-text).

ALTER TABLE prescription_item
    ADD COLUMN medication_id UUID REFERENCES medication(id) ON DELETE SET NULL,
    ADD COLUMN dispense_quantity NUMERIC(14,3);

CREATE INDEX idx_prescription_item_medication ON prescription_item(medication_id)
    WHERE medication_id IS NOT NULL;

-- 2. prescription_dispense — append-only ledger of per-lot dispensations.
--    Linked to stock_movement so every dispense is auditable end-to-end.

CREATE TABLE prescription_dispense (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id             UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    prescription_item_id  UUID NOT NULL REFERENCES prescription_item(id) ON DELETE CASCADE,
    warehouse_id          UUID NOT NULL REFERENCES warehouse(id) ON DELETE RESTRICT,
    medication_id         UUID NOT NULL REFERENCES medication(id) ON DELETE RESTRICT,
    lot_no                TEXT NOT NULL DEFAULT '',
    expiry_date           DATE,
    quantity              NUMERIC(14,3) NOT NULL CHECK (quantity > 0),
    stock_movement_id     UUID NOT NULL REFERENCES stock_movement(id) ON DELETE RESTRICT,
    dispensed_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dispensed_by_user_id  UUID REFERENCES app_user(id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dispense_item ON prescription_dispense(prescription_item_id);
CREATE INDEX idx_dispense_branch_at ON prescription_dispense(branch_id, dispensed_at DESC);

COMMIT;
