"use client";

import { api } from "../api/client";

// Video assistant / kiosk intake — single POST that creates a patient
// (if missing) and books an appointment in 'arrived' state so it
// shows up immediately in the poliklinik queue.

export type IntakeInput = {
  tc_kimlik_no: string;
  first_name: string;
  last_name: string;
  birth_year?: number;
  gender?: "male" | "female" | "other" | "unknown";
  phone?: string;
  complaint?: string;
  specialization_id?: string;
  doctor_id?: string;
};

export type IntakeResult = {
  patient_id: string;
  patient_mrn: string;
  patient_created: boolean;
  appointment_id: string;
  appointment_no: string;
  scheduled_at: string;
  doctor_id?: string;
  doctor_full_name?: string;
  specialization_name?: string;
};

export function submitIntake(input: IntakeInput): Promise<IntakeResult> {
  return api().post<IntakeResult>("/api/intake", input);
}

// ---------- NLU slot-filling ----------

export type NLUStep =
  | "tc"
  | "name"
  | "birthYear"
  | "phone"
  | "complaint"
  | "specialization"
  | "confirm";

export type NLUResult = {
  tc?: string;
  first_name?: string;
  last_name?: string;
  birth_year?: number;
  phone?: string;
  complaint?: string;
  specialization_id?: string;
  specialization_name?: string;
  confirm_yes?: boolean;
  confirm_no?: boolean;
  confidence: number;
  echo?: string;
};

export function parseTranscript(
  step: NLUStep,
  transcript: string,
): Promise<{ result: NLUResult }> {
  return api().post<{ result: NLUResult }>("/api/intake/parse", {
    step,
    transcript,
  });
}
