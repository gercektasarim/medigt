"use client";

import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  MedulaCodeName,
  MedulaDoctorDetail,
  MedulaDrugPayment,
  MedulaEraport,
  MedulaEraportDetail,
  MedulaEraportKind,
  MedulaEraportStatus,
  MedulaInvoiceSubmission,
  MedulaProvision,
  MedulaProvisionStatus,
  MedulaProvisionType,
  MedulaReferral,
  MedulaReferralStatus,
  MedulaReferralType,
  MedulaSubmitStatus,
  MedulaTakipDetail,
} from "../types";

export const medulaKeys = {
  all: (branchId: string) => ["medula", branchId] as const,
  provisions: (branchId: string, status: string) =>
    [...medulaKeys.all(branchId), "provisions", status] as const,
  provisionDetail: (branchId: string, id: string) =>
    [...medulaKeys.all(branchId), "provisions", "detail", id] as const,
  submissions: (branchId: string, status: string) =>
    [...medulaKeys.all(branchId), "submissions", status] as const,
  referrals: (branchId: string, status: string) =>
    [...medulaKeys.all(branchId), "referrals", status] as const,
  eraports: (branchId: string, status: string) =>
    [...medulaKeys.all(branchId), "eraports", status] as const,
  takipDetail: (takipNo: string) => ["medula-query", "takip", takipNo] as const,
  eraportDetail: (eraportNo: string) => ["medula-query", "eraport", eraportNo] as const,
  doctorDetail: (tc: string) => ["medula-query", "doctor", tc] as const,
  branches: () => ["medula-query", "branches"] as const,
  treatmentTypes: () => ["medula-query", "treatment-types"] as const,
  drugPayment: (barcode: string) => ["medula-query", "drug", barcode] as const,
};

// ---------- Provisions ----------

export function medulaProvisionListOptions(branchId: string, status?: MedulaProvisionStatus) {
  return queryOptions({
    queryKey: medulaKeys.provisions(branchId, status ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (status) params.set("status", status);
      const qs = params.toString();
      return api().get<MedulaProvision[]>(`/api/medula/provisions${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
    refetchInterval: (q) => {
      const rows = q.state.data ?? [];
      return rows.some((r) => r.status === "pending" || r.status === "in_progress") ? 10_000 : false;
    },
  });
}

export function medulaProvisionDetailOptions(branchId: string, id: string) {
  return queryOptions({
    queryKey: medulaKeys.provisionDetail(branchId, id),
    queryFn: () => api().get<MedulaProvision>(`/api/medula/provisions/${encodeURIComponent(id)}`),
    enabled: !!branchId && !!id,
  });
}

export type CreateProvisionInput = {
  patient_id: string;
  institution_id?: string;
  provision_type?: MedulaProvisionType;
  branch_code?: string;
};

export function createMedulaProvision(input: CreateProvisionInput): Promise<MedulaProvision> {
  return api().post<MedulaProvision>("/api/medula/provisions", input);
}

export function cancelMedulaProvision(id: string, reason?: string): Promise<{ ok: boolean }> {
  return api().post<{ ok: boolean }>(`/api/medula/provisions/${encodeURIComponent(id)}/cancel`, { reason });
}

export function closeMedulaTakip(id: string): Promise<{ ok: boolean }> {
  return api().post<{ ok: boolean }>(`/api/medula/provisions/${encodeURIComponent(id)}/close-takip`, {});
}

// ---------- Invoice submissions ----------

export function invoiceSubmissionListOptions(branchId: string, status?: MedulaSubmitStatus) {
  return queryOptions({
    queryKey: medulaKeys.submissions(branchId, status ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (status) params.set("status", status);
      const qs = params.toString();
      return api().get<MedulaInvoiceSubmission[]>(`/api/medula/invoice-submissions${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
    refetchInterval: (q) => {
      const rows = q.state.data ?? [];
      return rows.some((r) => r.status === "pending" || r.status === "in_progress") ? 10_000 : false;
    },
  });
}

export type CreateInvoiceSubmissionInput = {
  invoice_id: string;
  provision_id?: string;
};

export function createInvoiceSubmission(input: CreateInvoiceSubmissionInput): Promise<MedulaInvoiceSubmission> {
  return api().post<MedulaInvoiceSubmission>("/api/medula/invoice-submissions", input);
}

export function cancelInvoiceSubmission(id: string, reason?: string): Promise<{ ok: boolean }> {
  return api().post<{ ok: boolean }>(`/api/medula/invoice-submissions/${encodeURIComponent(id)}/cancel`, { reason });
}

// ---------- Referrals ----------

export function referralListOptions(branchId: string, status?: MedulaReferralStatus) {
  return queryOptions({
    queryKey: medulaKeys.referrals(branchId, status ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (status) params.set("status", status);
      const qs = params.toString();
      return api().get<MedulaReferral[]>(`/api/medula/referrals${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
    refetchInterval: (q) => {
      const rows = q.state.data ?? [];
      return rows.some((r) => r.status === "pending" || r.status === "in_progress") ? 10_000 : false;
    },
  });
}

export type CreateReferralInput = {
  patient_id: string;
  referring_doctor_id?: string;
  target_provider_code: string;
  target_provider_name?: string;
  target_branch_code?: string;
  reason: string;
  diagnosis_icd10?: string;
  referral_type?: MedulaReferralType;
};

export function createMedulaReferral(input: CreateReferralInput): Promise<MedulaReferral> {
  return api().post<MedulaReferral>("/api/medula/referrals", input);
}

// ---------- e-Rapor ----------

export function eraportListOptions(branchId: string, status?: MedulaEraportStatus) {
  return queryOptions({
    queryKey: medulaKeys.eraports(branchId, status ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (status) params.set("status", status);
      const qs = params.toString();
      return api().get<MedulaEraport[]>(`/api/medula/eraports${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
    refetchInterval: (q) => {
      const rows = q.state.data ?? [];
      return rows.some((r) => r.status === "pending" || r.status === "in_progress") ? 10_000 : false;
    },
  });
}

export type CreateEraportInput = {
  patient_id: string;
  doctor_id?: string;
  kind?: MedulaEraportKind;
  diagnoses_icd10: string[];
  drug_codes: string[];
  valid_from: string;
  valid_to?: string;
  report_text?: string;
};

export function createMedulaEraport(input: CreateEraportInput): Promise<MedulaEraport> {
  return api().post<MedulaEraport>("/api/medula/eraports", input);
}

export function cancelMedulaEraport(id: string, reason?: string): Promise<{ ok: boolean }> {
  return api().post<{ ok: boolean }>(`/api/medula/eraports/${encodeURIComponent(id)}/cancel`, { reason });
}

// ---------- Sync queries (mock today, real SOAP later) ----------

export function takipQueryOptions(takipNo: string) {
  return queryOptions({
    queryKey: medulaKeys.takipDetail(takipNo),
    queryFn: () => api().get<MedulaTakipDetail>(`/api/medula/queries/takip/${encodeURIComponent(takipNo)}`),
    enabled: !!takipNo,
  });
}

export function eraportQueryOptions(eraportNo: string) {
  return queryOptions({
    queryKey: medulaKeys.eraportDetail(eraportNo),
    queryFn: () => api().get<MedulaEraportDetail>(`/api/medula/queries/eraport/${encodeURIComponent(eraportNo)}`),
    enabled: !!eraportNo,
  });
}

export function doctorQueryOptions(tc: string) {
  return queryOptions({
    queryKey: medulaKeys.doctorDetail(tc),
    queryFn: () => api().get<MedulaDoctorDetail>(`/api/medula/queries/doctor/${encodeURIComponent(tc)}`),
    enabled: !!tc && tc.length === 11,
  });
}

export function branchesQueryOptions() {
  return queryOptions({
    queryKey: medulaKeys.branches(),
    queryFn: () => api().get<MedulaCodeName[]>("/api/medula/queries/branches"),
    staleTime: 24 * 60 * 60 * 1000,
  });
}

export function treatmentTypesQueryOptions() {
  return queryOptions({
    queryKey: medulaKeys.treatmentTypes(),
    queryFn: () => api().get<MedulaCodeName[]>("/api/medula/queries/treatment-types"),
    staleTime: 24 * 60 * 60 * 1000,
  });
}

export function drugPaymentQueryOptions(barcode: string) {
  return queryOptions({
    queryKey: medulaKeys.drugPayment(barcode),
    queryFn: () => api().get<MedulaDrugPayment>(`/api/medula/queries/drug-payment/${encodeURIComponent(barcode)}`),
    enabled: !!barcode,
  });
}
