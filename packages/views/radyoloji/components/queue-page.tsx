"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ScanLine } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { radOrderListOptions } from "@medigt/core/radyoloji";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import type {
  RadiologyModality,
  RadiologyOrder,
  RadiologyOrderStatus,
} from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SelectInput } from "../../common/form-fields";

const STATUS_LABELS: Record<RadiologyOrderStatus, string> = {
  ordered: "İstendi",
  scheduled: "Planlandı",
  in_progress: "Çekimde",
  acquired: "Çekildi",
  reported: "Rapor Yazıldı",
  verified: "Onaylı",
  cancelled: "İptal",
};

const STATUS_COLORS: Record<RadiologyOrderStatus, string> = {
  ordered: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  scheduled: "bg-sky-100 text-sky-900 dark:bg-sky-950/40 dark:text-sky-200",
  in_progress: "bg-violet-100 text-violet-900 dark:bg-violet-950/40 dark:text-violet-200",
  acquired: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
  reported: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  verified: "bg-emerald-200 text-emerald-900 dark:bg-emerald-900/60 dark:text-emerald-100",
  cancelled: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};

const MODALITY_LABELS: Record<RadiologyModality, string> = {
  XR: "Röntgen",
  USG: "Ultrason",
  CT: "BT",
  MR: "MR",
  MAMMO: "Mamografi",
  NM: "Nükleer Tıp",
  DEXA: "Kemik Dansitometri",
  PET: "PET",
  ANGIO: "Anjiyo",
  FLUORO: "Floroskopi",
  OTHER: "Diğer",
};

export function RadyolojiQueuePage() {
  const branch = useHospitalStore((s) => s.branch);
  const org = useHospitalStore((s) => s.organization);
  const branchId = branch?.id ?? "";
  const [status, setStatus] = useState<RadiologyOrderStatus | "">("");
  const [modality, setModality] = useState<RadiologyModality | "">("");
  const list = useQuery(
    radOrderListOptions(branchId, {
      status: status || undefined,
      modality: modality || undefined,
    }),
  );

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Radyoloji"
          subtitle="Görüntüleme istekleri. Çekim → rapor yazımı → sorumlu onayı."
        />

        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <label className="text-sm font-medium">Durum</label>
            <SelectInput
              value={status}
              onChange={(e) => setStatus(e.target.value as RadiologyOrderStatus | "")}
              className="max-w-xs"
            >
              <option value="">Tümü</option>
              {Object.entries(STATUS_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </div>
          <div className="space-y-1">
            <label className="text-sm font-medium">Modalite</label>
            <SelectInput
              value={modality}
              onChange={(e) => setModality(e.target.value as RadiologyModality | "")}
              className="max-w-xs"
            >
              <option value="">Tümü</option>
              {Object.entries(MODALITY_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </div>
        </div>

        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Liste yüklenemedi.</div>
        ) : (list.data ?? []).length === 0 ? (
          <div className="empty-state">Hiç radyoloji isteği yok.</div>
        ) : (
          <DataTable<RadiologyOrder>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={columns(org?.slug ?? "", branch?.slug ?? "")}
          />
        )}
      </div>
    </DashboardLayout>
  );
}

function columns(orgSlug: string, branchSlug: string): Column<RadiologyOrder>[] {
  return [
    {
      key: "no",
      header: "İstek No",
      cell: (o) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{o.order_no}</code>,
    },
    {
      key: "patient",
      header: "Hasta",
      cell: (o) => (
        <div>
          <div className="font-medium">{o.patient_first_name} {o.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">MRN {o.patient_mrn}</div>
        </div>
      ),
    },
    {
      key: "procedure",
      header: "Tetkik",
      cell: (o) => (
        <div>
          <div className="font-medium">{o.procedure_name}</div>
          <div className="text-xs text-muted-foreground">
            {MODALITY_LABELS[o.modality] ?? o.modality}
            {o.body_region && ` · ${o.body_region}`}
          </div>
        </div>
      ),
    },
    {
      key: "doctor",
      header: "İsteyen",
      cell: (o) =>
        o.doctor_first_name ? (
          <span>
            {o.doctor_title ? o.doctor_title + " " : ""}
            {o.doctor_first_name} {o.doctor_last_name}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
    {
      key: "priority",
      header: "Öncelik",
      cell: (o) => (
        <span className={"text-sm " + (o.priority === "stat" ? "font-semibold text-[var(--critical)]" : "")}>
          {o.priority.toUpperCase()}
        </span>
      ),
    },
    {
      key: "ordered",
      header: "İstendi",
      cell: (o) => (
        <span className="text-xs text-muted-foreground">
          {new Date(o.ordered_at).toLocaleString("tr-TR", {
            hour: "2-digit", minute: "2-digit", day: "2-digit", month: "2-digit",
          })}
        </span>
      ),
    },
    {
      key: "status",
      header: "Durum",
      cell: (o) => (
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${STATUS_COLORS[o.status]}`}>
          {STATUS_LABELS[o.status]}
        </span>
      ),
    },
    {
      key: "open",
      header: "",
      cell: (o) => <OpenLink orderId={o.id} orgSlug={orgSlug} branchSlug={branchSlug} />,
      className: "text-right",
    },
  ];
}

function OpenLink({ orderId, orgSlug, branchSlug }: { orderId: string; orgSlug: string; branchSlug: string }) {
  const nav = useNavigation();
  return (
    <button
      type="button"
      onClick={() => nav.push(paths.hospital(orgSlug).branch(branchSlug).radyoloji.exam(orderId))}
      className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
    >
      <ScanLine className="h-3.5 w-3.5" /> Aç
    </button>
  );
}
