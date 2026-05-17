import type { Uuid } from "./common";
import type { PaymentMethod } from "./cash";

// ---------- Cari hesap (avans) ----------

export type PatientAccountEntryKind =
  | "advance_in"
  | "advance_use"
  | "advance_refund"
  | "refund_to_advance";

export type PatientAccountEntry = {
  id: Uuid;
  kind: PatientAccountEntryKind;
  amount: number;
  direction: 1 | -1;
  signed_amount: number;
  payment_id?: Uuid;
  invoice_id?: Uuid;
  notes?: string;
  performed_at: string;
};

export type PatientAccountSummary = {
  patient_id: Uuid;
  balance: number;
  entries: PatientAccountEntry[];
};

// ---------- Refund ----------

export type Refund = {
  id: Uuid;
  refund_no: string;
  patient_id: Uuid;
  patient_mrn?: string;
  patient_first_name?: string;
  patient_last_name?: string;
  invoice_id?: Uuid;
  invoice_no?: string;
  payment_id?: Uuid;
  amount: number;
  method: PaymentMethod;
  cash_register_id?: Uuid;
  to_advance: boolean;
  reason?: string;
  performed_at: string;
};

// ---------- Installment ----------

export type InstallmentStatus = "pending" | "paid" | "partial" | "overdue" | "cancelled";
export type InstallmentPlanStatus = "active" | "completed" | "cancelled";

export type Installment = {
  id: Uuid;
  seq: number;
  due_date: string;
  amount: number;
  paid_amount: number;
  status: InstallmentStatus;
  paid_at?: string;
  payment_id?: Uuid;
  notes?: string;
};

export type InstallmentPlan = {
  id: Uuid;
  invoice_id: Uuid;
  total_amount: number;
  num_installments: number;
  status: InstallmentPlanStatus;
  notes?: string;
  created_at: string;
  installments: Installment[];
};

export type UpcomingInstallment = {
  installment_id: Uuid;
  plan_id: Uuid;
  seq: number;
  due_date: string;
  amount: number;
  paid_amount: number;
  status: InstallmentStatus;
  invoice_id: Uuid;
  invoice_no: string;
  patient_id: Uuid;
  patient_mrn: string;
  patient_first_name: string;
  patient_last_name: string;
};
