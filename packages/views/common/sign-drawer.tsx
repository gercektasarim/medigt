"use client";

// SignDrawer — reusable e-imza (TURKKEP) onay drawer'ı.
//
// Kullanım:
//   const [sigID, setSigID] = useState<string | null>(null);
//   const init = useMutation({ ... initSignature(...) });
//   <SignDrawer
//     signatureId={sigID}
//     onClose={() => setSigID(null)}
//     onSigned={(sig) => { ... sig.id'i hedef hareketi tetiklemek için kullan ... }}
//   />
//
// Drawer açıkken her 2 saniyede bir pollSignature çağırılır; status
// 'signed' / 'failed' / 'cancelled' / 'expired' olunca polling durur.

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, Loader2, ShieldCheck, X, XCircle } from "lucide-react";
import {
  cancelSignature,
  imzaKeys,
  pollSignature,
  signatureOptions,
} from "@medigt/core/imza";
import type { DigitalSignature, SignatureStatus } from "@medigt/core/types";
import { SideSheet } from "./side-sheet";
import { PrimaryButton, SecondaryButton } from "./form-fields";

const STATUS_LABELS: Record<SignatureStatus, string> = {
  pending: "Onay bekleniyor",
  in_progress: "Onay alınıyor",
  signed: "İmzalandı",
  cancelled: "İptal edildi",
  failed: "Başarısız",
  expired: "Süre doldu",
};

const TERMINAL: SignatureStatus[] = ["signed", "cancelled", "failed", "expired"];

export function SignDrawer({
  signatureId,
  onClose,
  onSigned,
  title = "e-İmza ile Onayla",
  hint,
}: {
  signatureId: string | null;
  onClose: () => void;
  onSigned: (sig: DigitalSignature) => void;
  title?: string;
  hint?: string;
}) {
  const qc = useQueryClient();
  const enabled = !!signatureId;

  const sig = useQuery({
    ...signatureOptions(signatureId ?? ""),
    enabled,
  });

  // Server-side polling: backend Poll endpoint advances the state machine
  // by calling TURKKEP. We hit it on a loop until terminal.
  const advance = useMutation({
    mutationFn: () => pollSignature(signatureId!),
    onSuccess: (data) => {
      qc.setQueryData(imzaKeys.detail(signatureId!), data);
    },
  });

  const cancel = useMutation({
    mutationFn: () => cancelSignature(signatureId!),
    onSuccess: () => qc.invalidateQueries({ queryKey: imzaKeys.detail(signatureId!) }),
  });

  // Drive the polling loop.
  const status = sig.data?.status;
  const isTerminal = status ? TERMINAL.includes(status) : false;
  const [pollEnabled, setPollEnabled] = useState(true);

  useEffect(() => {
    if (!enabled) return;
    if (isTerminal) {
      setPollEnabled(false);
      return;
    }
    if (!pollEnabled) return;
    const t = setInterval(() => {
      advance.mutate();
    }, 1800);
    return () => clearInterval(t);
  }, [enabled, isTerminal, pollEnabled, advance]);

  // Notify caller exactly once on first signed transition.
  const [notified, setNotified] = useState(false);
  useEffect(() => {
    if (!notified && sig.data && sig.data.status === "signed") {
      setNotified(true);
      onSigned(sig.data);
    }
  }, [sig.data, notified, onSigned]);

  if (!signatureId) return null;

  return (
    <SideSheet open onClose={onClose} title={title}>
      <div className="space-y-4">
        {hint && <p className="text-sm text-muted-foreground">{hint}</p>}

        {sig.isLoading && <p className="text-sm text-muted-foreground">Yükleniyor...</p>}

        {sig.data && (
          <>
            <StatusBanner status={sig.data.status} error={sig.data.error_message} />

            {(sig.data.status === "pending" || sig.data.status === "in_progress") && (
              <div className="space-y-3 rounded-md border border-border bg-card p-4">
                <p className="text-sm">
                  TURKKEP mobil uygulamanızda gelen onay isteğini açın. Onay kodu:
                </p>
                <div className="text-center font-mono text-3xl tracking-[0.4em] font-semibold">
                  {sig.data.challenge_code ?? "—"}
                </div>
                <p className="text-xs text-muted-foreground">
                  Onay isteğini onayladığınızda burası otomatik güncellenecek
                  (her ~2 saniyede bir kontrol ediliyor).
                </p>
              </div>
            )}

            {sig.data.status === "signed" && sig.data.certificate_subject && (
              <div className="space-y-1 rounded-md border border-emerald-200 bg-emerald-50 p-3 text-xs dark:border-emerald-900 dark:bg-emerald-950/30">
                <div className="text-emerald-900 dark:text-emerald-200">
                  <strong>Sertifika:</strong> {sig.data.certificate_subject}
                </div>
                {sig.data.certificate_serial && (
                  <div className="text-emerald-900 dark:text-emerald-200">
                    <strong>Seri:</strong> <code className="rounded bg-emerald-100 px-1 dark:bg-emerald-900/60">{sig.data.certificate_serial}</code>
                  </div>
                )}
              </div>
            )}

            <details className="text-xs text-muted-foreground">
              <summary className="cursor-pointer">Doküman özeti (SHA-256)</summary>
              <code className="mt-1 block break-all rounded bg-muted px-2 py-1">{sig.data.document_hash}</code>
            </details>
          </>
        )}

        <div className="flex gap-2">
          {!isTerminal ? (
            <>
              <SecondaryButton
                type="button"
                onClick={() => cancel.mutate()}
                disabled={cancel.isPending}
                className="flex-1"
              >
                <span className="inline-flex items-center gap-1">
                  <X className="h-4 w-4" /> İptal Et
                </span>
              </SecondaryButton>
              <SecondaryButton type="button" onClick={onClose} className="flex-1">
                Kapat
              </SecondaryButton>
            </>
          ) : (
            <PrimaryButton type="button" onClick={onClose} className="flex-1">
              Tamam
            </PrimaryButton>
          )}
        </div>
      </div>
    </SideSheet>
  );
}

function StatusBanner({ status, error }: { status: SignatureStatus; error?: string }) {
  if (status === "signed") {
    return (
      <div className="flex items-center gap-2 rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200">
        <CheckCircle2 className="h-5 w-5 shrink-0" />
        <div>
          <strong>İmzalandı.</strong> {STATUS_LABELS[status]}
        </div>
      </div>
    );
  }
  if (status === "failed" || status === "expired" || status === "cancelled") {
    return (
      <div className="flex items-start gap-2 rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-900 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-200">
        <XCircle className="mt-0.5 h-5 w-5 shrink-0" />
        <div>
          <div><strong>{STATUS_LABELS[status]}</strong></div>
          {error && <div className="text-xs">{error}</div>}
        </div>
      </div>
    );
  }
  return (
    <div className="flex items-start gap-2 rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200">
      {status === "in_progress" ? <Loader2 className="mt-0.5 h-5 w-5 shrink-0 animate-spin" /> : <ShieldCheck className="mt-0.5 h-5 w-5 shrink-0" />}
      <div>{STATUS_LABELS[status]}</div>
    </div>
  );
}
