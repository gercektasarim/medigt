import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  Medication,
  MedicationForm,
  PrescriptionClass,
} from "../types/medication";

export const ilacKeys = {
  all: (orgId: string) => ["ilac", orgId] as const,
  list: (orgId: string, q: string, form: string, klass: string, active: boolean) =>
    [...ilacKeys.all(orgId), "list", q, form, klass, active] as const,
  detail: (orgId: string, id: string) =>
    [...ilacKeys.all(orgId), "detail", id] as const,
};

export type MedicationListFilter = {
  q?: string;
  form?: MedicationForm;
  prescriptionClass?: PrescriptionClass;
  activeOnly?: boolean;
};

export function medicationListOptions(orgId: string, f: MedicationListFilter = {}) {
  return queryOptions({
    queryKey: ilacKeys.list(
      orgId,
      f.q ?? "",
      f.form ?? "",
      f.prescriptionClass ?? "",
      f.activeOnly !== false,
    ),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.q) params.set("q", f.q);
      if (f.form) params.set("form", f.form);
      if (f.prescriptionClass) params.set("class", f.prescriptionClass);
      if (f.activeOnly === false) params.set("active", "false");
      const qs = params.toString();
      return api().get<Medication[]>(`/api/medications${qs ? `?${qs}` : ""}`);
    },
    enabled: !!orgId,
  });
}

export function medicationDetailOptions(orgId: string, id: string) {
  return queryOptions({
    queryKey: ilacKeys.detail(orgId, id),
    queryFn: () => api().get<Medication>(`/api/medications/${encodeURIComponent(id)}`),
    enabled: !!orgId && !!id,
  });
}
