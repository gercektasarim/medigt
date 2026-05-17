import type { Uuid } from "./common";

export type MedicationForm =
  | "tablet"
  | "capsule"
  | "syrup"
  | "injection"
  | "ampoule"
  | "cream"
  | "ointment"
  | "drops"
  | "spray"
  | "patch"
  | "suppository"
  | "solution"
  | "powder"
  | "other";

export type PrescriptionClass =
  | "otc"
  | "normal"
  | "green"
  | "red"
  | "orange"
  | "purple";

export type Medication = {
  id: Uuid;
  atc_code?: string;
  barcode?: string;
  name: string;
  generic_name?: string;
  form: MedicationForm;
  strength?: string;
  pack_size?: string;
  prescription_class: PrescriptionClass;
  requires_cold_chain: boolean;
  is_controlled: boolean;
  manufacturer?: string;
  list_price?: number;
  notes?: string;
  is_active: boolean;
};
