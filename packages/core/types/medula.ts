import type { Uuid } from "./common";

// ---------- Invoice submission ----------

export type MedulaSubmitStatus =
  | "pending"
  | "in_progress"
  | "submitted"
  | "accepted"
  | "rejected"
  | "cancelled"
  | "failed";

export type MedulaInvoiceSubmission = {
  id: Uuid;
  invoice_id: Uuid;
  invoice_no?: string;
  total?: number;
  patient_first_name?: string;
  patient_last_name?: string;
  patient_mrn?: string;
  provision_id?: Uuid;
  batch_no?: string;
  sgk_invoice_no?: string;
  status: MedulaSubmitStatus;
  response_code?: string;
  error_message?: string;
  cancelled_at?: string;
  cancellation_reason?: string;
  response_payload?: Record<string, unknown>;
  requested_at: string;
  completed_at?: string;
};

// ---------- Referral ----------

export type MedulaReferralStatus =
  | "pending"
  | "in_progress"
  | "created"
  | "rejected"
  | "cancelled"
  | "failed";

export type MedulaReferralType = "normal" | "acil" | "kontrol";

export type MedulaReferral = {
  id: Uuid;
  patient_id: Uuid;
  patient_mrn?: string;
  patient_first_name?: string;
  patient_last_name?: string;
  referring_doctor_id?: Uuid;
  target_provider_code: string;
  target_provider_name?: string;
  target_branch_code?: string;
  reason: string;
  diagnosis_icd10?: string;
  referral_type: MedulaReferralType;
  status: MedulaReferralStatus;
  sevk_no?: string;
  response_code?: string;
  error_message?: string;
  response_payload?: Record<string, unknown>;
  requested_at: string;
  completed_at?: string;
  cancelled_at?: string;
};

// ---------- e-Rapor ----------

export type MedulaEraportKind =
  | "chronic_drug"
  | "inpatient"
  | "work_incapacity"
  | "special_procedure";

export type MedulaEraportStatus =
  | "pending"
  | "in_progress"
  | "submitted"
  | "approved"
  | "rejected"
  | "cancelled"
  | "failed";

export type MedulaEraport = {
  id: Uuid;
  patient_id: Uuid;
  patient_mrn?: string;
  patient_first_name?: string;
  patient_last_name?: string;
  doctor_id?: Uuid;
  kind: MedulaEraportKind;
  diagnoses_icd10: string[];
  drug_codes: string[];
  valid_from: string;
  valid_to?: string;
  report_text?: string;
  status: MedulaEraportStatus;
  eraport_no?: string;
  response_code?: string;
  error_message?: string;
  response_payload?: Record<string, unknown>;
  requested_at: string;
  completed_at?: string;
  cancelled_at?: string;
};

// ---------- Sync query responses ----------

export type MedulaTakipDetail = {
  takip_no: string;
  status: string;
  provision_type: string;
  opened_at: string;
  closed_at?: string;
  patient: Record<string, unknown>;
  raw: Record<string, unknown>;
};

export type MedulaEraportDetail = {
  eraport_no: string;
  status: string;
  kind: string;
  valid_from: string;
  valid_to?: string;
  diagnoses: string[];
  drug_codes: string[];
  raw: Record<string, unknown>;
};

export type MedulaDoctorDetail = {
  medula_doctor_code: string;
  full_name: string;
  branch_code: string;
  branch_name: string;
  is_active: boolean;
};

export type MedulaCodeName = {
  code: string;
  name: string;
};

export type MedulaDrugPayment = {
  barcode: string;
  drug_name: string;
  is_reimbursed: boolean;
  patient_share_pct: number;
  notes?: string;
};
