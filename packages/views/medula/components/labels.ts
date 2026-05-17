import type {
  MedulaEraportKind,
  MedulaEraportStatus,
  MedulaProvisionStatus,
  MedulaProvisionType,
  MedulaReferralStatus,
  MedulaReferralType,
  MedulaSubmitStatus,
} from "@medigt/core/types";

export const PROVISION_STATUS_LABELS: Record<MedulaProvisionStatus, string> = {
  pending: "Kuyrukta",
  in_progress: "Gönderiliyor",
  completed: "Tamamlandı",
  failed: "Reddedildi",
  cancelled: "İptal",
};

export const PROVISION_STATUS_COLORS: Record<MedulaProvisionStatus, string> = {
  pending: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  in_progress: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  completed: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  failed: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
  cancelled: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
};

export const PROVISION_TYPE_LABELS: Record<MedulaProvisionType, string> = {
  normal: "Normal",
  acil: "Acil",
  yatis: "Yatış",
};

export const SUBMIT_STATUS_LABELS: Record<MedulaSubmitStatus, string> = {
  pending: "Kuyrukta",
  in_progress: "Gönderiliyor",
  submitted: "Gönderildi",
  accepted: "Kabul",
  rejected: "Reddedildi",
  cancelled: "İptal",
  failed: "Başarısız",
};

export const SUBMIT_STATUS_COLORS: Record<MedulaSubmitStatus, string> = {
  pending: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  in_progress: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  submitted: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  accepted: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  rejected: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
  cancelled: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
  failed: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};

export const REFERRAL_STATUS_LABELS: Record<MedulaReferralStatus, string> = {
  pending: "Kuyrukta",
  in_progress: "Gönderiliyor",
  created: "Oluşturuldu",
  rejected: "Reddedildi",
  cancelled: "İptal",
  failed: "Başarısız",
};

export const REFERRAL_STATUS_COLORS: Record<MedulaReferralStatus, string> = {
  pending: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  in_progress: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  created: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  rejected: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
  cancelled: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
  failed: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};

export const REFERRAL_TYPE_LABELS: Record<MedulaReferralType, string> = {
  normal: "Normal",
  acil: "Acil",
  kontrol: "Kontrol",
};

export const ERAPORT_KIND_LABELS: Record<MedulaEraportKind, string> = {
  chronic_drug: "Kronik İlaç",
  inpatient: "Yatış",
  work_incapacity: "İş Göremezlik",
  special_procedure: "Özel Girişim",
};

export const ERAPORT_STATUS_LABELS: Record<MedulaEraportStatus, string> = {
  pending: "Kuyrukta",
  in_progress: "Gönderiliyor",
  submitted: "Gönderildi",
  approved: "Onaylı",
  rejected: "Reddedildi",
  cancelled: "İptal",
  failed: "Başarısız",
};

export const ERAPORT_STATUS_COLORS: Record<MedulaEraportStatus, string> = {
  pending: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  in_progress: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  submitted: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  approved: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  rejected: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
  cancelled: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
  failed: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};
