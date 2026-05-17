import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  OperatingRoom,
  Surgery,
  SurgeryStatus,
} from "../types/surgery";

export const ameliyatKeys = {
  all: (branchId: string) => ["ameliyat", branchId] as const,
  operatingRooms: (branchId: string) => [...ameliyatKeys.all(branchId), "or"] as const,
  list: (branchId: string, status: string, from: string, to: string, orId: string) =>
    [...ameliyatKeys.all(branchId), "list", status, from, to, orId] as const,
  detail: (branchId: string, id: string) =>
    [...ameliyatKeys.all(branchId), "detail", id] as const,
};

export function operatingRoomListOptions(branchId: string) {
  return queryOptions({
    queryKey: ameliyatKeys.operatingRooms(branchId),
    queryFn: () => api().get<OperatingRoom[]>("/api/operating-rooms"),
    enabled: !!branchId,
  });
}

export type SurgeryListFilter = {
  status?: SurgeryStatus;
  from?: string;       // YYYY-MM-DD
  to?: string;
  operatingRoomId?: string;
};

export function surgeryListOptions(branchId: string, f: SurgeryListFilter = {}) {
  return queryOptions({
    queryKey: ameliyatKeys.list(branchId, f.status ?? "", f.from ?? "", f.to ?? "", f.operatingRoomId ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.status) params.set("status", f.status);
      if (f.from) params.set("from", f.from);
      if (f.to) params.set("to", f.to);
      if (f.operatingRoomId) params.set("operating_room_id", f.operatingRoomId);
      const qs = params.toString();
      return api().get<Surgery[]>(`/api/surgeries${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}

export function surgeryDetailOptions(branchId: string, id: string) {
  return queryOptions({
    queryKey: ameliyatKeys.detail(branchId, id),
    queryFn: () => api().get<Surgery>(`/api/surgeries/${encodeURIComponent(id)}`),
    enabled: !!branchId && !!id,
  });
}
