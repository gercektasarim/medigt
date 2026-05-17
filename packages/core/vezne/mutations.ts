import { api } from "../api/client";
import type { CashMovementKind, PaymentMethod } from "../types/cash";

export type OpenRegisterInput = {
  opening_balance: number;
  notes?: string;
};

export function openRegister(input: OpenRegisterInput): Promise<{ id: string; register_no: string }> {
  return api().post<{ id: string; register_no: string }>("/api/cash-registers", input);
}

export type CloseRegisterInput = {
  declared_balance: number;
  notes?: string;
};

export function closeRegister(id: string, input: CloseRegisterInput): Promise<{ ok: boolean }> {
  return api().post<{ ok: boolean }>(
    `/api/cash-registers/${encodeURIComponent(id)}/close`,
    input,
  );
}

export type RecordMovementInput = {
  kind: CashMovementKind;
  method: PaymentMethod;
  amount: number;
  counterparty?: string;
  description?: string;
};

export function recordMovement(
  registerId: string,
  input: RecordMovementInput,
): Promise<{ movement_no: string }> {
  return api().post<{ movement_no: string }>(
    `/api/cash-registers/${encodeURIComponent(registerId)}/movements`,
    input,
  );
}
