import { create } from "zustand";
import type { Branch, Organization } from "../types/hospital";

type HospitalState = {
  organization: Organization | null;
  branch: Branch | null;
  availableOrgs: Organization[];
  availableBranches: Branch[];
  setOrganization: (org: Organization | null) => void;
  setBranch: (branch: Branch | null) => void;
  setAvailable: (orgs: Organization[], branches: Branch[]) => void;
  clear: () => void;
};

export const useHospitalStore = create<HospitalState>((set) => ({
  organization: null,
  branch: null,
  availableOrgs: [],
  availableBranches: [],
  setOrganization: (organization) => set({ organization }),
  setBranch: (branch) => set({ branch }),
  setAvailable: (availableOrgs, availableBranches) => set({ availableOrgs, availableBranches }),
  clear: () => set({ organization: null, branch: null }),
}));

export function getCurrentOrgId(): string | null {
  return useHospitalStore.getState().organization?.id ?? null;
}

export function getCurrentBranchId(): string | null {
  return useHospitalStore.getState().branch?.id ?? null;
}
