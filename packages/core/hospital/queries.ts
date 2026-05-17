import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Branch, Organization } from "../types/hospital";

export const hospitalKeys = {
  all: () => ["hospital"] as const,
  organizations: () => [...hospitalKeys.all(), "organizations"] as const,
  organization: (slug: string) => [...hospitalKeys.all(), "organization", slug] as const,
  branches: (orgId: string) => [...hospitalKeys.all(), "branches", orgId] as const,
  branch: (orgId: string, branchSlug: string) => [...hospitalKeys.all(), "branch", orgId, branchSlug] as const,
};

export function organizationsListOptions() {
  return queryOptions({
    queryKey: hospitalKeys.organizations(),
    queryFn: () => api().get<Organization[]>("/api/organizations"),
  });
}

export function organizationOptions(slug: string) {
  return queryOptions({
    queryKey: hospitalKeys.organization(slug),
    queryFn: () => api().get<Organization>(`/api/organizations/${encodeURIComponent(slug)}`),
    enabled: !!slug,
  });
}

export function branchesListOptions(orgId: string) {
  return queryOptions({
    queryKey: hospitalKeys.branches(orgId),
    queryFn: () => api().get<Branch[]>(`/api/organizations/${encodeURIComponent(orgId)}/branches`),
    enabled: !!orgId,
  });
}

export function branchOptions(orgId: string, branchSlug: string) {
  return queryOptions({
    queryKey: hospitalKeys.branch(orgId, branchSlug),
    queryFn: () => api().get<Branch>(`/api/organizations/${encodeURIComponent(orgId)}/branches/${encodeURIComponent(branchSlug)}`),
    enabled: !!orgId && !!branchSlug,
  });
}
