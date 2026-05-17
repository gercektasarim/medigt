import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { CommissionRule, HakedisItem, HakedisSummary } from "../types/hakedis";

export const hakedisKeys = {
  all: (branchId: string) => ["hakedis", branchId] as const,
  summary: (branchId: string, from: string, to: string) =>
    [...hakedisKeys.all(branchId), "summary", from, to] as const,
  items: (branchId: string, doctorId: string, from: string, to: string) =>
    [...hakedisKeys.all(branchId), "items", doctorId, from, to] as const,
  rules: (doctorId: string) => ["hakedis", "rules", doctorId] as const,
};

export type HakedisRange = { from: string; to: string };

export function hakedisSummaryOptions(branchId: string, range: HakedisRange) {
  return queryOptions({
    queryKey: hakedisKeys.summary(branchId, range.from, range.to),
    queryFn: () =>
      api().get<HakedisSummary[]>(
        `/api/hakedis?from=${encodeURIComponent(range.from)}&to=${encodeURIComponent(range.to)}`,
      ),
    enabled: !!branchId,
  });
}

export function hakedisItemsOptions(branchId: string, doctorId: string, range: HakedisRange) {
  return queryOptions({
    queryKey: hakedisKeys.items(branchId, doctorId, range.from, range.to),
    queryFn: () =>
      api().get<HakedisItem[]>(
        `/api/hakedis/${encodeURIComponent(doctorId)}/items?from=${encodeURIComponent(range.from)}&to=${encodeURIComponent(range.to)}`,
      ),
    enabled: !!branchId && !!doctorId,
  });
}

export function commissionRulesOptions(doctorId: string) {
  return queryOptions({
    queryKey: hakedisKeys.rules(doctorId),
    queryFn: () =>
      api().get<CommissionRule[]>(`/api/hakedis/${encodeURIComponent(doctorId)}/rules`),
    enabled: !!doctorId,
  });
}
