import type { Uuid } from "./common";

export type ServiceCategoryKey =
  | "consultation"
  | "lab"
  | "imaging"
  | "procedure"
  | "surgery"
  | "inpatient"
  | "medication"
  | "supply"
  | "package"
  | "other";

export type HakedisSummary = {
  doctor_id: Uuid;
  first_name: string;
  last_name: string;
  title?: string;
  item_count: number;
  gross_revenue: number;
  earning_total: number;
};

export type HakedisItem = {
  invoice_item_id: Uuid;
  invoice_id: Uuid;
  invoice_no: string;
  issued_at: string;
  patient_mrn: string;
  patient_first_name: string;
  patient_last_name: string;
  code: string;
  name: string;
  category?: ServiceCategoryKey;
  line_total: number;
  commission_pct: number;
  earning: number;
};

export type CommissionRule = {
  id: Uuid;
  doctor_id: Uuid;
  category?: ServiceCategoryKey;
  commission_pct: number;
  valid_from: string; // YYYY-MM-DD
  valid_to?: string;
  notes?: string;
};
