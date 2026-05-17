import { api } from "../api/client";
import type { ServiceCatalogItem, ServiceCategory, ServicePrice } from "../types/service";

export type CreateHizmetInput = {
  code: string;
  name: string;
  category: ServiceCategory;
  sut_code?: string;
  description?: string;
  unit?: string;
  vat_rate: number;
  base_price?: number;
  requires_doctor?: boolean;
};

export function createHizmet(input: CreateHizmetInput): Promise<ServiceCatalogItem> {
  return api().post<ServiceCatalogItem>("/api/services", input);
}

export type CreateHizmetPriceInput = {
  external_institution_id?: string; // null = varsayılan / oop
  price: number;
  currency?: string;
  valid_from?: string;
  valid_to?: string;
  notes?: string;
};

export function createHizmetPrice(serviceId: string, input: CreateHizmetPriceInput): Promise<ServicePrice> {
  return api().post<ServicePrice>(`/api/services/${encodeURIComponent(serviceId)}/prices`, input);
}

// ---------- Bulk price update wizard ----------

export type BulkPriceFilter = {
  service_ids?: string[];
  category?: ServiceCategory | "";
  institution_ids?: string[];
  include_oop?: boolean;
};

export type BulkPriceAdjustment = {
  kind: "percent" | "fixed" | "set";
  amount: number;
  valid_from?: string;
  notes?: string;
  min_price?: number;
  max_price?: number;
};

export type BulkPriceInput = BulkPriceFilter & BulkPriceAdjustment;

export type BulkPricePreviewRow = {
  service_id: string;
  service_code: string;
  service_name: string;
  institution_id?: string;
  institution_name?: string;
  old_price: number;
  new_price: number;
};

export type BulkPricePreview = {
  affected: number;
  changed: number;
  avg_pct: number;
  total_old: number;
  total_new: number;
  rows: BulkPricePreviewRow[];
};

export type BulkPriceResult = {
  affected: number;
  inserted: number;
  skipped: number;
};

export function previewBulkPriceUpdate(input: BulkPriceInput): Promise<BulkPricePreview> {
  return api().post<BulkPricePreview>(`/api/service-prices/bulk-preview`, input);
}

export function applyBulkPriceUpdate(input: BulkPriceInput): Promise<BulkPriceResult> {
  return api().post<BulkPriceResult>(`/api/service-prices/bulk-update`, input);
}
