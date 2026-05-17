import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { ExternalInstitution } from "../types/institution";

export const kurumKeys = {
  all: (orgId: string) => ["kurum", orgId] as const,
  list: (orgId: string, activeOnly: boolean) => [...kurumKeys.all(orgId), "list", activeOnly] as const,
};

export function kurumListOptions(orgId: string, activeOnly = false) {
  const qs = activeOnly ? "?active=true" : "";
  return queryOptions({
    queryKey: kurumKeys.list(orgId, activeOnly),
    queryFn: () => api().get<ExternalInstitution[]>(`/api/institutions${qs}`),
    enabled: !!orgId,
  });
}
