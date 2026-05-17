import type { InvoiceStatus } from "@medigt/core/types";

export const STATUS_LABELS: Record<InvoiceStatus, string> = {
  draft: "Taslak",
  finalized: "Onaylı",
  paid: "Ödendi",
  cancelled: "İptal",
};

export const STATUS_COLORS: Record<InvoiceStatus, string> = {
  draft: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  finalized: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  paid: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  cancelled: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};
