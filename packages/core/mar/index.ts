"use client";

import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";

// Medication Administration Record (MAR) — yatan hasta ilaç yönetimi.
// Doktor `medication_order` açar, hemşire her doz için
// `medication_administration` kaydı atar. 5 doğru protokol (doğru hasta,
// ilaç, doz, yol, zaman) verme anında kontrol edilir; barkod taranan
// değerler iz amacıyla saklanır.

export type MedicationRoute =
  | "oral" | "iv" | "im" | "sc" | "topical" | "inhalation"
  | "rectal" | "sublingual" | "intranasal" | "ophthalmic" | "otic" | "other";

export type MedicationOrderStatus =
  | "active" | "on_hold" | "completed" | "cancelled" | "expired";

export type AdministrationStatus =
  | "given" | "refused" | "withheld" | "missed" | "wrong_time";

export type MedicationOrder = {
  id: string;
  order_no: string;
  admission_id: string;
  patient_id: string;
  medication_id: string;
  medication_name?: string;
  medication_code?: string;
  ordering_doctor_id?: string;
  doctor_first_name?: string;
  doctor_last_name?: string;
  dose_amount: number;
  dose_unit: string;
  route: MedicationRoute;
  frequency: string;
  scheduled_times: string[];
  is_prn: boolean;
  prn_reason?: string;
  starts_at: string;
  ends_at?: string;
  instructions?: string;
  status: MedicationOrderStatus;
  created_at: string;
};

export type MedicationAdministration = {
  id: string;
  medication_order_id: string;
  admission_id: string;
  patient_id: string;
  scheduled_at?: string;
  administered_at: string;
  status: AdministrationStatus;
  five_rights_checked: boolean;
  patient_barcode_scanned?: string;
  medication_barcode_scanned?: string;
  dose_amount?: number;
  dose_unit?: string;
  route?: MedicationRoute;
  notes?: string;
  performed_by_user_id?: string;
  witnessed_by_user_id?: string;
  created_at: string;
};

export const marKeys = {
  all: (branchId: string) => ["mar", branchId] as const,
  orders: (branchId: string, admissionId: string) =>
    [...marKeys.all(branchId), "orders", admissionId] as const,
  admins: (branchId: string, admissionId: string) =>
    [...marKeys.all(branchId), "admins", admissionId] as const,
  adminsForOrder: (orderId: string) => ["mar", "order-admins", orderId] as const,
};

export function medicationOrdersOptions(branchId: string, admissionId: string) {
  return queryOptions({
    queryKey: marKeys.orders(branchId, admissionId),
    queryFn: () =>
      api().get<MedicationOrder[]>(
        `/api/admissions/${encodeURIComponent(admissionId)}/medication-orders`,
      ),
    enabled: !!branchId && !!admissionId,
  });
}

export function administrationsForAdmissionOptions(
  branchId: string,
  admissionId: string,
) {
  return queryOptions({
    queryKey: marKeys.admins(branchId, admissionId),
    queryFn: () =>
      api().get<MedicationAdministration[]>(
        `/api/admissions/${encodeURIComponent(admissionId)}/administrations`,
      ),
    enabled: !!branchId && !!admissionId,
  });
}

export function administrationsForOrderOptions(orderId: string) {
  return queryOptions({
    queryKey: marKeys.adminsForOrder(orderId),
    queryFn: () =>
      api().get<MedicationAdministration[]>(
        `/api/medication-orders/${encodeURIComponent(orderId)}/administrations`,
      ),
    enabled: !!orderId,
  });
}

// ---------- Mutations ----------

export type CreateMedicationOrderInput = {
  medication_id: string;
  ordering_doctor_id?: string;
  dose_amount: number;
  dose_unit: string;
  route: MedicationRoute;
  frequency: string;
  scheduled_times?: string[];
  is_prn?: boolean;
  prn_reason?: string;
  starts_at?: string;
  ends_at?: string;
  instructions?: string;
};

export function createMedicationOrder(
  admissionId: string,
  input: CreateMedicationOrderInput,
): Promise<MedicationOrder> {
  return api().post<MedicationOrder>(
    `/api/admissions/${encodeURIComponent(admissionId)}/medication-orders`,
    input,
  );
}

export function updateMedicationOrderStatus(
  orderId: string,
  status: MedicationOrderStatus,
): Promise<{ ok: true }> {
  return api().patch<{ ok: true }>(
    `/api/medication-orders/${encodeURIComponent(orderId)}/status`,
    { status },
  );
}

export type RecordAdministrationInput = {
  scheduled_at?: string;
  status?: AdministrationStatus;
  five_rights_checked: boolean;
  patient_barcode_scanned?: string;
  medication_barcode_scanned?: string;
  dose_amount?: number;
  dose_unit?: string;
  route?: MedicationRoute;
  notes?: string;
  witnessed_by_user_id?: string;
};

export function recordAdministration(
  orderId: string,
  input: RecordAdministrationInput,
): Promise<MedicationAdministration> {
  return api().post<MedicationAdministration>(
    `/api/medication-orders/${encodeURIComponent(orderId)}/administrations`,
    input,
  );
}
