import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Diagnosis, Prescription, Visit, VitalSigns } from "../types/clinical";

export const poliklinikKeys = {
  all: (branchId: string) => ["poliklinik", branchId] as const,
  list: (branchId: string, status: string, from: string, to: string) =>
    [...poliklinikKeys.all(branchId), "list", status, from, to] as const,
  detail: (branchId: string, visitId: string) =>
    [...poliklinikKeys.all(branchId), "detail", visitId] as const,
  diagnoses: (visitId: string) => ["poliklinik", "diagnoses", visitId] as const,
  prescriptions: (visitId: string) => ["poliklinik", "prescriptions", visitId] as const,
  vitals: (visitId: string) => ["poliklinik", "vitals", visitId] as const,
};

export type VisitListFilter = {
  status?: string;        // "in_progress" | "completed" | ""
  from?: string;          // YYYY-MM-DD
  to?: string;
};

export function visitListOptions(branchId: string, f: VisitListFilter = {}) {
  return queryOptions({
    queryKey: poliklinikKeys.list(branchId, f.status ?? "", f.from ?? "", f.to ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.status) params.set("status", f.status);
      if (f.from) params.set("from", f.from);
      if (f.to) params.set("to", f.to);
      const qs = params.toString();
      return api().get<Visit[]>(`/api/visits${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}

export function visitDetailOptions(branchId: string, visitId: string) {
  return queryOptions({
    queryKey: poliklinikKeys.detail(branchId, visitId),
    queryFn: () => api().get<Visit>(`/api/visits/${encodeURIComponent(visitId)}`),
    enabled: !!branchId && !!visitId,
  });
}

export function diagnosisListOptions(visitId: string) {
  return queryOptions({
    queryKey: poliklinikKeys.diagnoses(visitId),
    queryFn: () => api().get<Diagnosis[]>(`/api/visits/${encodeURIComponent(visitId)}/diagnoses`),
    enabled: !!visitId,
  });
}

export function prescriptionListOptions(visitId: string) {
  return queryOptions({
    queryKey: poliklinikKeys.prescriptions(visitId),
    queryFn: () => api().get<Prescription[]>(`/api/visits/${encodeURIComponent(visitId)}/prescriptions`),
    enabled: !!visitId,
  });
}

export function vitalsListOptions(visitId: string) {
  return queryOptions({
    queryKey: poliklinikKeys.vitals(visitId),
    queryFn: () => api().get<VitalSigns[]>(`/api/visits/${encodeURIComponent(visitId)}/vitals`),
    enabled: !!visitId,
  });
}
