import { api } from "../api/client";
import type { CommissionRule, ServiceCategoryKey } from "../types/hakedis";

export type CreateCommissionRuleInput = {
  category?: ServiceCategoryKey;
  commission_pct: number;
  valid_from?: string; // YYYY-MM-DD; defaults to today server-side
  notes?: string;
};

export function createCommissionRule(
  doctorId: string,
  input: CreateCommissionRuleInput,
): Promise<CommissionRule> {
  return api().post<CommissionRule>(
    `/api/hakedis/${encodeURIComponent(doctorId)}/rules`,
    input,
  );
}

// ---------- Bulk rule import ----------

export type BulkCommissionRuleInput = {
  doctor_ids?: string[];
  specialization_codes?: string[];
  category?: ServiceCategoryKey;
  commission_pct: number;
  valid_from?: string;
  notes?: string;
};

export type BulkCommissionRulePreview = {
  targeted_doctors: number;
  doctor_ids: string[];
};

export type BulkCommissionRuleResult = {
  targeted_doctors: number;
  rules_added: number;
  skipped: number;
  errors: string[];
};

export function previewBulkCommissionRules(
  input: BulkCommissionRuleInput,
): Promise<BulkCommissionRulePreview> {
  return api().post<BulkCommissionRulePreview>(
    `/api/hakedis/bulk-rules/preview`,
    input,
  );
}

export function bulkCreateCommissionRules(
  input: BulkCommissionRuleInput,
): Promise<BulkCommissionRuleResult> {
  return api().post<BulkCommissionRuleResult>(
    `/api/hakedis/bulk-rules`,
    input,
  );
}
