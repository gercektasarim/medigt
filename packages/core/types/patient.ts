import type { Timestamps, Uuid } from "./common";

export type PatientGender = "male" | "female" | "unknown";

export type PatientIdentifierKind =
  | "tc"
  | "passport"
  | "foreigner_id"
  | "temporary_protection"
  | "newborn";

export type BloodType =
  | "A_pos" | "A_neg"
  | "B_pos" | "B_neg"
  | "AB_pos" | "AB_neg"
  | "O_pos" | "O_neg"
  | "unknown";

export type Patient = {
  id: Uuid;
  organization_id: Uuid;
  mrn: string;
  first_name: string;
  last_name: string;
  birth_date?: string;
  gender: PatientGender;
  blood_type: BloodType;
  identifier_kind?: PatientIdentifierKind;
  identifier_value?: string;
  identifier_masked?: string;
  mernis_verified_at?: string;
  phone?: string;
  email?: string;
  address?: string;
  next_of_kin_name?: string;
  next_of_kin_phone?: string;
  notes?: string;
  is_deceased: boolean;
  deceased_at?: string;
} & Timestamps;
