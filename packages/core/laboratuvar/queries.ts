import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { LabOrder, LabOrderStatus, LabTest } from "../types/lab";

export const labKeys = {
  all: (branchId: string) => ["lab", branchId] as const,
  tests: (orgId: string, q: string) => ["lab", orgId, "tests", q] as const,
  orders: (branchId: string, status: string, visitId: string) =>
    [...labKeys.all(branchId), "orders", status, visitId] as const,
  order: (branchId: string, id: string) =>
    [...labKeys.all(branchId), "order", id] as const,
};

export type LabOrderFilter = {
  status?: LabOrderStatus;
  visitId?: string;
};

export function labTestSearchOptions(orgId: string, q: string) {
  return queryOptions({
    queryKey: labKeys.tests(orgId, q),
    queryFn: () => {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      return api().get<LabTest[]>(`/api/lab-tests?${params.toString()}`);
    },
    enabled: !!orgId,
    staleTime: 60_000,
  });
}

export function labOrderListOptions(branchId: string, f: LabOrderFilter = {}) {
  return queryOptions({
    queryKey: labKeys.orders(branchId, f.status ?? "", f.visitId ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.status) params.set("status", f.status);
      if (f.visitId) params.set("visit_id", f.visitId);
      const qs = params.toString();
      return api().get<LabOrder[]>(`/api/lab-orders${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}

export function labOrderDetailOptions(branchId: string, orderId: string) {
  return queryOptions({
    queryKey: labKeys.order(branchId, orderId),
    queryFn: () => api().get<LabOrder>(`/api/lab-orders/${encodeURIComponent(orderId)}`),
    enabled: !!branchId && !!orderId,
  });
}
