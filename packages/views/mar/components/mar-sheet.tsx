"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Pill, Plus, Syringe, Clock, AlertTriangle } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  medicationOrdersOptions,
  administrationsForOrderOptions,
  type MedicationOrder,
  type MedicationRoute,
  type AdministrationStatus,
  type MedicationAdministration,
} from "@medigt/core/mar";
import { PrimaryButton, SecondaryButton } from "../../common/form-fields";
import { OrderCreateSheet } from "./order-create-sheet";
import { GiveDoseSheet } from "./give-dose-sheet";

// Bedside MAR (Medication Administration Record) — yatan hasta için
// aktif ilaç emirlerinin listesi + her biri için son verilen dozlar.
// Hemşire "Doz ver" → 5-rights checklist drawer → kaydet.

const ROUTE_LABELS: Record<MedicationRoute, string> = {
  oral: "Oral",
  iv: "IV",
  im: "IM",
  sc: "SC",
  topical: "Topikal",
  inhalation: "İnhalasyon",
  rectal: "Rektal",
  sublingual: "Sublingual",
  intranasal: "Burun içi",
  ophthalmic: "Göz",
  otic: "Kulak",
  other: "Diğer",
};

const STATUS_LABELS: Record<AdministrationStatus, string> = {
  given: "Verildi",
  refused: "Reddetti",
  withheld: "Atlandı",
  missed: "Kaçırıldı",
  wrong_time: "Yanlış zaman",
};

const STATUS_COLORS: Record<AdministrationStatus, string> = {
  given: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  refused: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
  withheld: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  missed: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
  wrong_time: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
};

export function MARSheet({ admissionId }: { admissionId: string }) {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const orders = useQuery(medicationOrdersOptions(branchId, admissionId));
  const [createOpen, setCreateOpen] = useState(false);
  const [giveFor, setGiveFor] = useState<MedicationOrder | null>(null);

  const active = (orders.data ?? []).filter((o) => o.status === "active");
  const inactive = (orders.data ?? []).filter((o) => o.status !== "active");

  return (
    <section className="space-y-4">
      <header className="flex items-center justify-between">
        <h2 className="flex items-center gap-2 text-base font-semibold">
          <Pill className="h-5 w-5" /> İlaç Çizelgesi (MAR)
        </h2>
        <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
          <span className="inline-flex items-center gap-1">
            <Plus className="h-4 w-4" /> Yeni İlaç Emri
          </span>
        </PrimaryButton>
      </header>

      {orders.isLoading ? (
        <div className="empty-state">Yükleniyor...</div>
      ) : active.length === 0 ? (
        <div className="empty-state">Aktif ilaç emri yok.</div>
      ) : (
        <ul className="space-y-2">
          {active.map((o) => (
            <OrderRow key={o.id} order={o} onGive={() => setGiveFor(o)} />
          ))}
        </ul>
      )}

      {inactive.length > 0 && (
        <details className="rounded-md border border-border bg-card">
          <summary className="cursor-pointer p-3 text-sm font-medium">
            Pasif emirler ({inactive.length})
          </summary>
          <ul className="space-y-2 p-3 pt-0">
            {inactive.map((o) => (
              <OrderRow key={o.id} order={o} onGive={() => setGiveFor(o)} />
            ))}
          </ul>
        </details>
      )}

      {createOpen && (
        <OrderCreateSheet
          admissionId={admissionId}
          onClose={() => setCreateOpen(false)}
        />
      )}
      {giveFor && (
        <GiveDoseSheet
          order={giveFor}
          onClose={() => setGiveFor(null)}
        />
      )}
    </section>
  );
}

function OrderRow({
  order,
  onGive,
}: {
  order: MedicationOrder;
  onGive: () => void;
}) {
  const history = useQuery(administrationsForOrderOptions(order.id));
  const recent = (history.data ?? []).slice(0, 3);
  const isActive = order.status === "active";

  return (
    <li className="rounded-md border border-border bg-card p-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold">
              {order.medication_name || order.medication_id}
            </span>
            <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
              {order.order_no}
            </code>
            {!isActive && (
              <span className="rounded bg-slate-200 px-2 py-0.5 text-xs text-slate-800 dark:bg-slate-700/60 dark:text-slate-200">
                {order.status}
              </span>
            )}
            {order.is_prn && (
              <span className="inline-flex items-center gap-1 rounded bg-amber-100 px-2 py-0.5 text-xs text-amber-900 dark:bg-amber-950/40 dark:text-amber-200">
                <AlertTriangle className="h-3 w-3" /> PRN
              </span>
            )}
          </div>
          <div className="mt-1 text-sm text-muted-foreground">
            <span className="font-mono">{order.dose_amount} {order.dose_unit}</span>
            {" · "}
            <span>{ROUTE_LABELS[order.route] ?? order.route}</span>
            {" · "}
            <span>{order.frequency}</span>
            {order.scheduled_times.length > 0 && (
              <span className="ml-2 inline-flex items-center gap-1">
                <Clock className="h-3 w-3" /> {order.scheduled_times.join(" / ")}
              </span>
            )}
          </div>
          {order.instructions && (
            <p className="mt-1 text-xs text-muted-foreground">
              {order.instructions}
            </p>
          )}
          {order.prn_reason && (
            <p className="mt-1 text-xs italic text-amber-900 dark:text-amber-200">
              PRN sebep: {order.prn_reason}
            </p>
          )}
        </div>
        {isActive && (
          <SecondaryButton type="button" onClick={onGive}>
            <span className="inline-flex items-center gap-1">
              <Syringe className="h-4 w-4" /> Doz ver
            </span>
          </SecondaryButton>
        )}
      </div>

      {recent.length > 0 && (
        <ul className="mt-3 space-y-1 border-t border-border pt-2">
          {recent.map((a) => (
            <AdminRow key={a.id} admin={a} />
          ))}
          {(history.data ?? []).length > 3 && (
            <li className="text-xs text-muted-foreground">
              + {(history.data ?? []).length - 3} eski kayıt daha…
            </li>
          )}
        </ul>
      )}
    </li>
  );
}

function AdminRow({ admin }: { admin: MedicationAdministration }) {
  return (
    <li className="flex items-center justify-between text-xs">
      <div className="flex items-center gap-2">
        <span className="font-mono text-muted-foreground">
          {new Date(admin.administered_at).toLocaleString("tr-TR", {
            day: "2-digit",
            month: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
          })}
        </span>
        <span
          className={`inline-flex rounded px-2 py-0.5 ${STATUS_COLORS[admin.status]}`}
        >
          {STATUS_LABELS[admin.status]}
        </span>
        {admin.dose_amount != null && admin.dose_unit && (
          <span className="font-mono text-muted-foreground">
            {admin.dose_amount} {admin.dose_unit}
          </span>
        )}
      </div>
      {admin.five_rights_checked && admin.status === "given" && (
        <span className="text-emerald-700 dark:text-emerald-300">✓ 5 doğru</span>
      )}
    </li>
  );
}
