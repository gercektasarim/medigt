import { api } from "../api/client";
import type {
  DialysisMachine,
  DialysisModality,
  DialysisSession,
  VascularAccessType,
} from "../types/dialysis";

export type CreateDialysisMachineInput = {
  code: string;
  name: string;
  manufacturer?: string;
  model?: string;
  location?: string;
  notes?: string;
};

export function createDialysisMachine(
  input: CreateDialysisMachineInput,
): Promise<DialysisMachine> {
  return api().post<DialysisMachine>("/api/dialysis-machines", input);
}

export type CreateDialysisSessionInput = {
  patient_id: string;
  machine_id?: string;
  admission_id?: string;
  primary_nurse_id?: string;
  supervisor_doctor_id?: string;
  modality?: DialysisModality;
  vascular_access?: VascularAccessType;
  scheduled_at: string;
  duration_minutes?: number;
  dry_weight_kg?: number;
  dialyzer_type?: string;
  anticoagulant?: string;
  ultrafiltration_target_ml?: number;
  blood_flow_rate?: number;
  dialysate_flow_rate?: number;
};

export function createDialysisSession(
  input: CreateDialysisSessionInput,
): Promise<DialysisSession> {
  return api().post<DialysisSession>("/api/dialysis-sessions", input);
}

export function updateDialysisStatus(
  id: string,
  status: "in_progress" | "completed" | "cancelled",
): Promise<DialysisSession> {
  return api().post<DialysisSession>(
    `/api/dialysis-sessions/${encodeURIComponent(id)}/status`,
    { status },
  );
}

export type SaveDialysisRecordInput = {
  pre_weight_kg?: number;
  pre_systolic_bp?: number;
  pre_diastolic_bp?: number;
  post_weight_kg?: number;
  post_systolic_bp?: number;
  post_diastolic_bp?: number;
  actual_ultrafiltration_ml?: number;
  complications?: string;
  session_notes?: string;
};

export function saveDialysisRecord(
  id: string,
  input: SaveDialysisRecordInput,
): Promise<DialysisSession> {
  return api().patch<DialysisSession>(
    `/api/dialysis-sessions/${encodeURIComponent(id)}/record`,
    input,
  );
}
