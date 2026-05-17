import { api } from "../api/client";
import type { RadiologyOrder, RadiologyOrderPriority } from "../types/radiology";

export type CreateRadOrderInput = {
  visit_id?: string;
  patient_id?: string;
  procedure_id: string;
  ordering_doctor_id?: string;
  priority?: RadiologyOrderPriority;
  clinical_indication?: string;
  clinical_question?: string;
  notes?: string;
};

export function createRadOrder(input: CreateRadOrderInput): Promise<RadiologyOrder> {
  return api().post<RadiologyOrder>("/api/radiology-orders", input);
}

export function updateRadOrderStatus(
  orderId: string,
  status: "scheduled" | "in_progress" | "acquired" | "cancelled",
): Promise<RadiologyOrder> {
  return api().post<RadiologyOrder>(
    `/api/radiology-orders/${encodeURIComponent(orderId)}/status`,
    { status },
  );
}

export type SaveRadReportInput = {
  findings?: string;
  impression?: string;
  recommendations?: string;
  reporting_doctor_id?: string;
  pacs_study_uid?: string;
  pacs_accession_number?: string;
  thumbnail_url?: string;
};

export function saveRadReport(orderId: string, input: SaveRadReportInput): Promise<RadiologyOrder> {
  return api().patch<RadiologyOrder>(
    `/api/radiology-orders/${encodeURIComponent(orderId)}/report`,
    input,
  );
}

export function verifyRadReport(orderId: string): Promise<RadiologyOrder> {
  return api().post<RadiologyOrder>(
    `/api/radiology-orders/${encodeURIComponent(orderId)}/verify`,
  );
}
