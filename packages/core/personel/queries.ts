import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { StaffMember } from "../types/people";

export const personelKeys = {
  all: (orgId: string) => ["personel", orgId] as const,
  list: (orgId: string, activeOnly: boolean) => [...personelKeys.all(orgId), "list", activeOnly] as const,
};

export function personelListOptions(orgId: string, activeOnly = false) {
  const qs = activeOnly ? "?active=true" : "";
  return queryOptions({
    queryKey: personelKeys.list(orgId, activeOnly),
    queryFn: () => api().get<StaffMember[]>(`/api/staff${qs}`),
    enabled: !!orgId,
  });
}
