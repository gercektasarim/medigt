import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  RadiologyModality,
  RadiologyOrder,
  RadiologyOrderStatus,
  RadiologyProcedure,
} from "../types/radiology";

export const radyolojiKeys = {
  all: (branchId: string) => ["radyoloji", branchId] as const,
  procedures: (orgId: string, q: string, modality: string) =>
    ["radyoloji", orgId, "procedures", q, modality] as const,
  orders: (branchId: string, status: string, modality: string, visitId: string) =>
    [...radyolojiKeys.all(branchId), "orders", status, modality, visitId] as const,
  order: (branchId: string, id: string) =>
    [...radyolojiKeys.all(branchId), "order", id] as const,
  images: (branchId: string, orderId: string) =>
    [...radyolojiKeys.all(branchId), "images", orderId] as const,
};

export type ImageReference = {
  id: string;
  study_instance_uid: string;
  series_instance_uid?: string;
  modality: string;
  study_date?: string;
  description?: string;
  instance_count: number;
  pacs_base_url?: string;
  thumbnail_url?: string;
  viewer_url: string;
  submitted_at?: string;
  last_synced_at?: string;
};

export function orderImageReferencesOptions(branchId: string, orderId: string) {
  return queryOptions({
    queryKey: radyolojiKeys.images(branchId, orderId),
    queryFn: () =>
      api().get<ImageReference[]>(
        `/api/radiology-orders/${encodeURIComponent(orderId)}/images`,
      ),
    enabled: !!branchId && !!orderId,
    // Order yeni oluşturulduğunda PACS schedule goroutine'i ~0.5s sonra
    // image_reference yazar; UI ilk açılışta boş gösterip 3s sonra
    // yeniden çekecek.
    refetchInterval: (q) => (q.state.data?.length ? false : 3_000),
  });
}

export function radProcedureSearchOptions(orgId: string, q: string, modality?: RadiologyModality) {
  return queryOptions({
    queryKey: radyolojiKeys.procedures(orgId, q, modality ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      if (modality) params.set("modality", modality);
      return api().get<RadiologyProcedure[]>(`/api/radiology-procedures?${params.toString()}`);
    },
    enabled: !!orgId,
    staleTime: 60_000,
  });
}

export type RadOrderFilter = {
  status?: RadiologyOrderStatus;
  modality?: RadiologyModality;
  visitId?: string;
};

export function radOrderListOptions(branchId: string, f: RadOrderFilter = {}) {
  return queryOptions({
    queryKey: radyolojiKeys.orders(
      branchId,
      f.status ?? "",
      f.modality ?? "",
      f.visitId ?? "",
    ),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.status) params.set("status", f.status);
      if (f.modality) params.set("modality", f.modality);
      if (f.visitId) params.set("visit_id", f.visitId);
      const qs = params.toString();
      return api().get<RadiologyOrder[]>(`/api/radiology-orders${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}

export function radOrderDetailOptions(branchId: string, orderId: string) {
  return queryOptions({
    queryKey: radyolojiKeys.order(branchId, orderId),
    queryFn: () =>
      api().get<RadiologyOrder>(`/api/radiology-orders/${encodeURIComponent(orderId)}`),
    enabled: !!branchId && !!orderId,
  });
}
