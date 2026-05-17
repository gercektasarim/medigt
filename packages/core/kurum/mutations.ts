import { api } from "../api/client";
import type { ExternalInstitution, InstitutionKind } from "../types/institution";

export type CreateKurumInput = {
  code: string;
  name: string;
  kind: InstitutionKind;
  tax_id?: string;
  phone?: string;
  email?: string;
  address?: string;
  contract_no?: string;
  notes?: string;
};

export function createKurum(input: CreateKurumInput): Promise<ExternalInstitution> {
  return api().post<ExternalInstitution>("/api/institutions", input);
}
