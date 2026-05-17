import type { Timestamps, Uuid } from "./common";

export type OrganizationKind = "single_hospital" | "hospital_group" | "clinic" | "polyclinic";

export type Organization = {
  id: Uuid;
  slug: string;
  name: string;
  kind: OrganizationKind;
  tax_id?: string;
  sgk_employer_no?: string;
  logo_url?: string;
} & Timestamps;

export type BranchKind = "hospital" | "polyclinic" | "lab" | "imaging_center" | "dialysis_center" | "dental_clinic";

export type Branch = {
  id: Uuid;
  organization_id: Uuid;
  slug: string;
  name: string;
  kind: BranchKind;
  address?: string;
  phone?: string;
  sgk_facility_code?: string;
} & Timestamps;

export type DepartmentKind =
  | "outpatient"
  | "inpatient"
  | "emergency"
  | "lab"
  | "radiology"
  | "pharmacy"
  | "cashier"
  | "surgery"
  | "dialysis"
  | "dental"
  | "administration";

export type Department = {
  id: Uuid;
  branch_id: Uuid;
  name: string;
  kind: DepartmentKind;
  parent_id?: Uuid;
} & Timestamps;
