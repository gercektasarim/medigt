"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, History, Pill, Receipt } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { warehouseListOptions } from "@medigt/core/depo";
import {
  dispenseHistoryOptions,
  dispenseItem,
  eczaneKeys,
  eczanePendingOptions,
  fefoLotsOptions,
} from "@medigt/core/ecza";
import { depoKeys } from "@medigt/core/depo";
import type {
  DispenseHistoryRow,
  LotSummary,
  Medication,
  PendingPrescription,
  PendingPrescriptionItem,
} from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  TextInput,
} from "../../common/form-fields";
import { MedicationSearch } from "../../depo/components/medication-search";

type Tab = "queue" | "history";

export function EczanePage() {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const orgId = org?.id ?? "";
  const branchId = branch?.id ?? "";
  const [tab, setTab] = useState<Tab>("queue");
  const pending = useQuery(eczanePendingOptions(orgId));
  const history = useQuery(dispenseHistoryOptions(orgId));
  const [dispenseFor, setDispenseFor] = useState<{
    rx: PendingPrescription;
    item: PendingPrescriptionItem;
  } | null>(null);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Eczane"
          subtitle="Reçete kuyruğu, lot bazlı ilaç verme, dispense geçmişi."
        />

        <div className="flex gap-2 border-b border-border">
          <TabButton active={tab === "queue"} onClick={() => setTab("queue")}>
            <Receipt className="h-4 w-4" /> Bekleyen Reçeteler
            {pending.data && pending.data.length > 0 && (
              <span className="rounded-full bg-primary px-2 py-0.5 text-xs text-primary-foreground">
                {pending.data.length}
              </span>
            )}
          </TabButton>
          <TabButton active={tab === "history"} onClick={() => setTab("history")}>
            <History className="h-4 w-4" /> Geçmiş
          </TabButton>
        </div>

        {tab === "queue" ? (
          <Queue pending={pending.data ?? []} loading={pending.isLoading} onDispense={setDispenseFor} />
        ) : (
          <HistoryTable rows={history.data ?? []} loading={history.isLoading} />
        )}
      </div>

      {dispenseFor && (
        <DispenseSheet
          rx={dispenseFor.rx}
          item={dispenseFor.item}
          orgId={orgId}
          branchId={branchId}
          onClose={() => setDispenseFor(null)}
        />
      )}
    </DashboardLayout>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex items-center gap-1 border-b-2 px-3 py-2 text-sm font-medium ${
        active
          ? "border-primary text-foreground"
          : "border-transparent text-muted-foreground hover:text-foreground"
      }`}
    >
      {children}
    </button>
  );
}

// ---------- Queue ----------

function Queue({
  pending,
  loading,
  onDispense,
}: {
  pending: PendingPrescription[];
  loading: boolean;
  onDispense: (sel: { rx: PendingPrescription; item: PendingPrescriptionItem }) => void;
}) {
  if (loading) return <div className="empty-state">Yükleniyor...</div>;
  if (pending.length === 0) return <div className="empty-state">Bekleyen reçete yok.</div>;
  return (
    <div className="space-y-3">
      {pending.map((rx) => (
        <RxCard key={rx.id} rx={rx} onDispense={(item) => onDispense({ rx, item })} />
      ))}
    </div>
  );
}

function RxCard({
  rx,
  onDispense,
}: {
  rx: PendingPrescription;
  onDispense: (item: PendingPrescriptionItem) => void;
}) {
  return (
    <article className="rounded-lg border border-border bg-card p-4">
      <header className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold">
            {rx.patient_first_name} {rx.patient_last_name}{" "}
            <span className="text-xs font-normal text-muted-foreground">MRN {rx.patient_mrn}</span>
          </div>
          <div className="text-xs text-muted-foreground">
            Reçete <code className="rounded bg-muted px-1 py-0.5">{rx.prescription_no}</code>
            {rx.signed_at && ` · İmza ${new Date(rx.signed_at).toLocaleString("tr-TR")}`}
            {rx.doctor_first_name && (
              <>
                {" · "}
                {rx.doctor_title ? rx.doctor_title + " " : ""}
                {rx.doctor_first_name} {rx.doctor_last_name}
              </>
            )}
          </div>
        </div>
      </header>

      <ul className="mt-3 divide-y divide-border">
        {rx.items.map((it) => {
          const need = it.dispense_quantity ?? 0;
          const have = it.dispensed_total;
          const fullyDispensed = need > 0 && have >= need;
          return (
            <li key={it.item_id} className="flex items-start justify-between gap-3 py-2">
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium">{it.medication_name}</div>
                <div className="text-xs text-muted-foreground">
                  {[it.dosage, it.frequency, it.quantity].filter(Boolean).join(" · ")}
                </div>
                {it.instructions && (
                  <div className="text-xs italic text-muted-foreground">"{it.instructions}"</div>
                )}
                {need > 0 && (
                  <div className="mt-1 text-xs">
                    Verilen: <span className="font-medium">{have}</span> /{" "}
                    <span className="font-medium">{need}</span>
                  </div>
                )}
              </div>
              {fullyDispensed ? (
                <span className="inline-flex items-center gap-1 rounded-md bg-emerald-100 px-2 py-1 text-xs text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200">
                  <CheckCircle2 className="h-3.5 w-3.5" /> Tamamlandı
                </span>
              ) : (
                <button
                  type="button"
                  onClick={() => onDispense(it)}
                  className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-3 py-1.5 text-xs hover:bg-muted"
                >
                  <Pill className="h-3.5 w-3.5" /> Ver
                </button>
              )}
            </li>
          );
        })}
      </ul>
    </article>
  );
}

// ---------- History ----------

function HistoryTable({ rows, loading }: { rows: DispenseHistoryRow[]; loading: boolean }) {
  if (loading) return <div className="empty-state">Yükleniyor...</div>;
  if (rows.length === 0) return <div className="empty-state">Henüz dispense kaydı yok.</div>;
  const columns: Column<DispenseHistoryRow>[] = [
    {
      key: "at",
      header: "Tarih",
      cell: (r) => (
        <div>
          <div className="text-sm">{new Date(r.dispensed_at).toLocaleDateString("tr-TR")}</div>
          <div className="text-xs text-muted-foreground">
            {new Date(r.dispensed_at).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" })}
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
          <div className="text-xs text-muted-foreground">
            MRN {r.patient_mrn} · Reçete <code className="rounded bg-muted px-1 py-0.5">{r.prescription_no}</code>
          </div>
        </div>
      ),
    },
    {
      key: "med",
      header: "İlaç",
      cell: (r) => <span className="font-medium">{r.medication_name}</span>,
    },
    {
      key: "src",
      header: "Depo / Lot",
      cell: (r) => (
        <div className="text-xs">
          <div>{r.warehouse_name}</div>
          {r.lot_no && <code className="rounded bg-muted px-1 py-0.5">{r.lot_no}</code>}
          {r.expiry_date && <span className="ml-2 text-muted-foreground">SKT {r.expiry_date}</span>}
        </div>
      ),
    },
    {
      key: "qty",
      header: "Miktar",
      cell: (r) => <span className="font-mono text-sm font-medium">{r.quantity}</span>,
      className: "text-right",
    },
    {
      key: "mvt",
      header: "Hareket",
      cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.movement_no}</code>,
    },
    {
      key: "its",
      header: "İTS",
      cell: (r) => <ItsBadge status={r.its_status} error={r.its_error} />,
    },
  ];
  return <DataTable<DispenseHistoryRow> rows={rows} rowKey={(r) => r.id} columns={columns} />;
}

const ITS_LABEL: Record<string, string> = {
  pending: "Bekliyor",
  in_progress: "Gönderiliyor",
  notified: "Bildirildi",
  rejected: "Reddedildi",
  failed: "Başarısız",
};

const ITS_COLOR: Record<string, string> = {
  pending: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  in_progress: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  notified: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  rejected: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
  failed: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};

function ItsBadge({ status, error }: { status: string; error?: string }) {
  const label = ITS_LABEL[status] ?? status;
  const color = ITS_COLOR[status] ?? ITS_COLOR.pending;
  return (
    <span title={error ?? ""} className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${color}`}>
      {label}
    </span>
  );
}

// ---------- Dispense drawer ----------

function DispenseSheet({
  rx,
  item,
  orgId,
  branchId,
  onClose,
}: {
  rx: PendingPrescription;
  item: PendingPrescriptionItem;
  orgId: string;
  branchId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const warehouses = useQuery(warehouseListOptions(branchId));
  const [medication, setMedication] = useState<Medication | null>(null);
  const [warehouseId, setWarehouseId] = useState("");
  const [lot, setLot] = useState<LotSummary | null>(null);
  const [qty, setQty] = useState("");

  // If the item already has medication_id set, the eczane operator doesn't
  // need to pick it again — we don't have the full Medication object handy,
  // so we display the free-text name as a stub. For first-time dispense,
  // operator picks the catalog row.
  const medId = medication?.id ?? item.medication_id ?? "";

  const fefo = useQuery({
    ...fefoLotsOptions(orgId, warehouseId, medId),
    enabled: !!warehouseId && !!medId,
  });

  useEffect(() => {
    setLot(null);
  }, [warehouseId, medId]);

  // Default qty to remaining need, if known.
  useEffect(() => {
    if (item.dispense_quantity && qty === "") {
      const remaining = item.dispense_quantity - item.dispensed_total;
      if (remaining > 0) setQty(String(remaining));
    }
  }, [item.dispense_quantity, item.dispensed_total, qty]);

  const dispense = useMutation({
    mutationFn: () =>
      dispenseItem(item.item_id, {
        medication_id: medId,
        warehouse_id: warehouseId,
        lot_no: lot!.lot_no,
        expiry_date: lot!.expiry_date,
        quantity: Number(qty),
        counterparty: `${rx.patient_first_name} ${rx.patient_last_name} (MRN ${rx.patient_mrn})`,
      }),
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: eczaneKeys.all(orgId) }),
        qc.invalidateQueries({ queryKey: depoKeys.all(branchId) }),
      ]);
      onClose();
    },
  });

  const canSubmit =
    !!medId && !!warehouseId && !!lot && Number(qty) > 0 && Number(qty) <= (lot?.quantity ?? 0) && !dispense.isPending;

  return (
    <SideSheet open onClose={onClose} title="İlaç Ver">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); dispense.mutate(); }}>
        <div className="rounded-md border border-border bg-muted/40 p-3 text-sm">
          <div className="font-medium">{item.medication_name}</div>
          <div className="text-xs text-muted-foreground">
            {[item.dosage, item.frequency, item.quantity].filter(Boolean).join(" · ")}
          </div>
          <div className="mt-1 text-xs text-muted-foreground">
            {rx.patient_first_name} {rx.patient_last_name} · MRN {rx.patient_mrn}
          </div>
        </div>

        {!item.medication_id && (
          <Field id="d-med" label="Katalogdan ilaç seç" required hint="Doktorun yazdığı serbest metnin ilaç kataloğundaki karşılığı.">
            {medication ? (
              <div className="flex items-center justify-between rounded-md border border-border bg-muted/40 px-3 py-2">
                <div>
                  <div className="font-medium">{medication.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {medication.strength ?? ""}
                    {medication.atc_code && ` · ${medication.atc_code}`}
                  </div>
                </div>
                <button type="button" onClick={() => setMedication(null)} className="text-xs text-muted-foreground hover:underline">
                  Değiştir
                </button>
              </div>
            ) : (
              <MedicationSearch onPick={setMedication} />
            )}
          </Field>
        )}

        <Field id="d-wh" label="Depo" required>
          <SelectInput
            id="d-wh"
            required
            value={warehouseId}
            onChange={(e) => setWarehouseId(e.target.value)}
          >
            <option value="">— Seçiniz —</option>
            {(warehouses.data ?? []).map((w) => (
              <option key={w.id} value={w.id}>{w.name}</option>
            ))}
          </SelectInput>
        </Field>

        {warehouseId && medId && (
          <Field id="d-lot" label="Lot (FEFO)" required hint="Erken son kullanma tarihli lot en üstte.">
            {fefo.isLoading ? (
              <p className="text-sm text-muted-foreground">Yükleniyor...</p>
            ) : (fefo.data ?? []).length === 0 ? (
              <p className="text-sm text-[var(--critical)]">Bu depoda bu ilaç için stok yok.</p>
            ) : (
              <ul className="space-y-1">
                {(fefo.data ?? []).map((l) => (
                  <li key={l.stock_id}>
                    <button
                      type="button"
                      onClick={() => setLot(l)}
                      className={`flex w-full items-center justify-between rounded-md border px-3 py-2 text-left text-sm ${
                        lot?.stock_id === l.stock_id
                          ? "border-primary bg-primary/10"
                          : "border-border bg-card hover:bg-muted"
                      }`}
                    >
                      <div>
                        <div className="font-medium">
                          {l.lot_no || <span className="text-muted-foreground">— lotsuz</span>}
                        </div>
                        {l.expiry_date && (
                          <div className="text-xs text-muted-foreground">SKT {l.expiry_date}</div>
                        )}
                      </div>
                      <span className="font-mono text-sm">{l.quantity}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </Field>
        )}

        <Field id="d-qty" label="Verilecek miktar" required>
          <TextInput
            id="d-qty"
            type="number"
            min="0"
            step="0.001"
            required
            value={qty}
            onChange={(e) => setQty(e.target.value)}
          />
        </Field>
        {lot && Number(qty) > lot.quantity && (
          <p className="text-sm text-[var(--critical)]">Bu lottaki miktarı aşıyor (maks. {lot.quantity}).</p>
        )}

        {dispense.isError && (
          <p className="text-sm text-[var(--critical)]">Dispense başarısız.</p>
        )}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            <span className="inline-flex items-center gap-1">
              <Pill className="h-4 w-4" /> {dispense.isPending ? "Veriliyor..." : "Hastaya ver"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
