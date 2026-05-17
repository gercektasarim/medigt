import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Specialization } from "../types/people";

export const bransKeys = {
  all: (orgId: string) => ["brans", orgId] as const,
  list: (orgId: string) => [...bransKeys.all(orgId), "list"] as const,
};

export function bransListOptions(orgId: string) {
  return queryOptions({
    queryKey: bransKeys.list(orgId),
    queryFn: () => api().get<Specialization[]>("/api/specializations"),
    enabled: !!orgId,
  });
}
