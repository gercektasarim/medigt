import type { Uuid } from "./common";

export type MernisVerifyInput = {
  tc_kimlik_no: string;
  first_name: string;
  last_name: string;
  birth_year: number;
};

export type MernisVerifyResult = {
  verified: boolean;
  response_code?: string;
  error?: string;
  latency_ms: number;
  log_id: string;
};

export type MernisLogRow = {
  id: Uuid;
  tc_last4: string;
  first_name: string;
  last_name: string;
  birth_year: number;
  verified: boolean;
  response_code?: string;
  error_message?: string;
  requested_at: string;
};

export type MedulaProvisionStatus =
  | "pending"
  | "in_progress"
  | "completed"
  | "failed"
  | "cancelled";

export type MedulaProvisionType = "normal" | "acil" | "yatis";

export type MedulaProvision = {
  id: Uuid;
  patient_id: Uuid;
  patient_mrn?: string;
  patient_first_name?: string;
  patient_last_name?: string;
  institution_id?: Uuid;
  institution_name?: string;
  takip_no?: string;
  provision_type: MedulaProvisionType;
  branch_code?: string;
  status: MedulaProvisionStatus;
  response_code?: string;
  error_message?: string;
  requested_at: string;
  completed_at?: string;
  response_payload?: Record<string, unknown>;
};
