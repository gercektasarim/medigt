"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, FileText, Send } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { invoiceListOptions } from "@medigt/core/fatura";
import {
  cancelInvoiceSubmission,
  createInvoiceSubmission,
  invoiceSubmissionListOptions,
  medulaKeys,
} from "@medigt/core/medula";
import { formatTl } from "@medigt/core/utils";
import type {
  Invoice,
  MedulaInvoiceSubmission,
  MedulaSubmitStatus,
} from "@medigt/core/types";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
} from "../../common/form-fields";
import { SUBMIT_STATUS_COLORS, SUBMIT_STATUS_LABELS } from "./labels";

export function MedulaSubmissionsTab() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const [statusFilter, setStatusFilter] = useState<MedulaSubmitStatus | "">("");
  const list = useQuery(invoiceSubmissionListOptions(branchId, statusFilter || undefined));
  const [createOpen, setCreateOpen] = useState(false);
  const [cancelTarget, setCancelTarget] = useState<MedulaInvoiceSubmission | null>(null);

  const columns: Column<MedulaInvoiceSubmission>[] = [
    {
      key: "at",
      header: "Tarih",
      cell: (r) => new Date(r.requested_at).toLocaleString("tr-TR"),
    },
    {
      key: "invoice",
      header: "Fatura",
      cell: (r) => (
        <div>
          {r.invoice_no && <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.invoice_no}</code>}
          {r.total !== undefined && r.total > 0 && (
            <div className="mt-0.5 font-mono text-xs">{formatTl(r.total)}</div>
          )}
        </div>
      ),
    },
    {
      key: "patient",
      header: "Hasta",
      cell: (r) => r.patient_first_name ? (
        <div>
          <div className="font-medium">{r.patient_first_name} {r.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">MRN {r.patient_mrn}</div>
        </div>
      ) : <span className="text-xs text-muted-foreground">—</span>,
    },
    {
      key: "sgk",
      header: "SGK No / Batch",
      cell: (r) => (
        <div className="text-xs">
          {r.sgk_invoice_no && <code className="rounded bg-muted px-1 py-0.5">{r.sgk_invoice_no}</code>}
          {r.batch_no && <div className="mt-0.5 text-muted-foreground">Batch: {r.batch_no}</div>}
          {!r.sgk_invoice_no && !r.batch_no && <span className="text-muted-foreground">—</span>}
        </div>
      ),
    },
    {
      key: "status",
      header: "Durum",
      cell: (r) => (
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${SUBMIT_STATUS_COLORS[r.status]}`}>
          {SUBMIT_STATUS_LABELS[r.status]}
        </span>
      ),
    },
    {
      key: "actions",
      header: "",
      cell: (r) => (
        r.status === "submitted" || r.status === "accepted" ? (
          <button
            type="button"
            onClick={() => setCancelTarget(r)}
            className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs text-[var(--critical)] hover:bg-muted"
          >
            <Ban className="h-3.5 w-3.5" /> İptal
          </button>
        ) : null
      ),
      className: "text-right",
    },
  ];

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Field id="sub-status" label="Durum">
          <SelectInput
            id="sub-status"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as MedulaSubmitStatus | "")}
            className="max-w-xs"
          >
            <option value="">Tümü</option>
            {Object.entries(SUBMIT_STATUS_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
          <span className="inline-flex items-center gap-1"><Send className="h-4 w-4" /> SGK'ya Gönder</span>
        </PrimaryButton>
      </div>

      {list.isLoading ? (
        <div className="empty-state">Yükleniyor...</div>
      ) : (list.data ?? []).length === 0 ? (
        <div className="empty-state">Henüz SGK fatura gönderimi yok.</div>
      ) : (
        <DataTable<MedulaInvoiceSubmission>
          rows={list.data ?? []}
          rowKey={(r) => r.id}
          columns={columns}
        />
      )}

      <CreateSubmissionSheet
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        branchId={branchId}
      />
      {cancelTarget && (
        <CancelSubmissionSheet
          submission={cancelTarget}
          branchId={branchId}
          onClose={() => setCancelTarget(null)}
        />
      )}
    </div>
  );
}

function CreateSubmissionSheet({
  open,
  onClose,
  branchId,
}: {
  open: boolean;
  onClose: () => void;
  branchId: string;
}) {
  const qc = useQueryClient();
  // Sadece onaylı + tahsil edilmemiş veya ödenmiş faturalar SGK'ya gönderilebilir.
  // Mock-mode: tüm finalized + paid'i izin verelim.
  const invoices = useQuery(invoiceListOptions(branchId, { status: "paid" }));
  const [invoiceId, setInvoiceId] = useState("");

  const create = useMutation({
    mutationFn: () => createInvoiceSubmission({ invoice_id: invoiceId }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: medulaKeys.all(branchId) });
      setInvoiceId("");
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Fatura SGK Gönderim">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <p className="text-sm text-muted-foreground">
          Yalnızca <strong>ödenmiş</strong> faturalar SGK'ya gönderilebilir (mock kısıtı).
          Gerçek SGK ortamında onay süreci farklılaşacak.
        </p>
        <Field id="sub-inv" label="Fatura" required>
          <SelectInput id="sub-inv" required value={invoiceId} onChange={(e) => setInvoiceId(e.target.value)}>
            <option value="">— Seçiniz —</option>
            {((invoices.data ?? []) as Invoice[]).map((inv) => (
              <option key={inv.id} value={inv.id}>
                {inv.invoice_no} · {inv.patient_first_name} {inv.patient_last_name} · {formatTl(inv.total)}
              </option>
            ))}
          </SelectInput>
        </Field>
        {create.isError && <p className="text-sm text-[var(--critical)]">{(create.error as Error)?.message}</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !invoiceId}>
            <span className="inline-flex items-center gap-1">
              <FileText className="h-4 w-4" /> {create.isPending ? "Gönderiliyor..." : "Kuyruğa gönder"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function CancelSubmissionSheet({
  submission,
  branchId,
  onClose,
}: {
  submission: MedulaInvoiceSubmission;
  branchId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [reason, setReason] = useState("");
  const cancel = useMutation({
    mutationFn: () => cancelInvoiceSubmission(submission.id, reason || undefined),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: medulaKeys.all(branchId) });
      onClose();
    },
  });
  return (
    <SideSheet open onClose={onClose} title={`SGK Fatura İptal · ${submission.sgk_invoice_no ?? submission.id.slice(0, 8)}`}>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); cancel.mutate(); }}>
        <Field id="cancel-sub-reason" label="İptal nedeni">
          <Textarea id="cancel-sub-reason" rows={3} value={reason} onChange={(e) => setReason(e.target.value)} />
        </Field>
        {cancel.isError && <p className="text-sm text-[var(--critical)]">{(cancel.error as Error)?.message}</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">Vazgeç</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={cancel.isPending}>
            {cancel.isPending ? "Gönderiliyor..." : "İptal et"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
