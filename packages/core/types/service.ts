import type { Timestamps, Uuid } from "./common";

export type ServiceCategory =
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

export type ServiceCatalogItem = {
  id: Uuid;
  organization_id: Uuid;
  code: string;
  sut_code?: string;
  name: string;
  category: ServiceCategory;
  description?: string;
  unit: string;
  vat_rate: number;
  base_price?: number;
  requires_doctor: boolean;
  is_active: boolean;
} & Timestamps;

export type ServicePrice = {
  id: Uuid;
  service_catalog_id: Uuid;
  external_institution_id?: Uuid;
  price: number;
  currency: string;
  valid_from: string;
  valid_to?: string;
  notes?: string;
  created_at: string;
};
