import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Icd10Code } from "../types/icd10";

export const icd10Keys = {
  all: (orgId: string) => ["icd10", orgId] as const,
  search: (orgId: string, q: string, limit: number) =>
    [...icd10Keys.all(orgId), "search", q, limit] as const,
};

export function icd10SearchOptions(orgId: string, q: string, limit = 50) {
  return queryOptions({
    queryKey: icd10Keys.search(orgId, q, limit),
    queryFn: () => {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      params.set("limit", String(limit));
      return api().get<Icd10Code[]>(`/api/icd10?${params.toString()}`);
    },
    enabled: !!orgId,
    staleTime: 60 * 1000,
  });
}
