import type { Uuid } from "./common";

export type PendingPrescriptionItem = {
  item_id: Uuid;
  medication_name: string;
  medication_id?: Uuid;
  dosage?: string;
  frequency?: string;
  quantity?: string;
  dispense_quantity?: number;
  dispensed_total: number;
  instructions?: string;
};

export type PendingPrescription = {
  id: Uuid;
  prescription_no: string;
  status: "signed" | "sent_to_sgk";
  signed_at?: string;
  patient_id: Uuid;
  patient_mrn: string;
  patient_first_name: string;
  patient_last_name: string;
  doctor_first_name?: string;
  doctor_last_name?: string;
  doctor_title?: string;
  items: PendingPrescriptionItem[];
};

export type LotSummary = {
  stock_id: Uuid;
  lot_no: string;
  expiry_date?: string;
  quantity: number;
};

export type ItsNotifyStatus = "pending" | "in_progress" | "notified" | "rejected" | "failed";

export type DispenseHistoryRow = {
  id: Uuid;
  prescription_no: string;
  patient_mrn: string;
  patient_first_name: string;
  patient_last_name: string;
  medication_name: string;
  warehouse_name: string;
  lot_no: string;
  expiry_date?: string;
  quantity: number;
  movement_no: string;
  dispensed_at: string;
  its_status: ItsNotifyStatus;
  its_notified_at?: string;
  its_error?: string;
};
