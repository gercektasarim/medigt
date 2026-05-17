import { api } from "../api/client";
import type { LabOrder, LabOrderItem, LabOrderPriority, LabResultFlag } from "../types/lab";

export type CreateLabOrderInput = {
  visit_id?: string;          // when ordering from a doctor's visit screen
  patient_id?: string;        // when ordering walk-in (no visit yet)
  ordering_doctor_id?: string;
  priority?: LabOrderPriority;
  clinical_indication?: string;
  notes?: string;
  test_ids: string[];
};

export function createLabOrder(input: CreateLabOrderInput): Promise<LabOrder> {
  return api().post<LabOrder>("/api/lab-orders", input);
}

export function updateLabOrderStatus(
  orderId: string,
  status: "sampled" | "in_progress" | "verified" | "cancelled",
): Promise<LabOrder> {
  return api().post<LabOrder>(
    `/api/lab-orders/${encodeURIComponent(orderId)}/status`,
    { status },
  );
}

export type UpdateLabItemInput = {
  value_numeric?: number;
  value_text?: string;
  flag?: LabResultFlag;
  notes?: string;
};

export function updateLabItemResult(itemId: string, input: UpdateLabItemInput): Promise<LabOrderItem> {
  return api().patch<LabOrderItem>(
    `/api/lab-order-items/${encodeURIComponent(itemId)}/result`,
    input,
  );
}
