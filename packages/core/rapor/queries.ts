import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { ReportResult } from "../types/report";

export const raporKeys = {
  all: (branchId: string) => ["rapor", branchId] as const,
  result: (branchId: string, id: string, paramsKey: string) =>
    [...raporKeys.all(branchId), id, paramsKey] as const,
};

export function runReportOptions(branchId: string, id: string, params: Record<string, string>) {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== "" && v != null) sp.set(k, v);
  }
  const qs = sp.toString();
  return queryOptions({
    queryKey: raporKeys.result(branchId, id, qs),
    queryFn: () =>
      api().get<ReportResult>(`/api/reports/${encodeURIComponent(id)}${qs ? `?${qs}` : ""}`),
    enabled: !!branchId && !!id,
    staleTime: 30_000,
  });
}
