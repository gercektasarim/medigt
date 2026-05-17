import type { Timestamps, Uuid } from "./common";

export type LabSampleType =
  | "blood"
  | "urine"
  | "stool"
  | "sputum"
  | "throat_swab"
  | "nasal_swab"
  | "csf"
  | "tissue"
  | "other";

export type LabOrderStatus =
  | "ordered"
  | "sampled"
  | "in_progress"
  | "resulted"
  | "verified"
  | "cancelled";

export type LabOrderPriority = "routine" | "urgent" | "stat";

export type LabResultFlag =
  | "normal"
  | "low"
  | "high"
  | "critical_low"
  | "critical_high"
  | "abnormal";

export type LabTest = {
  id: Uuid;
  code: string;
  name: string;
  sample_type: LabSampleType;
  unit?: string;
  reference_range?: string;
  loinc_code?: string;
  sut_code?: string;
  is_system: boolean;
};

export type LabOrderItem = {
  id: Uuid;
  test_code: string;
  test_name: string;
  sample_type: LabSampleType;
  unit?: string;
  reference_range?: string;
  status: LabOrderStatus;
  sort_order: number;
  value_numeric?: number;
  value_text?: string;
  flag?: LabResultFlag;
  resulted_at?: string;
  notes?: string;
};

export type LabOrder = {
  id: Uuid;
  order_no: string;
  status: LabOrderStatus;
  priority: LabOrderPriority;
  visit_id?: Uuid;
  patient_id: Uuid;
  patient_mrn: string;
  patient_first_name: string;
  patient_last_name: string;
  doctor_first_name?: string;
  doctor_last_name?: string;
  doctor_title?: string;
  clinical_indication?: string;
  notes?: string;
  ordered_at: string;
  sampled_at?: string;
  completed_at?: string;
  items: LabOrderItem[];
} & Timestamps;
