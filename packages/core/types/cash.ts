import type { Uuid } from "./common";

export type CashRegisterStatus = "open" | "closed";

export type PaymentMethod = "cash" | "card" | "transfer" | "mobile" | "other";

export type CashMovementKind =
  | "opening"
  | "income"
  | "expense"
  | "refund"
  | "closing"
  | "transfer_in"
  | "transfer_out";

export type CashRegister = {
  id: Uuid;
  register_no: string;
  cashier_user_id: Uuid;
  cashier_name: string;
  status: CashRegisterStatus;
  opening_balance: number;
  declared_balance?: number;
  notes?: string;
  opened_at: string;
  closed_at?: string;
};

export type CashMovement = {
  id: Uuid;
  movement_no: string;
  kind: CashMovementKind;
  method: PaymentMethod;
  amount: number;
  reference_type?: string;
  reference_id?: string;
  counterparty?: string;
  description?: string;
  performed_at: string;
};

export type ZReportRow = {
  kind: CashMovementKind;
  method: PaymentMethod;
  total: number;
  count: number;
};

export type ZReport = {
  register: CashRegister;
  movements: CashMovement[];
  by_kind_method: ZReportRow[];
  total_income: number;
  total_expense: number;
  total_refund: number;
  expected_close: number;
  variance?: number;
};
