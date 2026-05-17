import type { Timestamps, Uuid } from "./common";

export type AppointmentStatus =
  | "scheduled"
  | "arrived"
  | "in_progress"
  | "completed"
  | "no_show"
  | "cancelled";

export type VisitKind =
  | "outpatient"
  | "follow_up"
  | "emergency"
  | "consultation"
  | "control";

export type Appointment = {
  id: Uuid;
  organization_id: Uuid;
  branch_id: Uuid;
  patient_id: Uuid;
  doctor_id?: Uuid;
  department_id?: Uuid;
  scheduled_at: string;        // ISO 8601
  duration_minutes: number;
  status: AppointmentStatus;
  kind: VisitKind;
  reason?: string;
  notes?: string;
  arrived_at?: string;
  started_at?: string;
  completed_at?: string;
  cancelled_at?: string;
  cancellation_reason?: string;

  // Joined display fields (present in list responses only).
  patient_mrn?: string;
  patient_first_name?: string;
  patient_last_name?: string;
  patient_phone?: string;
  doctor_first_name?: string;
  doctor_last_name?: string;
  doctor_title?: string;
} & Timestamps;
