import { api } from "../api/client";
import type {
  Admission,
  AdmissionKind,
  Bed,
  BedKind,
  BedStatus,
  DischargeKind,
  Ward,
  WardKind,
} from "../types/inpatient";

// ---- Ward ----

export type CreateWardInput = {
  code: string;
  name: string;
  kind: WardKind;
  floor?: string;
  capacity?: number;
  notes?: string;
};

export function createWard(input: CreateWardInput): Promise<Ward> {
  return api().post<Ward>("/api/wards", input);
}

// ---- Bed ----

export type CreateBedInput = {
  code: string;
  kind?: BedKind;
  notes?: string;
};

export function createBed(wardId: string, input: CreateBedInput): Promise<Bed> {
  return api().post<Bed>(`/api/wards/${encodeURIComponent(wardId)}/beds`, input);
}

export function setBedStatus(bedId: string, status: BedStatus): Promise<void> {
  return api().post<void>(`/api/beds/${encodeURIComponent(bedId)}/status`, { status });
}

// ---- Admission ----

export type AdmitInput = {
  patient_id: string;
  ward_id: string;
  bed_id?: string;
  admitting_doctor_id?: string;
  kind?: AdmissionKind;
  chief_complaint?: string;
  admission_diagnosis?: string;
  notes?: string;
};

export function admit(input: AdmitInput): Promise<Admission> {
  return api().post<Admission>("/api/admissions", input);
}

export type TransferInput = {
  to_bed_id: string;
  reason?: string;
};

export function transferAdmission(id: string, input: TransferInput): Promise<Admission> {
  return api().post<Admission>(
    `/api/admissions/${encodeURIComponent(id)}/transfer`,
    input,
  );
}

export type DischargeInput = {
  kind: DischargeKind;
  summary?: string;
};

export function discharge(id: string, input: DischargeInput): Promise<Admission> {
  return api().post<Admission>(
    `/api/admissions/${encodeURIComponent(id)}/discharge`,
    input,
  );
}
