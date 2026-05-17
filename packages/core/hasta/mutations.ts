import { api } from "../api/client";
import type {
  BloodType,
  Patient,
  PatientGender,
  PatientIdentifierKind,
} from "../types/patient";

export type CreateHastaInput = {
  first_name: string;
  last_name: string;
  birth_date?: string;          // YYYY-MM-DD
  gender?: PatientGender;
  blood_type?: BloodType;
  identifier_kind?: PatientIdentifierKind;
  identifier_value?: string;
  phone?: string;
  email?: string;
  address?: string;
  next_of_kin_name?: string;
  next_of_kin_phone?: string;
  notes?: string;
};

export function createHasta(input: CreateHastaInput): Promise<Patient> {
  return api().post<Patient>("/api/patients", input);
}

export type ValidateTCResult = { valid: boolean };
export function validateTC(tc: string): Promise<ValidateTCResult> {
  return api().post<ValidateTCResult>("/api/util/tc/validate", { tc });
}

// Local-only TC validator — mirrors the backend algorithm so the create form
// can give instant feedback without a round trip. Backend re-validates anyway.
export function validateTCLocal(tc: string): boolean {
  const s = tc.trim();
  if (s.length !== 11) return false;
  const d: number[] = [];
  for (const ch of s) {
    const c = ch.charCodeAt(0);
    if (c < 48 || c > 57) return false;
    d.push(c - 48);
  }
  if (d[0] === 0) return false;
  const oddSum = d[0]! + d[2]! + d[4]! + d[6]! + d[8]!;
  const evenSum = d[1]! + d[3]! + d[5]! + d[7]!;
  let d10 = (oddSum * 7 - evenSum) % 10;
  if (d10 < 0) d10 += 10;
  if (d10 !== d[9]) return false;
  const total = d.slice(0, 10).reduce((a, b) => a + b, 0);
  return total % 10 === d[10];
}
