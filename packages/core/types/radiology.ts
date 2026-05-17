import type { Timestamps, Uuid } from "./common";

export type RadiologyModality =
  | "XR"
  | "USG"
  | "CT"
  | "MR"
  | "MAMMO"
  | "NM"
  | "DEXA"
  | "PET"
  | "ANGIO"
  | "FLUORO"
  | "OTHER";

export type RadiologyOrderStatus =
  | "ordered"
  | "scheduled"
  | "in_progress"
  | "acquired"
  | "reported"
  | "verified"
  | "cancelled";

export type RadiologyOrderPriority = "routine" | "urgent" | "stat";

export type RadiologyProcedure = {
  id: Uuid;
  code: string;
  name: string;
  modality: RadiologyModality;
  body_region?: string;
  sut_code?: string;
  estimated_minutes?: number;
  preparation_notes?: string;
  is_system: boolean;
};

export type RadiologyOrder = {
  id: Uuid;
  order_no: string;
  status: RadiologyOrderStatus;
  priority: RadiologyOrderPriority;
  visit_id?: Uuid;
  patient_id: Uuid;
  patient_mrn: string;
  patient_first_name: string;
  patient_last_name: string;
  doctor_first_name?: string;
  doctor_last_name?: string;
  doctor_title?: string;
  procedure_id: Uuid;
  procedure_code: string;
  procedure_name: string;
  modality: RadiologyModality;
  body_region?: string;
  clinical_indication?: string;
  clinical_question?: string;
  notes?: string;
  scheduled_at?: string;
  acquired_at?: string;
  findings?: string;
  impression?: string;
  recommendations?: string;
  reported_at?: string;
  verified_at?: string;
  pacs_study_uid?: string;
  pacs_accession_number?: string;
  thumbnail_url?: string;
  ordered_at: string;
} & Timestamps;
