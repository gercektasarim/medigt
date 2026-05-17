import { api } from "../api/client";
import type { Specialization } from "../types/people";

export type CreateBransInput = {
  code: string;
  name: string;
  parent_id?: string;
};

export function createBrans(input: CreateBransInput): Promise<Specialization> {
  return api().post<Specialization>("/api/specializations", input);
}

export function deleteBrans(id: string): Promise<void> {
  return api().delete(`/api/specializations/${encodeURIComponent(id)}`);
}
