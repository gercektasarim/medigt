import { api } from "../api/client";
import type {
  Diagnosis,
  DiagnosisKind,
  Prescription,
  Visit,
  VitalSigns,
} from "../types/clinical";

export type StartVisitInput = { appointment_id: string };

export function startVisitFromAppointment(input: StartVisitInput): Promise<Visit> {
  return api().post<Visit>("/api/visits/start-from-appointment", input);
}

export type UpdateVisitNotesInput = {
  chief_complaint?: string;
  history_of_present_illness?: string;
  examination_findings?: string;
  treatment_plan?: string;
  notes?: string;
};

export function updateVisitNotes(visitId: string, input: UpdateVisitNotesInput): Promise<Visit> {
  return api().patch<Visit>(`/api/visits/${encodeURIComponent(visitId)}/notes`, input);
}

export function completeVisit(visitId: string): Promise<Visit> {
  return api().post<Visit>(`/api/visits/${encodeURIComponent(visitId)}/complete`);
}

export type AddDiagnosisInput = {
  icd10_code: string;
  icd10_title: string;
  kind?: DiagnosisKind;
  notes?: string;
};

export function addDiagnosis(visitId: string, input: AddDiagnosisInput): Promise<Diagnosis> {
  return api().post<Diagnosis>(`/api/visits/${encodeURIComponent(visitId)}/diagnoses`, input);
}

export function deleteDiagnosis(visitId: string, diagnosisId: string): Promise<void> {
  return api().delete(
    `/api/visits/${encodeURIComponent(visitId)}/diagnoses/${encodeURIComponent(diagnosisId)}`,
  );
}

export type CreatePrescriptionItemInput = {
  medication_name: string;
  dosage?: string;
  frequency?: string;
  duration_days?: number;
  quantity?: string;
  instructions?: string;
};

export type CreatePrescriptionInput = {
  notes?: string;
  items: CreatePrescriptionItemInput[];
};

export function createPrescription(visitId: string, input: CreatePrescriptionInput): Promise<Prescription> {
  return api().post<Prescription>(`/api/visits/${encodeURIComponent(visitId)}/prescriptions`, input);
}

// signPrescription: opsiyonel digital_signature_id (e-imza ile imzalama).
// Backend signature_id verildiyse imzanın 'signed' durumunda olduğunu,
// imzalayan kullanıcıya ait olduğunu ve aynı reçeteye bağlandığını
// doğrular. Boş bırakılırsa eski davranış: doğrudan imzala.
export function signPrescription(
  prescriptionId: string,
  digitalSignatureId?: string,
): Promise<void> {
  const body = digitalSignatureId ? { digital_signature_id: digitalSignatureId } : {};
  return api().post<void>(`/api/prescriptions/${encodeURIComponent(prescriptionId)}/sign`, body);
}

export type AddVitalsInput = {
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

export function addVitals(visitId: string, input: AddVitalsInput): Promise<VitalSigns> {
  return api().post<VitalSigns>(`/api/visits/${encodeURIComponent(visitId)}/vitals`, input);
}
