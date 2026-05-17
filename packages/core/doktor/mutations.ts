import { api } from "../api/client";
import type { Doctor } from "../types/people";
import type { CreatePersonelInput } from "../personel/mutations";

export type CreateDoktorInput = {
  // Either link to an existing staff_member...
  staff_member_id?: string;
  // ...or create a staff_member in the same call.
  staff?: CreatePersonelInput;
  diploma_no?: string;
  medula_doctor_code?: string;
  is_accepting_patients?: boolean;
  specialization_ids?: string[];
  primary_specialization_id?: string;
};

export function createDoktor(input: CreateDoktorInput): Promise<Doctor> {
  return api().post<Doctor>("/api/doctors", input);
}
