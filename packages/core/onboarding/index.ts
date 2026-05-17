"use client";

import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";

export type OnboardingStatus = {
  organization_id: string;
  branch_id: string;
  specializations_count: number;
  institutions_count: number;
  services_count: number;
  doctors_count: number;
  patients_count: number;
  warehouses_count: number;
  cash_registers_count: number;
  icd10_system_count: number;
};

export const onboardingKeys = {
  status: (orgId: string, branchId: string) =>
    ["onboarding", orgId, branchId, "status"] as const,
};

export function onboardingStatusOptions(orgId: string, branchId: string) {
  return queryOptions({
    queryKey: onboardingKeys.status(orgId, branchId),
    queryFn: () => api().get<OnboardingStatus>("/api/onboarding/status"),
    enabled: !!orgId && !!branchId,
    staleTime: 30_000,
  });
}

export type SeedDefaultsResult = {
  institutions_added: number;
  services_added: number;
  warehouses_added: number;
};

export function seedOnboardingDefaults(): Promise<SeedDefaultsResult> {
  return api().post<SeedDefaultsResult>("/api/onboarding/seed-defaults", {});
}
