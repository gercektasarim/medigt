import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  StockMovement,
  StockMovementKind,
  StockRow,
  Warehouse,
} from "../types/stock";

export const depoKeys = {
  all: (branchId: string) => ["depo", branchId] as const,
  warehouses: (branchId: string) => [...depoKeys.all(branchId), "warehouses"] as const,
  stock: (
    branchId: string,
    warehouseId: string,
    q: string,
    expiringDays: number,
    withZero: boolean,
  ) =>
    [...depoKeys.all(branchId), "stock", warehouseId, q, expiringDays, withZero] as const,
  movements: (
    branchId: string,
    warehouseId: string,
    kind: string,
    from: string,
    to: string,
  ) =>
    [...depoKeys.all(branchId), "movements", warehouseId, kind, from, to] as const,
};

export function warehouseListOptions(branchId: string) {
  return queryOptions({
    queryKey: depoKeys.warehouses(branchId),
    queryFn: () => api().get<Warehouse[]>("/api/warehouses"),
    enabled: !!branchId,
  });
}

export type StockListFilter = {
  warehouseId?: string;
  medicationId?: string;
  q?: string;
  expiringDays?: number;
  withZero?: boolean;
};

export function stockListOptions(branchId: string, f: StockListFilter = {}) {
  return queryOptions({
    queryKey: depoKeys.stock(
      branchId,
      f.warehouseId ?? "",
      f.q ?? "",
      f.expiringDays ?? 0,
      f.withZero ?? false,
    ),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.warehouseId) params.set("warehouse_id", f.warehouseId);
      if (f.medicationId) params.set("medication_id", f.medicationId);
      if (f.q) params.set("q", f.q);
      if (f.expiringDays && f.expiringDays > 0) params.set("expiring_days", String(f.expiringDays));
      if (f.withZero) params.set("with_zero", "true");
      const qs = params.toString();
      return api().get<StockRow[]>(`/api/stock${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}

export type MovementListFilter = {
  warehouseId?: string;
  medicationId?: string;
  kind?: StockMovementKind;
  from?: string;
  to?: string;
};

export function movementListOptions(branchId: string, f: MovementListFilter = {}) {
  return queryOptions({
    queryKey: depoKeys.movements(
      branchId,
      f.warehouseId ?? "",
      f.kind ?? "",
      f.from ?? "",
      f.to ?? "",
    ),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.warehouseId) params.set("warehouse_id", f.warehouseId);
      if (f.medicationId) params.set("medication_id", f.medicationId);
      if (f.kind) params.set("kind", f.kind);
      if (f.from) params.set("from", f.from);
      if (f.to) params.set("to", f.to);
      const qs = params.toString();
      return api().get<StockMovement[]>(`/api/stock-movements${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}
