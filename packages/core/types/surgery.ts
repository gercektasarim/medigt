import type { Timestamps, Uuid } from "./common";

export type SurgeryStatus = "scheduled" | "in_progress" | "completed" | "cancelled";

export type SurgeryPriority = "elective" | "urgent" | "emergency";

export type AnesthesiaType =
  | "general"
  | "regional"
  | "spinal"
  | "epidural"
  | "local"
  | "sedation"
  | "none";

export type SurgeryTeamRole =
  | "primary_surgeon"
  | "assistant"
  | "anesthesiologist"
  | "scrub_nurse"
  | "circulating_nurse"
  | "technician";

export type SurgeryTeamMember = {
  staff_member_id?: Uuid;
  doctor_id?: Uuid;
  role: SurgeryTeamRole;
  name: string;
};

export type OperatingRoom = {
  id: Uuid;
  code: string;
  name: string;
  floor?: string;
};

export type Surgery = {
  id: Uuid;
  surgery_no: string;
  status: SurgeryStatus;
  priority: SurgeryPriority;
  patient_id: Uuid;
  patient_mrn: string;
  patient_first_name: string;
  patient_last_name: string;
  operating_room_id: Uuid;
  operating_room_code: string;
  operating_room_name: string;
  primary_surgeon_id?: Uuid;
  surgeon_first_name?: string;
  surgeon_last_name?: string;
  surgeon_title?: string;
  admission_id?: Uuid;
  procedure_name: string;
  procedure_codes: string[];
  indication?: string;
  anesthesia_type: AnesthesiaType;
  scheduled_at: string;
  estimated_minutes: number;
  team: SurgeryTeamMember[];
  started_at?: string;
  ended_at?: string;
  op_note?: string;
  complications?: string;
  blood_loss_ml?: number;
  specimen_sent: boolean;
  cancelled_at?: string;
  cancellation_reason?: string;
} & Timestamps;
