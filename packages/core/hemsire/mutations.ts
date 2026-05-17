import { api } from "../api/client";
import type { VitalSigns } from "../types/clinical";

export type AddPatientVitalsInput = {
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
};

// POST /api/patients/{id}/vitals — records a vital sample directly against
// a patient (no visit_id). Used by the nursing dashboard for inpatient rounds.
export function addPatientVitals(patientId: string, input: AddPatientVitalsInput): Promise<VitalSigns> {
  return api().post<VitalSigns>(
    `/api/patients/${encodeURIComponent(patientId)}/vitals`,
    input,
  );
}
