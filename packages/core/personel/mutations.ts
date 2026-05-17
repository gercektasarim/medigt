import { api } from "../api/client";
import type { EmploymentType, StaffMember } from "../types/people";

export type CreatePersonelInput = {
  first_name: string;
  last_name: string;
  title?: string;
  employee_no?: string;
  employment_type?: EmploymentType;
  hire_date?: string; // YYYY-MM-DD
  phone?: string;
  email?: string;
  notes?: string;
};

export function createPersonel(input: CreatePersonelInput): Promise<StaffMember> {
  return api().post<StaffMember>("/api/staff", input);
}
