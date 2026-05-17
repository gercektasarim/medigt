import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  CashMovement,
  CashRegister,
  CashRegisterStatus,
  ZReport,
} from "../types/cash";

export const vezneKeys = {
  all: (branchId: string) => ["vezne", branchId] as const,
  my: (branchId: string) => [...vezneKeys.all(branchId), "my"] as const,
  list: (branchId: string, status: string, from: string, to: string) =>
    [...vezneKeys.all(branchId), "list", status, from, to] as const,
  movements: (branchId: string, registerId: string) =>
    [...vezneKeys.all(branchId), "movements", registerId] as const,
  zReport: (branchId: string, registerId: string) =>
    [...vezneKeys.all(branchId), "z", registerId] as const,
};

export function myRegisterOptions(branchId: string) {
  return queryOptions({
    queryKey: vezneKeys.my(branchId),
    queryFn: () => api().get<CashRegister | null>("/api/cash-registers/my"),
    enabled: !!branchId,
  });
}

export type RegisterListFilter = {
  status?: CashRegisterStatus;
  from?: string;
  to?: string;
};

export function registerListOptions(branchId: string, f: RegisterListFilter = {}) {
  return queryOptions({
    queryKey: vezneKeys.list(branchId, f.status ?? "", f.from ?? "", f.to ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.status) params.set("status", f.status);
      if (f.from) params.set("from", f.from);
      if (f.to) params.set("to", f.to);
      const qs = params.toString();
      return api().get<CashRegister[]>(`/api/cash-registers${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}

export function registerMovementsOptions(branchId: string, registerId: string) {
  return queryOptions({
    queryKey: vezneKeys.movements(branchId, registerId),
    queryFn: () =>
      api().get<CashMovement[]>(
        `/api/cash-registers/${encodeURIComponent(registerId)}/movements`,
      ),
    enabled: !!branchId && !!registerId,
  });
}

export function zReportOptions(branchId: string, registerId: string) {
  return queryOptions({
    queryKey: vezneKeys.zReport(branchId, registerId),
    queryFn: () =>
      api().get<ZReport>(`/api/cash-registers/${encodeURIComponent(registerId)}/z-report`),
    enabled: !!branchId && !!registerId,
  });
}
