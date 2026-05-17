import type { Timestamps, Uuid } from "./common";

export type Specialization = {
  id: Uuid;
  organization_id?: Uuid | null;
  code: string;
  name: string;
  parent_id?: Uuid | null;
  is_system: boolean;
} & Timestamps;

export type EmploymentType = "full_time" | "part_time" | "contract" | "consultant" | "intern";

export type StaffMember = {
  id: Uuid;
  organization_id: Uuid;
  user_id?: Uuid;
  employee_no?: string;
  first_name: string;
  last_name: string;
  title?: string;
  employment_type: EmploymentType;
  hire_date?: string;
  phone?: string;
  email?: string;
  notes?: string;
  is_active: boolean;
} & Timestamps;

export type Doctor = {
  id: Uuid;
  staff_member_id: Uuid;
  staff: StaffMember;
  diploma_no?: string;
  medula_doctor_code?: string;
  license_expires_at?: string;
  is_accepting_patients: boolean;
  specializations: Specialization[];
} & Timestamps;
