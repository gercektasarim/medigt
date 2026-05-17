import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Patient } from "../types/patient";

export const hastaKeys = {
  all: (orgId: string) => ["hasta", orgId] as const,
  list: (orgId: string, q: string) => [...hastaKeys.all(orgId), "list", q] as const,
  detail: (orgId: string, id: string) => [...hastaKeys.all(orgId), "detail", id] as const,
};

export function hastaListOptions(orgId: string, q = "") {
  return queryOptions({
    queryKey: hastaKeys.list(orgId, q),
    queryFn: () => {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      const qs = params.toString();
      return api().get<Patient[]>(`/api/patients${qs ? `?${qs}` : ""}`);
    },
    enabled: !!orgId,
  });
}

export function hastaDetailOptions(orgId: string, id: string) {
  return queryOptions({
    queryKey: hastaKeys.detail(orgId, id),
    queryFn: () => api().get<Patient>(`/api/patients/${encodeURIComponent(id)}`),
    enabled: !!orgId && !!id,
  });
}
