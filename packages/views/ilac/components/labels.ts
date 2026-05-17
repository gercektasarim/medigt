import type { MedicationForm, PrescriptionClass } from "@medigt/core/types";

export const MEDICATION_FORM_LABELS: Record<MedicationForm, string> = {
  tablet: "Tablet",
  capsule: "Kapsül",
  syrup: "Şurup",
  injection: "Enjeksiyon",
  ampoule: "Ampul",
  cream: "Krem",
  ointment: "Pomad",
  drops: "Damla",
  spray: "Sprey",
  patch: "Bant / Flaster",
  suppository: "Fitil",
  solution: "Solüsyon",
  powder: "Toz",
  other: "Diğer",
};

export const PRESCRIPTION_CLASS_LABELS: Record<PrescriptionClass, string> = {
  otc: "Reçetesiz",
  normal: "Normal (beyaz)",
  green: "Yeşil reçete",
  red: "Kırmızı reçete",
  orange: "Turuncu reçete",
  purple: "Mor reçete",
};

export const PRESCRIPTION_CLASS_COLORS: Record<PrescriptionClass, string> = {
  otc: "bg-slate-100 text-slate-700 dark:bg-slate-800/60 dark:text-slate-200",
  normal: "bg-slate-100 text-slate-700 dark:bg-slate-800/60 dark:text-slate-200",
  green: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  red: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
  orange: "bg-orange-100 text-orange-900 dark:bg-orange-950/40 dark:text-orange-200",
  purple: "bg-violet-100 text-violet-900 dark:bg-violet-950/40 dark:text-violet-200",
};
