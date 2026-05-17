"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Download, Play } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { findReport } from "@medigt/core/rapor/registry";
import { runReportOptions } from "@medigt/core/rapor/queries";
import { formatTl } from "@medigt/core/utils";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import type { ReportColumn, ReportDescriptor, ReportResult } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  TextInput,
} from "../../common/form-fields";

export function RaporRunnerPage({ slug }: { slug: string }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const nav = useNavigation();
  const report = findReport(slug);

  if (!report) {
    return (
      <DashboardLayout>
        <div className="page-shell">
          <div className="empty-state text-[var(--critical)]">Rapor bulunamadı: {slug}</div>
        </div>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title={report.title}
          subtitle={report.description}
          actions={
            <SecondaryButton
              type="button"
              onClick={() =>
                nav.push(paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "").rapor.list())
              }
            >
              <span className="inline-flex items-center gap-1"><ArrowLeft className="h-4 w-4" /> Rapor Listesi</span>
            </SecondaryButton>
          }
        />
        <ReportShell report={report} branchId={branchId} />
      </div>
    </DashboardLayout>
  );
}

function ReportShell({ report, branchId }: { report: ReportDescriptor; branchId: string }) {
  const initial = useMemo(() => {
    const out: Record<string, string> = {};
    for (const p of report.params) {
      if (p.default != null) out[p.key] = String(p.default);
    }
    return out;
  }, [report]);

  const [draft, setDraft] = useState<Record<string, string>>(initial);
  const [applied, setApplied] = useState<Record<string, string>>(initial);

  const q = useQuery(runReportOptions(branchId, report.id, applied));

  function run() {
    setApplied({ ...draft });
  }

  function exportCsv() {
    if (!q.data) return;
    downloadCsv(report.title, q.data);
  }

  return (
    <>
      {report.params.length > 0 && (
        <div className="rounded-md border border-border bg-card p-4">
          <div className="flex flex-wrap items-end gap-3">
            {report.params.map((p) => (
              <Field key={p.key} id={`p-${p.key}`} label={p.label} required={p.required}>
                {p.type === "select" ? (
                  <SelectInput
                    id={`p-${p.key}`}
                    value={draft[p.key] ?? ""}
                    onChange={(e) => setDraft({ ...draft, [p.key]: e.target.value })}
                  >
                    {(p.options ?? []).map((o) => (
                      <option key={o.value} value={o.value}>{o.label}</option>
                    ))}
                  </SelectInput>
                ) : (
                  <TextInput
                    id={`p-${p.key}`}
                    type={p.type}
                    value={draft[p.key] ?? ""}
                    onChange={(e) => setDraft({ ...draft, [p.key]: e.target.value })}
                  />
                )}
              </Field>
            ))}
            <PrimaryButton type="button" onClick={run} disabled={q.isFetching}>
              <span className="inline-flex items-center gap-1">
                <Play className="h-4 w-4" /> {q.isFetching ? "Çalışıyor..." : "Çalıştır"}
              </span>
            </PrimaryButton>
            <SecondaryButton type="button" onClick={exportCsv} disabled={!q.data || q.data.rows.length === 0}>
              <span className="inline-flex items-center gap-1"><Download className="h-4 w-4" /> CSV</span>
            </SecondaryButton>
          </div>
        </div>
      )}

      {q.isLoading ? (
        <div className="empty-state">Yükleniyor...</div>
      ) : q.isError ? (
        <div className="empty-state text-[var(--critical)]">
          Rapor başarısız: {(q.error as Error)?.message}
        </div>
      ) : q.data ? (
        <>
          {q.data.summary && Object.keys(q.data.summary).length > 0 && (
            <SummaryStrip summary={q.data.summary} columns={q.data.columns} />
          )}
          {q.data.rows.length === 0 ? (
            <div className="empty-state">Sonuç yok.</div>
          ) : (
            <DataTable
              rows={q.data.rows.map((r, i) => ({ ...r, __idx: i }))}
              rowKey={(r) => String((r as Record<string, unknown>).__idx)}
              columns={mapColumns(q.data.columns)}
            />
          )}
        </>
      ) : null}
    </>
  );
}

function mapColumns(cols: ReportColumn[]): Column<Record<string, unknown>>[] {
  return cols.map((c) => ({
    key: c.key,
    header: c.label,
    cell: (r) => renderCell(c, r[c.key]),
    className: c.align === "right" ? "text-right" : c.align === "center" ? "text-center" : undefined,
  }));
}

function renderCell(col: ReportColumn, v: unknown): React.ReactNode {
  if (v == null || v === "") return <span className="text-muted-foreground">—</span>;
  switch (col.type) {
    case "currency": {
      const n = typeof v === "number" ? v : Number(v);
      return Number.isNaN(n) ? String(v) : <span className="font-mono">{formatTl(n)}</span>;
    }
    case "number": {
      const n = typeof v === "number" ? v : Number(v);
      return Number.isNaN(n) ? String(v) : <span className="font-mono">{n}</span>;
    }
    case "pct": {
      const n = typeof v === "number" ? v : Number(v);
      return Number.isNaN(n) ? String(v) : <span className="font-mono">%{n.toFixed(1)}</span>;
    }
    case "date":
      return <span>{toDateDisplay(v)}</span>;
    case "datetime":
      return <span>{toDateTimeDisplay(v)}</span>;
    default:
      return String(v);
  }
}

function toDateDisplay(v: unknown): string {
  if (!v) return "—";
  const d = new Date(String(v));
  return Number.isNaN(d.getTime()) ? String(v) : d.toLocaleDateString("tr-TR");
}

function toDateTimeDisplay(v: unknown): string {
  if (!v) return "—";
  const d = new Date(String(v));
  return Number.isNaN(d.getTime()) ? String(v) : d.toLocaleString("tr-TR");
}

function SummaryStrip({
  summary,
  columns,
}: {
  summary: Record<string, unknown>;
  columns: ReportColumn[];
}) {
  // Pick label from columns where possible (when key matches a column key).
  const labelByKey = new Map(columns.map((c) => [c.key, c.label]));
  const entries = Object.entries(summary);
  if (entries.length === 0) return null;
  return (
    <div className="grid grid-cols-2 gap-2 rounded-md border border-border bg-card p-3 text-sm sm:grid-cols-4">
      {entries.map(([k, v]) => (
        <div key={k} className="rounded-md border border-border bg-background p-2">
          <div className="text-xs text-muted-foreground">
            {labelByKey.get(k) ?? humanise(k)}
          </div>
          <div className="mt-1 font-mono text-sm">
            {renderSummaryValue(v)}
          </div>
        </div>
      ))}
    </div>
  );
}

function humanise(key: string): string {
  return key
    .replaceAll("_", " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

function renderSummaryValue(v: unknown): React.ReactNode {
  if (v == null) return "—";
  if (typeof v === "number") {
    // Heuristic: large numbers ≥ 1000 OR with decimals → format as currency.
    return Math.abs(v) >= 1000 || !Number.isInteger(v) ? formatTl(v) : String(v);
  }
  return String(v);
}

// ---------- CSV export ----------

function downloadCsv(title: string, result: ReportResult) {
  const lines: string[] = [];
  lines.push(result.columns.map((c) => csvEscape(c.label)).join(","));
  for (const row of result.rows) {
    lines.push(result.columns.map((c) => csvEscape(formatForCsv(c, row[c.key]))).join(","));
  }
  // BOM for Excel TR locale.
  const blob = new Blob(["﻿" + lines.join("\r\n")], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  const safe = title.replaceAll(/[^\p{L}\p{N}_-]+/gu, "_");
  a.download = `${safe}_${new Date().toISOString().slice(0, 10)}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

function csvEscape(v: string): string {
  if (v == null) return "";
  const s = String(v);
  if (s.includes(",") || s.includes("\n") || s.includes('"')) {
    return '"' + s.replaceAll('"', '""') + '"';
  }
  return s;
}

function formatForCsv(col: ReportColumn, v: unknown): string {
  if (v == null || v === "") return "";
  switch (col.type) {
    case "currency":
    case "number": {
      const n = typeof v === "number" ? v : Number(v);
      return Number.isNaN(n) ? String(v) : String(n);
    }
    case "pct": {
      const n = typeof v === "number" ? v : Number(v);
      return Number.isNaN(n) ? String(v) : n.toFixed(2);
    }
    case "date":
      return toDateDisplay(v);
    case "datetime":
      return toDateTimeDisplay(v);
    default:
      return String(v);
  }
}
