export type SystemRole = "platform_admin" | "org_owner" | "org_admin" | "org_member";

export type ClinicalRole =
  | "clinical_lead"
  | "doctor"
  | "doctor_assistant"
  | "nurse"
  | "reception"
  | "lab_technician"
  | "radiology_technician"
  | "radiologist"
  | "pharmacist"
  | "cashier"
  | "finance_lead"
  | "accountant"
  | "admin_clerk";

export type Permission = {
  code: string;
  module: string;
  action: string;
  description?: string;
};
