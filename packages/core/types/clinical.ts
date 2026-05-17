import type { Timestamps, Uuid } from "./common";

export type VisitStatus = "in_progress" | "completed" | "cancelled";

export type EncounterType =
  | "outpatient"
  | "emergency"
  | "follow_up"
  | "consultation"
  | "control"
  | "admission";

export type Visit = {
  id: Uuid;
  organization_id: Uuid;
  branch_id: Uuid;
  patient_id: Uuid;
  doctor_id?: Uuid;
  appointment_id?: Uuid;
  encounter_type: EncounterType;
  status: VisitStatus;
  chief_complaint?: string;
  history_of_present_illness?: string;
  examination_findings?: string;
  treatment_plan?: string;
  notes?: string;
  started_at: string;
  ended_at?: string;

  // List-only joined display fields.
  patient_mrn?: string;
  patient_first_name?: string;
  patient_last_name?: string;
  patient_phone?: string;
  doctor_first_name?: string;
  doctor_last_name?: string;
  doctor_title?: string;
} & Timestamps;

export type DiagnosisKind =
  | "primary"
  | "secondary"
  | "provisional"
  | "differential"
  | "ruled_out";

export type Diagnosis = {
  id: Uuid;
  visit_id: Uuid;
  icd10_code: string;
  icd10_title: string;
  kind: DiagnosisKind;
  notes?: string;
  created_at: string;
};

export type PrescriptionStatus =
  | "draft"
  | "signed"
  | "sent_to_sgk"
  | "dispensed"
  | "cancelled";

export type PrescriptionItem = {
  id: Uuid;
  medication_name: string;
  dosage?: string;
  frequency?: string;
  duration_days?: number;
  quantity?: string;
  instructions?: string;
  sort_order: number;
};

export type Prescription = {
  id: Uuid;
  visit_id: Uuid;
  patient_id: Uuid;
  doctor_id?: Uuid;
  prescription_no: string;
  e_prescription_no?: string;
  status: PrescriptionStatus;
  notes?: string;
  signed_at?: string;
  items: PrescriptionItem[];
} & Timestamps;

export type VitalSigns = {
  id: Uuid;
  patient_id: Uuid;
  visit_id?: Uuid;
  measured_at: string;
  systolic_bp?: number;
  diastolic_bp?: number;
  pulse?: number;
  temperature_c?: number;
  spo2_percent?: number;
  respiration?: number;
  weight_kg?: number;
  height_cm?: number;
  pain_score?: number;
  notes?: string;
  created_at: string;
};
