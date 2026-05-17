import { api } from "../api/client";
import type { PaymentMethod } from "../types/cash";

export type CreateInvoiceItemInput = {
  service_id?: string;
  code: string;
  name: string;
  visit_id?: string;
  lab_order_id?: string;
  radiology_order_id?: string;
  surgery_id?: string;
  doctor_id?: string;
  quantity: number;
  unit_price: number;
  discount_pct?: number;
  vat_rate?: number;
  notes?: string;
};

export type CreateInvoiceInput = {
  patient_id: string;
  institution_id?: string;
  visit_id?: string;
  admission_id?: string;
  notes?: string;
  items: CreateInvoiceItemInput[];
  finalize?: boolean;
};

export function createInvoice(input: CreateInvoiceInput): Promise<{ id: string; invoice_no: string }> {
  return api().post<{ id: string; invoice_no: string }>("/api/invoices", input);
}

export function finalizeInvoice(id: string): Promise<{ ok: boolean }> {
  return api().post<{ ok: boolean }>(`/api/invoices/${encodeURIComponent(id)}/finalize`, {});
}

export function cancelInvoice(id: string, reason?: string): Promise<{ ok: boolean }> {
  return api().post<{ ok: boolean }>(`/api/invoices/${encodeURIComponent(id)}/cancel`, { reason });
}

export type PaymentAllocationInput = {
  invoice_id: string;
  amount: number;
};

export type RecordPaymentInput = {
  patient_id: string;
  method: PaymentMethod;
  amount: number;
  reference?: string;
  notes?: string;
  cash_register_id?: string; // required if method='cash'
  allocations: PaymentAllocationInput[];
};

export function recordPayment(input: RecordPaymentInput): Promise<{
  payment_id: string;
  payment_no: string;
  cash_movement_no?: string;
}> {
  return api().post("/api/payments", input);
}
