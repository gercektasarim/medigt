import type { Uuid } from "./common";

export type WarehouseKind =
  | "pharmacy"
  | "general"
  | "central"
  | "ward"
  | "operating_room"
  | "other";

export type StockMovementKind =
  | "receive"
  | "issue"
  | "transfer_out"
  | "transfer_in"
  | "adjust"
  | "expire"
  | "return";

export type Warehouse = {
  id: Uuid;
  code: string;
  name: string;
  kind: WarehouseKind;
  location?: string;
};

export type StockRow = {
  stock_id: Uuid;
  warehouse_id: Uuid;
  warehouse_code: string;
  warehouse_name: string;
  medication_id: Uuid;
  medication_name: string;
  generic_name?: string;
  form: string;
  strength?: string;
  lot_no: string;
  expiry_date?: string; // YYYY-MM-DD
  quantity: number;
  last_movement_at: string;
};

export type StockMovement = {
  id: Uuid;
  movement_no: string;
  warehouse_id: Uuid;
  warehouse_code: string;
  warehouse_name: string;
  medication_id: Uuid;
  medication_name: string;
  lot_no: string;
  expiry_date?: string;
  kind: StockMovementKind;
  quantity: number;
  unit_price?: number;
  reference_type?: string;
  reference_id?: string;
  counterparty?: string;
  notes?: string;
  performed_at: string;
};
