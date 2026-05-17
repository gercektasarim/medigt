import { api } from "../api/client";
import type { Branch, Organization, OrganizationKind } from "../types/hospital";

export type InitialBranchInput = {
  slug: string;
  name: string;
  kind: Branch["kind"];
  sgk_facility_code?: string;
};

export type CreateOrganizationInput = {
  slug: string;
  name: string;
  kind: OrganizationKind;
  tax_id?: string;
  sgk_employer_no?: string;
  initial_branch?: InitialBranchInput;
};

export type CreateOrganizationResult = {
  organization: Organization;
  branch?: Branch;
};

export function createOrganization(input: CreateOrganizationInput): Promise<Organization> {
  // Backend returns { organization, branch? } — we only return Organization
  // for backwards-compatible callers; full result is available via createOrganizationFull.
  return api()
    .post<CreateOrganizationResult>("/api/organizations", input)
    .then((r) => r.organization);
}

export function createOrganizationFull(input: CreateOrganizationInput): Promise<CreateOrganizationResult> {
  return api().post<CreateOrganizationResult>("/api/organizations", input);
}

export type CreateBranchInput = {
  organization_id: string;
  slug: string;
  name: string;
  kind: Branch["kind"];
  sgk_facility_code?: string;
};

export function createBranch(input: CreateBranchInput): Promise<Branch> {
  return api().post<Branch>(`/api/organizations/${encodeURIComponent(input.organization_id)}/branches`, input);
}
