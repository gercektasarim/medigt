import type { Timestamps, Uuid } from "./common";

export type DialysisStatus = "scheduled" | "in_progress" | "completed" | "cancelled";

export type DialysisModality = "hemodialysis" | "hemodiafiltration" | "peritoneal";

export type VascularAccessType =
  | "av_fistula"
  | "av_graft"
  | "central_catheter"
  | "peritoneal_catheter"
  | "other";

export type DialysisMachine = {
  id: Uuid;
  code: string;
  name: string;
  manufacturer?: string;
  model?: string;
  location?: string;
};

export type DialysisSession = {
  id: Uuid;
  session_no: string;
  status: DialysisStatus;
  modality: DialysisModality;
  vascular_access: VascularAccessType;
  patient_id: Uuid;
  patient_mrn?: string;
  patient_first_name?: string;
  patient_last_name?: string;
  machine_id?: Uuid;
  machine_code?: string;
  machine_name?: string;
  admission_id?: Uuid;
  primary_nurse_id?: Uuid;
  supervisor_doctor_id?: Uuid;
  scheduled_at: string;
  duration_minutes: number;
  pre_weight_kg?: number;
  pre_systolic_bp?: number;
  pre_diastolic_bp?: number;
  dry_weight_kg?: number;
  dialyzer_type?: string;
  anticoagulant?: string;
  ultrafiltration_target_ml?: number;
  blood_flow_rate?: number;
  dialysate_flow_rate?: number;
  started_at?: string;
  ended_at?: string;
  post_weight_kg?: number;
  post_systolic_bp?: number;
  post_diastolic_bp?: number;
  actual_ultrafiltration_ml?: number;
  complications?: string;
  session_notes?: string;
  cancelled_at?: string;
  cancellation_reason?: string;
} & Timestamps;
