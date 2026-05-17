import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  Invoice,
  InvoiceDetail,
  InvoicePaymentSummary,
  InvoiceStatus,
} from "../types/invoice";

export const faturaKeys = {
  all: (branchId: string) => ["fatura", branchId] as const,
  list: (
    branchId: string,
    status: string,
    patientId: string,
    from: string,
    to: string,
    unpaid: boolean,
  ) =>
    [...faturaKeys.all(branchId), "list", status, patientId, from, to, unpaid] as const,
  detail: (branchId: string, id: string) => [...faturaKeys.all(branchId), "detail", id] as const,
  payments: (branchId: string, id: string) => [...faturaKeys.all(branchId), "payments", id] as const,
};

export type InvoiceListFilter = {
  status?: InvoiceStatus;
  patientId?: string;
  from?: string;
  to?: string;
  onlyUnpaid?: boolean;
};

export function invoiceListOptions(branchId: string, f: InvoiceListFilter = {}) {
  return queryOptions({
    queryKey: faturaKeys.list(
      branchId,
      f.status ?? "",
      f.patientId ?? "",
      f.from ?? "",
      f.to ?? "",
      f.onlyUnpaid ?? false,
    ),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.status) params.set("status", f.status);
      if (f.patientId) params.set("patient_id", f.patientId);
      if (f.from) params.set("from", f.from);
      if (f.to) params.set("to", f.to);
      if (f.onlyUnpaid) params.set("unpaid", "true");
      const qs = params.toString();
      return api().get<Invoice[]>(`/api/invoices${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}

export function invoiceDetailOptions(branchId: string, id: string) {
  return queryOptions({
    queryKey: faturaKeys.detail(branchId, id),
    queryFn: () => api().get<InvoiceDetail>(`/api/invoices/${encodeURIComponent(id)}`),
    enabled: !!branchId && !!id,
  });
}

export function invoicePaymentsOptions(branchId: string, id: string) {
  return queryOptions({
    queryKey: faturaKeys.payments(branchId, id),
    queryFn: () =>
      api().get<InvoicePaymentSummary[]>(`/api/invoices/${encodeURIComponent(id)}/payments`),
    enabled: !!branchId && !!id,
  });
}
