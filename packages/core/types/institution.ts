import type { Timestamps, Uuid } from "./common";

export type InstitutionKind =
  | "sgk"
  | "private_insurance"
  | "corporate"
  | "foreign_insurance"
  | "oop"
  | "other";

export type ExternalInstitution = {
  id: Uuid;
  organization_id: Uuid;
  code: string;
  name: string;
  kind: InstitutionKind;
  tax_id?: string;
  address?: string;
  phone?: string;
  email?: string;
  contract_no?: string;
  contract_starts_at?: string;
  contract_ends_at?: string;
  is_active: boolean;
  notes?: string;
} & Timestamps;
