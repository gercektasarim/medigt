"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { FileText, Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { invoiceListOptions } from "@medigt/core/fatura";
import { formatTl } from "@medigt/core/utils";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import type { Invoice, InvoiceStatus } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import {
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  TextInput,
} from "../../common/form-fields";
import { STATUS_COLORS, STATUS_LABELS } from "./labels";
import { CreateInvoiceSheet } from "./create-invoice-sheet";

function todayISO(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

export function FaturaListPage() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const [status, setStatus] = useState<InvoiceStatus | "">("");
  const [unpaid, setUnpaid] = useState(false);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [createOpen, setCreateOpen] = useState(false);

  const filter = useMemo(
    () => ({
      status: status || undefined,
      onlyUnpaid: unpaid || undefined,
      from: from || undefined,
      to: to || undefined,
    }),
    [status, unpaid, from, to],
  );
  const list = useQuery(invoiceListOptions(branchId, filter));

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Faturalar"
          subtitle="Hasta faturaları, kalemler, tahsilat takibi. Yeni fatura oluşturun veya açık olanlara ödeme alın."
          actions={
            <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Fatura</span>
            </PrimaryButton>
          }
        />

        <div className="flex flex-wrap items-end gap-3">
          <SelectInput
            value={status}
            onChange={(e) => setStatus(e.target.value as InvoiceStatus | "")}
            className="max-w-xs"
          >
            <option value="">Tüm durumlar</option>
            {Object.entries(STATUS_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
          <TextInput type="date" value={from} onChange={(e) => setFrom(e.target.value)} className="max-w-xs" />
          <TextInput type="date" value={to} onChange={(e) => setTo(e.target.value)} className="max-w-xs" />
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={unpaid}
              onChange={(e) => setUnpaid(e.target.checked)}
              className="h-4 w-4 rounded border-input"
            />
            Sadece ödenmemiş
          </label>
          <SecondaryButton type="button" onClick={() => { setFrom(""); setTo(""); setStatus(""); setUnpaid(false); }}>
            Temizle
          </SecondaryButton>
          <SecondaryButton type="button" onClick={() => { const t = todayISO(); setFrom(t); setTo(t); }}>
            Bugün
          </SecondaryButton>
        </div>

        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : (list.data ?? []).length === 0 ? (
          <div className="empty-state">Fatura bulunamadı.</div>
        ) : (
          <DataTable<Invoice>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={columns()}
          />
        )}
      </div>

      <CreateInvoiceSheet open={createOpen} onClose={() => setCreateOpen(false)} branchId={branchId} />
    </DashboardLayout>
  );
}

function columns(): Column<Invoice>[] {
  return [
    {
      key: "no",
      header: "Fatura",
      cell: (r) => (
        <div>
          <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.invoice_no}</code>
          <div className="mt-1 text-xs text-muted-foreground">
            {new Date(r.created_at).toLocaleDateString("tr-TR")}
          </div>
        </div>
      ),
    },
    {
      key: "patient",
      header: "Hasta",
      cell: (r) => (
        <div>
          <div className="font-medium">{r.patient_first_name} {r.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">MRN {r.patient_mrn}</div>
        </div>
      ),
    },
    {
      key: "institution",
      header: "Kurum",
      cell: (r) => r.institution_name ?? <span className="text-xs text-muted-foreground">Cepten ödeme</span>,
    },
    {
      key: "status",
      header: "Durum",
      cell: (r) => (
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${STATUS_COLORS[r.status]}`}>
          {STATUS_LABELS[r.status]}
        </span>
      ),
    },
    {
      key: "total",
      header: "Tutar",
      cell: (r) => (
        <div className="font-mono text-sm">
          <div>{formatTl(r.total)}</div>
          {r.balance_due > 0 && r.status !== "cancelled" && (
            <div className="text-xs text-[var(--critical)]">Kalan: {formatTl(r.balance_due)}</div>
          )}
        </div>
      ),
      className: "text-right",
    },
    {
      key: "open",
      header: "",
      cell: (r) => <OpenLink id={r.id} />,
      className: "text-right",
    },
  ];
}

function OpenLink({ id }: { id: string }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const nav = useNavigation();
  return (
    <button
      type="button"
      onClick={() =>
        nav.push(paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "").fatura.detail(id))
      }
      className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
    >
      <FileText className="h-3.5 w-3.5" /> Aç
    </button>
  );
}
