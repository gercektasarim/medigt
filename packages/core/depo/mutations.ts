import { api } from "../api/client";
import type { Warehouse, WarehouseKind } from "../types/stock";

export type CreateWarehouseInput = {
  code: string;
  name: string;
  kind?: WarehouseKind;
  location?: string;
  notes?: string;
};

export function createWarehouse(input: CreateWarehouseInput): Promise<Warehouse> {
  return api().post<Warehouse>("/api/warehouses", input);
}

export type ReceiveStockInput = {
  warehouse_id: string;
  medication_id: string;
  lot_no: string;
  expiry_date?: string; // YYYY-MM-DD
  quantity: number;
  unit_price?: number;
  counterparty?: string;
  notes?: string;
};

export function receiveStock(input: ReceiveStockInput): Promise<{ movement_no: string }> {
  return api().post<{ movement_no: string }>("/api/stock-movements/receive", input);
}

export type AdjustStockInput = {
  warehouse_id: string;
  medication_id: string;
  lot_no: string;
  expiry_date?: string;
  new_quantity: number;
  notes?: string;
};

export function adjustStock(input: AdjustStockInput): Promise<{ movement_no: string }> {
  return api().post<{ movement_no: string }>("/api/stock-movements/adjust", input);
}
