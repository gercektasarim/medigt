import type { Uuid } from "./common";
import type { PaymentMethod } from "./cash";

export type InvoiceStatus = "draft" | "finalized" | "paid" | "cancelled";

export type InvoiceItem = {
  id: Uuid;
  service_id?: Uuid;
  code: string;
  name: string;
  visit_id?: Uuid;
  lab_order_id?: Uuid;
  radiology_order_id?: Uuid;
  surgery_id?: Uuid;
  doctor_id?: Uuid;
  quantity: number;
  unit_price: number;
  discount_pct: number;
  vat_rate: number;
  line_subtotal: number;
  line_tax: number;
  line_total: number;
  sort_order: number;
  notes?: string;
};

export type Invoice = {
  id: Uuid;
  invoice_no: string;
  status: InvoiceStatus;
  patient_id: Uuid;
  patient_mrn?: string;
  patient_first_name?: string;
  patient_last_name?: string;
  institution_id?: Uuid;
  institution_name?: string;
  visit_id?: Uuid;
  admission_id?: Uuid;
  subtotal: number;
  discount_total: number;
  tax_total: number;
  total: number;
  paid_total: number;
  balance_due: number;
  issued_at?: string;
  cancelled_at?: string;
  notes?: string;
  created_at: string;
  updated_at: string;
};

export type InvoiceDetail = {
  invoice: Invoice;
  items: InvoiceItem[];
};

export type InvoicePaymentSummary = {
  id: Uuid;
  payment_no: string;
  method: PaymentMethod;
  amount: number;
  allocated_to_this_invoice: number;
  reference?: string;
  received_at: string;
};
