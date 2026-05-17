import { api } from "../api/client";

export type DispenseInput = {
  medication_id: string;
  warehouse_id: string;
  lot_no: string;
  expiry_date?: string; // YYYY-MM-DD
  quantity: number;
  counterparty?: string;
};

export type DispenseResult = {
  dispense_id: string;
  movement_no: string;
};

export function dispenseItem(itemId: string, input: DispenseInput): Promise<DispenseResult> {
  return api().post<DispenseResult>(
    `/api/prescription-items/${encodeURIComponent(itemId)}/dispense`,
    input,
  );
}
