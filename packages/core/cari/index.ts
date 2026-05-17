"use client";

import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  InstallmentPlan,
  PatientAccountSummary,
  Refund,
  UpcomingInstallment,
} from "../types";
import type { PaymentMethod } from "../types/cash";

export const cariKeys = {
  patientAccount: (patientId: string) => ["cari", "patient", patientId] as const,
  refunds: (branchId: string) => ["cari", "refunds", branchId] as const,
  invoiceRefunds: (invoiceId: string) => ["cari", "invoice-refunds", invoiceId] as const,
  installmentPlan: (invoiceId: string) => ["cari", "installment-plan", invoiceId] as const,
  upcoming: (branchId: string, through: string) =>
    ["cari", "upcoming", branchId, through] as const,
};

// ---------- Patient account (avans) ----------

export function patientAccountOptions(patientId: string) {
  return queryOptions({
    queryKey: cariKeys.patientAccount(patientId),
    queryFn: () =>
      api().get<PatientAccountSummary>(`/api/patients/${encodeURIComponent(patientId)}/cari`),
    enabled: !!patientId,
  });
}

export type ReceiveAdvanceInput = {
  amount: number;
  method: PaymentMethod;
  cash_register_id?: string;
  notes?: string;
};

export function receiveAdvance(patientId: string, input: ReceiveAdvanceInput): Promise<{ entry_id: string }> {
  return api().post<{ entry_id: string }>(
    `/api/patients/${encodeURIComponent(patientId)}/advance`,
    input,
  );
}

export type ApplyAdvanceInput = {
  patient_id: string;
  invoice_id: string;
  amount: number;
  notes?: string;
};

export function applyAdvance(input: ApplyAdvanceInput): Promise<{ payment_id: string }> {
  return api().post<{ payment_id: string }>("/api/advances/apply", input);
}

export type RefundAdvanceInput = {
  amount: number;
  cash_register_id: string;
  reason?: string;
};

export function refundAdvance(patientId: string, input: RefundAdvanceInput): Promise<{ entry_id: string }> {
  return api().post<{ entry_id: string }>(
    `/api/patients/${encodeURIComponent(patientId)}/advance-refund`,
    input,
  );
}

// ---------- Refund (fatura iadesi) ----------

export function refundListOptions(branchId: string) {
  return queryOptions({
    queryKey: cariKeys.refunds(branchId),
    queryFn: () => api().get<Refund[]>("/api/refunds"),
    enabled: !!branchId,
  });
}

export function invoiceRefundsOptions(invoiceId: string) {
  return queryOptions({
    queryKey: cariKeys.invoiceRefunds(invoiceId),
    queryFn: () => api().get<Refund[]>(`/api/invoices/${encodeURIComponent(invoiceId)}/refunds`),
    enabled: !!invoiceId,
  });
}

export type ProcessRefundInput = {
  patient_id: string;
  invoice_id?: string;
  payment_id?: string;
  amount: number;
  method: PaymentMethod;
  cash_register_id?: string;
  to_advance?: boolean;
  reason?: string;
};

export function processRefund(input: ProcessRefundInput): Promise<{
  refund_id: string;
  refund_no: string;
  cash_movement_id?: string;
}> {
  return api().post("/api/refunds", input);
}

// ---------- Installment plan ----------

export function installmentPlanOptions(invoiceId: string) {
  return queryOptions({
    queryKey: cariKeys.installmentPlan(invoiceId),
    queryFn: () =>
      api().get<InstallmentPlan | null>(`/api/invoices/${encodeURIComponent(invoiceId)}/installment-plan`),
    enabled: !!invoiceId,
  });
}

export function upcomingInstallmentsOptions(branchId: string, throughDate?: string) {
  const through = throughDate ?? "";
  return queryOptions({
    queryKey: cariKeys.upcoming(branchId, through),
    queryFn: () => {
      const qs = through ? `?through=${encodeURIComponent(through)}` : "";
      return api().get<UpcomingInstallment[]>(`/api/installments/upcoming${qs}`);
    },
    enabled: !!branchId,
  });
}

export type CreateInstallmentPlanInput = {
  num_installments: number;
  first_due_date?: string; // YYYY-MM-DD
  interval_days?: number;
  notes?: string;
};

export function createInstallmentPlan(invoiceId: string, input: CreateInstallmentPlanInput): Promise<{ plan_id: string }> {
  return api().post<{ plan_id: string }>(
    `/api/invoices/${encodeURIComponent(invoiceId)}/installment-plan`,
    input,
  );
}
