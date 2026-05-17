import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  DialysisMachine,
  DialysisSession,
  DialysisStatus,
} from "../types/dialysis";

export const diyalizKeys = {
  all: (branchId: string) => ["diyaliz", branchId] as const,
  machines: (branchId: string) => [...diyalizKeys.all(branchId), "machines"] as const,
  list: (
    branchId: string,
    status: string,
    from: string,
    to: string,
    patientId: string,
    machineId: string,
  ) => [...diyalizKeys.all(branchId), "list", status, from, to, patientId, machineId] as const,
  detail: (branchId: string, id: string) =>
    [...diyalizKeys.all(branchId), "detail", id] as const,
};

export function dialysisMachineListOptions(branchId: string) {
  return queryOptions({
    queryKey: diyalizKeys.machines(branchId),
    queryFn: () => api().get<DialysisMachine[]>("/api/dialysis-machines"),
    enabled: !!branchId,
  });
}

export type DialysisSessionListFilter = {
  status?: DialysisStatus;
  from?: string; // YYYY-MM-DD
  to?: string;
  patientId?: string;
  machineId?: string;
};

export function dialysisSessionListOptions(branchId: string, f: DialysisSessionListFilter = {}) {
  return queryOptions({
    queryKey: diyalizKeys.list(
      branchId,
      f.status ?? "",
      f.from ?? "",
      f.to ?? "",
      f.patientId ?? "",
      f.machineId ?? "",
    ),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.status) params.set("status", f.status);
      if (f.from) params.set("from", f.from);
      if (f.to) params.set("to", f.to);
      if (f.patientId) params.set("patient_id", f.patientId);
      if (f.machineId) params.set("machine_id", f.machineId);
      const qs = params.toString();
      return api().get<DialysisSession[]>(`/api/dialysis-sessions${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}

export function dialysisSessionDetailOptions(branchId: string, id: string) {
  return queryOptions({
    queryKey: diyalizKeys.detail(branchId, id),
    queryFn: () =>
      api().get<DialysisSession>(`/api/dialysis-sessions/${encodeURIComponent(id)}`),
    enabled: !!branchId && !!id,
  });
}
