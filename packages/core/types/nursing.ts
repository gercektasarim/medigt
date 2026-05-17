import type { Uuid } from "./common";
import type { AdmissionKind, WardKind } from "./inpatient";

// Flat row shape coming back from /api/inpatient-board — admission + patient
// + ward + bed + the patient's most-recent vital sample in one shot.
export type InpatientBoardRow = {
  admission_id: Uuid;
  admission_no: string;
  patient_id: Uuid;
  patient_first_name: string;
  patient_last_name: string;
  patient_mrn: string;
  ward_id: Uuid;
  ward_name: string;
  ward_kind: WardKind;
  bed_code?: string;
  kind: AdmissionKind;
  chief_complaint?: string;
  admission_diagnosis?: string;
  admitted_at: string;

  vitals_measured_at?: string;
  systolic_bp?: number;
  diastolic_bp?: number;
  pulse?: number;
  temperature_c?: number;
  spo2_percent?: number;
  respiration?: number;
  pain_score?: number;
};
