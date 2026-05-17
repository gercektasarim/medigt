import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  DispenseHistoryRow,
  LotSummary,
  PendingPrescription,
} from "../types/eczane";

export const eczaneKeys = {
  all: (orgId: string) => ["eczane", orgId] as const,
  pending: (orgId: string) => [...eczaneKeys.all(orgId), "pending"] as const,
  fefo: (orgId: string, warehouseId: string, medicationId: string) =>
    [...eczaneKeys.all(orgId), "fefo", warehouseId, medicationId] as const,
  history: (orgId: string) => [...eczaneKeys.all(orgId), "history"] as const,
};

export function eczanePendingOptions(orgId: string) {
  return queryOptions({
    queryKey: eczaneKeys.pending(orgId),
    queryFn: () => api().get<PendingPrescription[]>("/api/eczane/pending"),
    enabled: !!orgId,
    // The queue is shared across operators — refresh frequently so two
    // pharmacists don't dispense the same item.
    refetchInterval: 30_000,
  });
}

export function fefoLotsOptions(orgId: string, warehouseId: string, medicationId: string) {
  return queryOptions({
    queryKey: eczaneKeys.fefo(orgId, warehouseId, medicationId),
    queryFn: () =>
      api().get<LotSummary[]>(
        `/api/eczane/fefo?warehouse_id=${encodeURIComponent(warehouseId)}&medication_id=${encodeURIComponent(medicationId)}`,
      ),
    enabled: !!orgId && !!warehouseId && !!medicationId,
  });
}

export function dispenseHistoryOptions(orgId: string) {
  return queryOptions({
    queryKey: eczaneKeys.history(orgId),
    queryFn: () => api().get<DispenseHistoryRow[]>("/api/eczane/history"),
    enabled: !!orgId,
  });
}
