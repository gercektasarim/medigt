import type { Timestamps, Uuid } from "./common";

export type WardKind =
  | "general"
  | "icu"
  | "ccu"
  | "pediatrics"
  | "maternity"
  | "surgical"
  | "isolation"
  | "observation";

export type BedKind =
  | "standard"
  | "icu"
  | "isolation"
  | "pediatric"
  | "vip"
  | "observation";

export type BedStatus = "free" | "occupied" | "reserved" | "cleaning" | "blocked";

export type AdmissionKind = "planned" | "emergency" | "transfer_in" | "newborn";

export type AdmissionStatus = "active" | "discharged";

export type DischargeKind =
  | "home"
  | "home_with_help"
  | "referred"
  | "against_advice"
  | "left_without_notice"
  | "transferred"
  | "expired";

export type Ward = {
  id: Uuid;
  code: string;
  name: string;
  kind: WardKind;
  floor?: string;
  capacity?: number;
  is_active: boolean;
  notes?: string;
};

export type Bed = {
  id: Uuid;
  ward_id: Uuid;
  code: string;
  kind: BedKind;
  status: BedStatus;
  is_active: boolean;
  notes?: string;
};

export type BedMapEntry = {
  bed: Bed;
  ward_id: Uuid;
  ward_name: string;
  ward_kind: WardKind;
  admission_id?: Uuid;
  admission_no?: string;
  patient_id?: Uuid;
  patient_first_name?: string;
  patient_last_name?: string;
  patient_mrn?: string;
  admitted_at?: string;
};

export type Admission = {
  id: Uuid;
  admission_no: string;
  patient_id: Uuid;
  patient_mrn: string;
  patient_first_name: string;
  patient_last_name: string;
  patient_phone?: string;
  ward_id: Uuid;
  ward_code: string;
  ward_name: string;
  bed_id?: Uuid;
  bed_code?: string;
  admitting_doctor_id?: Uuid;
  doctor_first_name?: string;
  doctor_last_name?: string;
  doctor_title?: string;
  kind: AdmissionKind;
  status: AdmissionStatus;
  chief_complaint?: string;
  admission_diagnosis?: string;
  notes?: string;
  admitted_at: string;
  discharged_at?: string;
  discharge_kind?: DischargeKind;
  discharge_summary?: string;
} & Timestamps;

export type BedTransferEntry = {
  id: Uuid;
  from_bed_code?: string;
  to_bed_code: string;
  from_ward_name?: string;
  to_ward_name: string;
  reason?: string;
  transferred_at: string;
};
