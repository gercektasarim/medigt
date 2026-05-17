import { api } from "../api/client";
import type {
  Medication,
  MedicationForm,
  PrescriptionClass,
} from "../types/medication";

export type CreateMedicationInput = {
  atc_code?: string;
  barcode?: string;
  name: string;
  generic_name?: string;
  form?: MedicationForm;
  strength?: string;
  pack_size?: string;
  prescription_class?: PrescriptionClass;
  requires_cold_chain?: boolean;
  is_controlled?: boolean;
  manufacturer?: string;
  list_price?: number;
  notes?: string;
};

export function createMedication(input: CreateMedicationInput): Promise<Medication> {
  return api().post<Medication>("/api/medications", input);
}
