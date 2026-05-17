import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  Admission,
  AdmissionStatus,
  BedMapEntry,
  BedTransferEntry,
  Ward,
} from "../types/inpatient";

export const yatisKeys = {
  all: (branchId: string) => ["yatis", branchId] as const,
  wards: (branchId: string) => [...yatisKeys.all(branchId), "wards"] as const,
  bedMap: (branchId: string) => [...yatisKeys.all(branchId), "bed-map"] as const,
  admissions: (branchId: string, status: string, wardId: string) =>
    [...yatisKeys.all(branchId), "admissions", status, wardId] as const,
  admission: (branchId: string, id: string) =>
    [...yatisKeys.all(branchId), "admission", id] as const,
  transfers: (admissionId: string) => ["yatis", "transfers", admissionId] as const,
};

export function wardListOptions(branchId: string, activeOnly = true) {
  return queryOptions({
    queryKey: [...yatisKeys.wards(branchId), activeOnly] as const,
    queryFn: () => {
      const qs = activeOnly ? "?active=true" : "";
      return api().get<Ward[]>(`/api/wards${qs}`);
    },
    enabled: !!branchId,
  });
}

export function bedMapOptions(branchId: string) {
  return queryOptions({
    queryKey: yatisKeys.bedMap(branchId),
    queryFn: () => api().get<BedMapEntry[]>("/api/bed-map"),
    enabled: !!branchId,
  });
}

export type AdmissionFilter = {
  status?: AdmissionStatus;
  wardId?: string;
};

export function admissionListOptions(branchId: string, f: AdmissionFilter = {}) {
  return queryOptions({
    queryKey: yatisKeys.admissions(branchId, f.status ?? "", f.wardId ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (f.status) params.set("status", f.status);
      if (f.wardId) params.set("ward_id", f.wardId);
      const qs = params.toString();
      return api().get<Admission[]>(`/api/admissions${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
  });
}

export function admissionDetailOptions(branchId: string, id: string) {
  return queryOptions({
    queryKey: yatisKeys.admission(branchId, id),
    queryFn: () => api().get<Admission>(`/api/admissions/${encodeURIComponent(id)}`),
    enabled: !!branchId && !!id,
  });
}

export function admissionTransfersOptions(admissionId: string) {
  return queryOptions({
    queryKey: yatisKeys.transfers(admissionId),
    queryFn: () =>
      api().get<BedTransferEntry[]>(
        `/api/admissions/${encodeURIComponent(admissionId)}/transfers`,
      ),
    enabled: !!admissionId,
  });
}

// ---------- HL7 ADT mesajları (gönderilen) ----------

export type ADTOutboundMessage = {
  id: string;
  message_control_id: string;
  event_type: "A01" | "A02" | "A03" | "A04" | "A08";
  status: "pending" | "in_flight" | "sent" | "failed" | "dead";
  retry_count: number;
  next_retry_at: string;
  last_error?: string;
  sent_at?: string;
  created_at: string;
  raw_message: string;
  ack_raw?: string;
};

export const adtKeys = {
  forAdmission: (admissionId: string) => ["yatis", "adt", admissionId] as const,
};

export function admissionADTMessagesOptions(admissionId: string) {
  return queryOptions({
    queryKey: adtKeys.forAdmission(admissionId),
    queryFn: () =>
      api().get<ADTOutboundMessage[]>(
        `/api/admissions/${encodeURIComponent(admissionId)}/adt-messages`,
      ),
    enabled: !!admissionId,
  });
}
