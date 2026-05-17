"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Calendar, CheckCircle2, CreditCard, FileText, Undo2, XCircle } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  cancelInvoice,
  faturaKeys,
  finalizeInvoice,
  invoiceDetailOptions,
  invoicePaymentsOptions,
  recordPayment,
  type PaymentAllocationInput,
} from "@medigt/core/fatura";
import {
  cariKeys,
  createInstallmentPlan,
  installmentPlanOptions,
  invoiceRefundsOptions,
  processRefund,
  type CreateInstallmentPlanInput,
  type ProcessRefundInput,
} from "@medigt/core/cari";
import { myRegisterOptions, vezneKeys } from "@medigt/core/vezne";
import { formatTl } from "@medigt/core/utils";
import type {
  Invoice,
  InvoiceDetail,
  InvoicePaymentSummary,
  PaymentMethod,
} from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";
import { STATUS_COLORS, STATUS_LABELS } from "./labels";

const METHOD_LABELS: Record<PaymentMethod, string> = {
  cash: "Nakit",
  card: "Kart",
  transfer: "Havale",
  mobile: "Mobil",
  other: "Diğer / Mahsup",
};

export function InvoiceDetailPage({ invoiceId }: { invoiceId: string }) {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const detail = useQuery(invoiceDetailOptions(branchId, invoiceId));

  if (detail.isLoading) {
    return (
      <DashboardLayout>
        <div className="page-shell">Yükleniyor...</div>
      </DashboardLayout>
    );
  }
  if (detail.isError || !detail.data) {
    return (
      <DashboardLayout>
        <div className="page-shell">
          <div className="empty-state text-[var(--critical)]">Fatura bulunamadı.</div>
        </div>
      </DashboardLayout>
    );
  }

  const inv = detail.data.invoice;

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title={`Fatura · ${inv.invoice_no}`}
          subtitle={`${inv.patient_first_name} ${inv.patient_last_name} · MRN ${inv.patient_mrn}${
            inv.institution_name ? ` · ${inv.institution_name}` : " · Cepten ödeme"
          }`}
          actions={<HeaderActions invoice={inv} branchId={branchId} />}
        />

        <Meta invoice={inv} />

        <ItemsTable detail={detail.data} />

        <PaymentsSection invoice={inv} branchId={branchId} />
      </div>
    </DashboardLayout>
  );
}

function Meta({ invoice }: { invoice: Invoice }) {
  return (
    <div className="grid grid-cols-2 gap-3 rounded-md border border-border bg-card p-3 text-sm sm:grid-cols-5">
      <Cell label="Durum" value={
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${STATUS_COLORS[invoice.status]}`}>
          {STATUS_LABELS[invoice.status]}
        </span>
      } />
      <Cell label="Ara Toplam" value={<span className="font-mono">{formatTl(invoice.subtotal)}</span>} />
      <Cell label="KDV" value={<span className="font-mono">{formatTl(invoice.tax_total)}</span>} />
      <Cell label="Genel Toplam" value={<span className="font-mono font-semibold">{formatTl(invoice.total)}</span>} />
      <Cell label="Kalan Bakiye" value={
        <span className={"font-mono font-semibold " + (invoice.balance_due > 0 ? "text-[var(--critical)]" : "text-emerald-700")}>
          {formatTl(invoice.balance_due)}
        </span>
      } />
      {invoice.issued_at && (
        <Cell label="Onay" value={new Date(invoice.issued_at).toLocaleString("tr-TR")} />
      )}
      {invoice.cancelled_at && (
        <Cell label="İptal" value={new Date(invoice.cancelled_at).toLocaleString("tr-TR")} />
      )}
      {invoice.notes && (
        <div className="col-span-full">
          <div className="text-xs text-muted-foreground">Notlar</div>
          <div className="whitespace-pre-line">{invoice.notes}</div>
        </div>
      )}
    </div>
  );
}

function Cell({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-sm">{value}</div>
    </div>
  );
}

function HeaderActions({ invoice, branchId }: { invoice: Invoice; branchId: string }) {
  const qc = useQueryClient();
  const finalize = useMutation({
    mutationFn: () => finalizeInvoice(invoice.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: faturaKeys.all(branchId) }),
  });
  const cancel = useMutation({
    mutationFn: () => cancelInvoice(invoice.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: faturaKeys.all(branchId) }),
  });
  const [payOpen, setPayOpen] = useState(false);
  const [refundOpen, setRefundOpen] = useState(false);
  const [planOpen, setPlanOpen] = useState(false);

  if (invoice.status === "draft") {
    return (
      <div className="flex gap-2">
        <SecondaryButton type="button" onClick={() => cancel.mutate()} disabled={cancel.isPending}>
          <span className="inline-flex items-center gap-1"><XCircle className="h-4 w-4" /> İptal</span>
        </SecondaryButton>
        <PrimaryButton type="button" onClick={() => finalize.mutate()} disabled={finalize.isPending}>
          <span className="inline-flex items-center gap-1"><CheckCircle2 className="h-4 w-4" /> Onayla</span>
        </PrimaryButton>
      </div>
    );
  }

  const isPayable = invoice.status === "finalized" && invoice.balance_due > 0;
  const isRefundable = (invoice.status === "paid" || invoice.status === "finalized") && invoice.paid_total > 0;

  return (
    <>
      <div className="flex flex-wrap gap-2">
        {invoice.status === "finalized" && invoice.paid_total === 0 && (
          <SecondaryButton type="button" onClick={() => cancel.mutate()} disabled={cancel.isPending}>
            <span className="inline-flex items-center gap-1"><XCircle className="h-4 w-4" /> İptal</span>
          </SecondaryButton>
        )}
        {isRefundable && (
          <SecondaryButton type="button" onClick={() => setRefundOpen(true)}>
            <span className="inline-flex items-center gap-1"><Undo2 className="h-4 w-4" /> İade</span>
          </SecondaryButton>
        )}
        {isPayable && (
          <SecondaryButton type="button" onClick={() => setPlanOpen(true)}>
            <span className="inline-flex items-center gap-1"><Calendar className="h-4 w-4" /> Taksitlendir</span>
          </SecondaryButton>
        )}
        {isPayable && (
          <PrimaryButton type="button" onClick={() => setPayOpen(true)}>
            <span className="inline-flex items-center gap-1"><CreditCard className="h-4 w-4" /> Ödeme Al</span>
          </PrimaryButton>
        )}
      </div>
      {payOpen && (
        <PaymentSheet
          invoice={invoice}
          branchId={branchId}
          onClose={() => setPayOpen(false)}
        />
      )}
      {refundOpen && (
        <RefundSheet
          invoice={invoice}
          branchId={branchId}
          onClose={() => setRefundOpen(false)}
        />
      )}
      {planOpen && (
        <InstallmentPlanSheet
          invoice={invoice}
          branchId={branchId}
          onClose={() => setPlanOpen(false)}
        />
      )}
    </>
  );
}

function ItemsTable({ detail }: { detail: InvoiceDetail }) {
  type Row = typeof detail.items[number];
  const columns: Column<Row>[] = [
    { key: "sort", header: "#", cell: (r) => <span className="text-xs text-muted-foreground">{r.sort_order + 1}</span> },
    { key: "code", header: "Kod", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.code}</code> },
    { key: "name", header: "Hizmet", cell: (r) => <span className="font-medium">{r.name}</span> },
    { key: "qty", header: "Mik", cell: (r) => r.quantity, className: "text-right" },
    { key: "up", header: "Birim", cell: (r) => formatTl(r.unit_price), className: "text-right" },
    { key: "disc", header: "İsk %", cell: (r) => r.discount_pct.toFixed(1), className: "text-right" },
    { key: "vat", header: "KDV %", cell: (r) => r.vat_rate.toFixed(1), className: "text-right" },
    { key: "sub", header: "Ara", cell: (r) => formatTl(r.line_subtotal), className: "text-right" },
    { key: "tax", header: "KDV", cell: (r) => formatTl(r.line_tax), className: "text-right" },
    {
      key: "tot",
      header: "Toplam",
      cell: (r) => <span className="font-mono font-medium">{formatTl(r.line_total)}</span>,
      className: "text-right",
    },
  ];
  return (
    <section>
      <h2 className="mb-2 flex items-center gap-2 text-sm font-semibold">
        <FileText className="h-4 w-4" /> Kalemler
      </h2>
      <DataTable<Row> rows={detail.items} rowKey={(r) => r.id} columns={columns} />
    </section>
  );
}

function PaymentsSection({ invoice, branchId }: { invoice: Invoice; branchId: string }) {
  const list = useQuery(invoicePaymentsOptions(branchId, invoice.id));
  if (list.isLoading) return null;
  if ((list.data ?? []).length === 0) return null;

  const columns: Column<InvoicePaymentSummary>[] = [
    {
      key: "at",
      header: "Tarih",
      cell: (r) => new Date(r.received_at).toLocaleString("tr-TR"),
    },
    { key: "no", header: "Tahsilat No", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.payment_no}</code> },
    { key: "method", header: "Yöntem", cell: (r) => METHOD_LABELS[r.method] ?? r.method },
    {
      key: "amount",
      header: "Toplam Tahsilat",
      cell: (r) => formatTl(r.amount),
      className: "text-right",
    },
    {
      key: "alloc",
      header: "Bu Faturaya",
      cell: (r) => <span className="font-mono font-medium">{formatTl(r.allocated_to_this_invoice)}</span>,
      className: "text-right",
    },
    {
      key: "ref",
      header: "Ref / Not",
      cell: (r) => <span className="text-xs text-muted-foreground">{r.reference ?? ""}</span>,
    },
  ];
  return (
    <section>
      <h2 className="mb-2 flex items-center gap-2 text-sm font-semibold">
        <CreditCard className="h-4 w-4" /> Tahsilatlar
      </h2>
      <DataTable<InvoicePaymentSummary> rows={list.data ?? []} rowKey={(r) => r.id} columns={columns} />
    </section>
  );
}

function PaymentSheet({
  invoice,
  branchId,
  onClose,
}: {
  invoice: Invoice;
  branchId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const register = useQuery(myRegisterOptions(branchId));
  const [method, setMethod] = useState<PaymentMethod>("cash");
  const [amount, setAmount] = useState(invoice.balance_due.toFixed(2));
  const [reference, setReference] = useState("");
  const [notes, setNotes] = useState("");

  const save = useMutation({
    mutationFn: () => {
      const allocations: PaymentAllocationInput[] = [
        { invoice_id: invoice.id, amount: Number(amount) },
      ];
      return recordPayment({
        patient_id: invoice.patient_id,
        method,
        amount: Number(amount),
        reference: reference.trim() || undefined,
        notes: notes.trim() || undefined,
        cash_register_id: method === "cash" ? register.data?.id : undefined,
        allocations,
      });
    },
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: faturaKeys.all(branchId) }),
        qc.invalidateQueries({ queryKey: vezneKeys.all(branchId) }),
      ]);
      onClose();
    },
  });

  const amountNum = Number(amount);
  const overAlloc = amountNum > invoice.balance_due + 0.005;
  const needsRegister = method === "cash" && !register.data;
  const canSubmit = amountNum > 0 && !overAlloc && !needsRegister && !save.isPending;

  return (
    <SideSheet open onClose={onClose} title={`Ödeme · ${invoice.invoice_no}`}>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
        <div className="rounded-md border border-border bg-muted/40 p-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Kalan bakiye</span>
            <span className="font-mono font-medium">{formatTl(invoice.balance_due)}</span>
          </div>
        </div>

        <Field id="p-method" label="Ödeme yöntemi">
          <SelectInput
            id="p-method"
            value={method}
            onChange={(e) => setMethod(e.target.value as PaymentMethod)}
          >
            {Object.entries(METHOD_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>

        {method === "cash" && (
          <div className={
            "rounded-md border p-3 text-xs " +
            (register.data
              ? "border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200"
              : "border-rose-200 bg-rose-50 text-rose-900 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-200")
          }>
            {register.data ? (
              <>Açık kasanız: <code className="rounded bg-emerald-100 px-1 dark:bg-emerald-900/60">{register.data.register_no}</code> ({register.data.cashier_name}) — tahsilat bu kasaya işlenecek.</>
            ) : (
              <>Nakit tahsilat için önce <strong>Vezne</strong> sayfasından bir kasa açmanız gerekiyor.</>
            )}
          </div>
        )}

        <Field id="p-amount" label="Tutar (TRY)" required>
          <TextInput
            id="p-amount"
            type="number"
            min="0"
            step="0.01"
            required
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </Field>
        {overAlloc && <p className="text-xs text-[var(--critical)]">Kalan bakiyeyi aşıyor.</p>}

        <Field id="p-ref" label="Referans (POS, havale no)">
          <TextInput id="p-ref" value={reference} onChange={(e) => setReference(e.target.value)} />
        </Field>
        <Field id="p-notes" label="Notlar">
          <Textarea id="p-notes" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>

        {save.isError && (
          <p className="text-sm text-[var(--critical)]">Tahsilat başarısız: {(save.error as Error)?.message}</p>
        )}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            <span className="inline-flex items-center gap-1">
              <CreditCard className="h-4 w-4" /> {save.isPending ? "Kaydediliyor..." : "Tahsilatı kaydet"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

// ---------- Refund drawer ----------

function RefundSheet({
  invoice,
  branchId,
  onClose,
}: {
  invoice: Invoice;
  branchId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const register = useQuery(myRegisterOptions(branchId));
  const existing = useQuery(invoiceRefundsOptions(invoice.id));
  const refundedTotal = (existing.data ?? []).reduce((s, r) => s + r.amount, 0);
  const refundable = Math.max(0, invoice.paid_total - refundedTotal);

  const [method, setMethod] = useState<PaymentMethod>("cash");
  const [toAdvance, setToAdvance] = useState(false);
  const [amount, setAmount] = useState(refundable.toFixed(2));
  const [reason, setReason] = useState("");

  const save = useMutation({
    mutationFn: () => {
      const input: ProcessRefundInput = {
        patient_id: invoice.patient_id,
        invoice_id: invoice.id,
        amount: Number(amount),
        method,
        cash_register_id: method === "cash" && !toAdvance ? register.data?.id : undefined,
        to_advance: toAdvance,
        reason: reason.trim() || undefined,
      };
      return processRefund(input);
    },
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: faturaKeys.all(branchId) }),
        qc.invalidateQueries({ queryKey: vezneKeys.all(branchId) }),
        qc.invalidateQueries({ queryKey: cariKeys.invoiceRefunds(invoice.id) }),
        qc.invalidateQueries({ queryKey: cariKeys.patientAccount(invoice.patient_id) }),
      ]);
      onClose();
    },
  });

  const amountNum = Number(amount);
  const overRefund = amountNum > refundable + 0.005;
  const cashNeedsKasa = method === "cash" && !toAdvance && !register.data;
  const canSubmit = amountNum > 0 && !overRefund && !cashNeedsKasa && !save.isPending;

  return (
    <SideSheet open onClose={onClose} title={`Fatura İadesi · ${invoice.invoice_no}`}>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
        <div className="space-y-1 rounded-md border border-border bg-muted/40 p-3 text-sm">
          <Row label="Tahsil edilen" value={formatTl(invoice.paid_total)} />
          <Row label="Önceden iade" value={formatTl(refundedTotal)} />
          <Row label="İade edilebilir" value={formatTl(refundable)} strong />
        </div>

        <Field id="rf-method" label="Yöntem">
          <SelectInput value={method} onChange={(e) => setMethod(e.target.value as PaymentMethod)} id="rf-method">
            <option value="cash">Nakit</option>
            <option value="card">Kart</option>
            <option value="transfer">Havale</option>
            <option value="other">Diğer (mahsup)</option>
          </SelectInput>
        </Field>

        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={toAdvance}
            onChange={(e) => setToAdvance(e.target.checked)}
            className="h-4 w-4 rounded border-input"
          />
          Nakit ödemek yerine hastanın avans hesabına yaz
        </label>

        {method === "cash" && !toAdvance && (
          <div className={
            "rounded-md border p-3 text-xs " +
            (register.data
              ? "border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200"
              : "border-rose-200 bg-rose-50 text-rose-900 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-200")
          }>
            {register.data
              ? <>Açık kasa: <code className="rounded bg-emerald-100 px-1 dark:bg-emerald-900/60">{register.data.register_no}</code></>
              : <>Nakit iade için <strong>Vezne</strong> sayfasından bir kasa açın.</>}
          </div>
        )}

        <Field id="rf-amount" label="İade tutarı (TRY)" required>
          <TextInput
            id="rf-amount"
            type="number"
            min="0"
            step="0.01"
            required
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </Field>
        {overRefund && <p className="text-xs text-[var(--critical)]">İade edilebilir tutarı aşıyor.</p>}

        <Field id="rf-reason" label="İade nedeni">
          <Textarea id="rf-reason" rows={3} value={reason} onChange={(e) => setReason(e.target.value)} />
        </Field>

        {save.isError && <p className="text-sm text-[var(--critical)]">İade başarısız: {(save.error as Error)?.message}</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            <span className="inline-flex items-center gap-1">
              <Undo2 className="h-4 w-4" /> {save.isPending ? "İşleniyor..." : "İadeyi işle"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function Row({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-muted-foreground">{label}</span>
      <span className={"font-mono " + (strong ? "font-semibold" : "")}>{value}</span>
    </div>
  );
}

// ---------- Installment plan drawer ----------

function InstallmentPlanSheet({
  invoice,
  branchId: _branchId,
  onClose,
}: {
  invoice: Invoice;
  branchId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const existing = useQuery(installmentPlanOptions(invoice.id));
  const [num, setNum] = useState("3");
  const [firstDue, setFirstDue] = useState(new Date().toISOString().slice(0, 10));
  const [interval, setInterval] = useState("30");
  const [notes, setNotes] = useState("");

  const save = useMutation({
    mutationFn: () => {
      const input: CreateInstallmentPlanInput = {
        num_installments: Number(num),
        first_due_date: firstDue,
        interval_days: Number(interval),
        notes: notes.trim() || undefined,
      };
      return createInstallmentPlan(invoice.id, input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: cariKeys.installmentPlan(invoice.id) });
    },
  });

  // If plan already exists, show installment list (read-only for now).
  if (existing.data) {
    return (
      <SideSheet open onClose={onClose} title={`Taksit Planı · ${invoice.invoice_no}`}>
        <div className="space-y-3">
          <div className="rounded-md border border-border bg-muted/40 p-3 text-sm">
            <Row label="Plan tutarı" value={formatTl(existing.data.total_amount)} strong />
            <Row label="Taksit sayısı" value={String(existing.data.num_installments)} />
            <Row label="Durum" value={existing.data.status} />
          </div>
          <ul className="space-y-1 text-sm">
            {existing.data.installments.map((i) => (
              <li
                key={i.id}
                className="flex items-center justify-between rounded-md border border-border px-3 py-2"
              >
                <div>
                  <div className="font-medium">#{i.seq} · {i.due_date}</div>
                  <div className="text-xs text-muted-foreground">
                    Ödenen {formatTl(i.paid_amount)} / {formatTl(i.amount)}
                  </div>
                </div>
                <span className={
                  "rounded px-2 py-0.5 text-xs " +
                  (i.status === "paid"
                    ? "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200"
                    : i.status === "partial"
                      ? "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200"
                      : i.status === "overdue"
                        ? "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200"
                        : "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200")
                }>
                  {i.status}
                </span>
              </li>
            ))}
          </ul>
          <SecondaryButton type="button" onClick={onClose} className="w-full">Kapat</SecondaryButton>
        </div>
      </SideSheet>
    );
  }

  const numNum = Number(num);
  const perInstallment = numNum > 0 ? invoice.balance_due / numNum : 0;

  return (
    <SideSheet open onClose={onClose} title={`Taksitlendir · ${invoice.invoice_no}`}>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
        <div className="rounded-md border border-border bg-muted/40 p-3 text-sm">
          <Row label="Plan tutarı" value={formatTl(invoice.balance_due)} strong />
          {numNum > 0 && <Row label="Taksit başına ≈" value={formatTl(perInstallment)} />}
        </div>

        <div className="grid grid-cols-3 gap-3">
          <Field id="ip-num" label="Taksit sayısı" required>
            <TextInput
              id="ip-num"
              type="number"
              min="1"
              max="60"
              required
              value={num}
              onChange={(e) => setNum(e.target.value)}
            />
          </Field>
          <Field id="ip-first" label="İlk vade" required>
            <TextInput
              id="ip-first"
              type="date"
              required
              value={firstDue}
              onChange={(e) => setFirstDue(e.target.value)}
            />
          </Field>
          <Field id="ip-interval" label="Aralık (gün)" required>
            <TextInput
              id="ip-interval"
              type="number"
              min="1"
              required
              value={interval}
              onChange={(e) => setInterval(e.target.value)}
            />
          </Field>
        </div>

        <Field id="ip-notes" label="Notlar">
          <Textarea id="ip-notes" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>

        {save.isError && <p className="text-sm text-[var(--critical)]">{(save.error as Error)?.message}</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={save.isPending || numNum <= 0}>
            <span className="inline-flex items-center gap-1">
              <Calendar className="h-4 w-4" /> {save.isPending ? "Oluşturuluyor..." : "Planı oluştur"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
