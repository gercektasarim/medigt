"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Boxes, Edit3, Package, PackagePlus, Plus, Warehouse as WarehouseIcon } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  adjustStock,
  createWarehouse,
  depoKeys,
  movementListOptions,
  receiveStock,
  stockListOptions,
  warehouseListOptions,
  type CreateWarehouseInput,
} from "@medigt/core/depo";
import { formatTl } from "@medigt/core/utils";
import type {
  Medication,
  StockMovement,
  StockMovementKind,
  StockRow,
  WarehouseKind,
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
import { MedicationSearch } from "./medication-search";

const KIND_LABELS: Record<WarehouseKind, string> = {
  pharmacy: "Eczane Deposu",
  general: "Genel Depo",
  central: "Merkez Depo",
  ward: "Servis Dolabı",
  operating_room: "Ameliyathane",
  other: "Diğer",
};

const MOVEMENT_LABELS: Record<StockMovementKind, string> = {
  receive: "Giriş",
  issue: "Çıkış",
  transfer_out: "Transfer (Çıkış)",
  transfer_in: "Transfer (Giriş)",
  adjust: "Düzeltme",
  expire: "SKT Geçti",
  return: "İade",
};

const MOVEMENT_COLORS: Record<StockMovementKind, string> = {
  receive: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  issue: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
  transfer_out: "bg-orange-100 text-orange-900 dark:bg-orange-950/40 dark:text-orange-200",
  transfer_in: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  adjust: "bg-violet-100 text-violet-900 dark:bg-violet-950/40 dark:text-violet-200",
  expire: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  return: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
};

type Tab = "stock" | "movements";

export function DepoPage() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const warehouses = useQuery(warehouseListOptions(branchId));

  const [tab, setTab] = useState<Tab>("stock");
  const [warehouseId, setWarehouseId] = useState("");
  const [search, setSearch] = useState("");
  const [expiring, setExpiring] = useState(0);

  const [newWarehouseOpen, setNewWarehouseOpen] = useState(false);
  const [receiveOpen, setReceiveOpen] = useState(false);
  const [adjustFor, setAdjustFor] = useState<StockRow | null>(null);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Depo"
          subtitle="İlaç stoğu, lot/SKT takibi, mal giriş ve düzeltmeleri. Hareketler tam denetlenebilir."
          actions={
            <div className="flex gap-2">
              <SecondaryButton type="button" onClick={() => setNewWarehouseOpen(true)}>
                <span className="inline-flex items-center gap-1"><WarehouseIcon className="h-4 w-4" /> Depolar</span>
              </SecondaryButton>
              <PrimaryButton type="button" onClick={() => setReceiveOpen(true)} disabled={(warehouses.data ?? []).length === 0}>
                <span className="inline-flex items-center gap-1"><PackagePlus className="h-4 w-4" /> Mal Girişi</span>
              </PrimaryButton>
            </div>
          }
        />

        <div className="flex flex-wrap items-end gap-3">
          <Field id="dep-wh" label="Depo">
            <SelectInput
              id="dep-wh"
              value={warehouseId}
              onChange={(e) => setWarehouseId(e.target.value)}
              className="max-w-xs"
            >
              <option value="">Tüm depolar</option>
              {(warehouses.data ?? []).map((w) => (
                <option key={w.id} value={w.id}>{w.name} ({KIND_LABELS[w.kind] ?? w.kind})</option>
              ))}
            </SelectInput>
          </Field>
          {tab === "stock" && (
            <>
              <Field id="dep-q" label="Ara">
                <TextInput
                  id="dep-q"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="İlaç adı, ATC, barkod"
                  className="max-w-xs"
                />
              </Field>
              <Field id="dep-exp" label="SKT (gün)">
                <SelectInput
                  id="dep-exp"
                  value={String(expiring)}
                  onChange={(e) => setExpiring(Number(e.target.value))}
                  className="max-w-[160px]"
                >
                  <option value="0">Hepsi</option>
                  <option value="30">30 gün içi</option>
                  <option value="90">90 gün içi</option>
                  <option value="180">180 gün içi</option>
                </SelectInput>
              </Field>
            </>
          )}
        </div>

        <div className="flex gap-2 border-b border-border">
          <TabButton active={tab === "stock"} onClick={() => setTab("stock")}>
            <Boxes className="h-4 w-4" /> Stok
          </TabButton>
          <TabButton active={tab === "movements"} onClick={() => setTab("movements")}>
            <Package className="h-4 w-4" /> Hareketler
          </TabButton>
        </div>

        {(warehouses.data ?? []).length === 0 ? (
          <div className="empty-state">
            Henüz depo tanımlanmamış. Önce <strong>Depolar</strong> butonundan en az bir depo ekleyin.
          </div>
        ) : tab === "stock" ? (
          <StockTab branchId={branchId} warehouseId={warehouseId} search={search} expiring={expiring} onAdjust={setAdjustFor} />
        ) : (
          <MovementsTab branchId={branchId} warehouseId={warehouseId} />
        )}
      </div>

      <NewWarehouseSheet
        open={newWarehouseOpen}
        onClose={() => setNewWarehouseOpen(false)}
        branchId={branchId}
      />
      <ReceiveSheet
        open={receiveOpen}
        onClose={() => setReceiveOpen(false)}
        branchId={branchId}
        defaultWarehouseId={warehouseId}
      />
      {adjustFor && (
        <AdjustSheet
          row={adjustFor}
          branchId={branchId}
          onClose={() => setAdjustFor(null)}
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

// ---------- Stock tab ----------

function StockTab({
  branchId,
  warehouseId,
  search,
  expiring,
  onAdjust,
}: {
  branchId: string;
  warehouseId: string;
  search: string;
  expiring: number;
  onAdjust: (row: StockRow) => void;
}) {
  const filter = useMemo(
    () => ({
      warehouseId: warehouseId || undefined,
      q: search || undefined,
      expiringDays: expiring || undefined,
    }),
    [warehouseId, search, expiring],
  );
  const list = useQuery(stockListOptions(branchId, filter));

  if (list.isLoading) return <div className="empty-state">Yükleniyor...</div>;
  if ((list.data ?? []).length === 0) return <div className="empty-state">Bu filtre için stok yok.</div>;

  const columns: Column<StockRow>[] = [
    {
      key: "med",
      header: "İlaç",
      cell: (r) => (
        <div>
          <div className="font-medium">{r.medication_name}</div>
          {r.generic_name && <div className="text-xs text-muted-foreground">{r.generic_name}</div>}
        </div>
      ),
    },
    { key: "wh", header: "Depo", cell: (r) => <span className="text-sm">{r.warehouse_name}</span> },
    {
      key: "lot",
      header: "Lot / SKT",
      cell: (r) => (
        <div className="text-xs">
          {r.lot_no ? <code className="rounded bg-muted px-1 py-0.5">{r.lot_no}</code> : <span className="text-muted-foreground">—</span>}
          {r.expiry_date && <span className={"ml-2 " + expiryColor(r.expiry_date)}>{r.expiry_date}</span>}
        </div>
      ),
    },
    {
      key: "qty",
      header: "Miktar",
      cell: (r) => <span className="font-mono text-sm font-medium">{formatQty(r.quantity)}</span>,
      className: "text-right",
    },
    {
      key: "act",
      header: "",
      cell: (r) => (
        <button
          type="button"
          onClick={() => onAdjust(r)}
          className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
        >
          <Edit3 className="h-3.5 w-3.5" /> Düzelt
        </button>
      ),
      className: "text-right",
    },
  ];

  return (
    <DataTable<StockRow>
      rows={list.data ?? []}
      rowKey={(r) => r.stock_id}
      columns={columns}
    />
  );
}

function formatQty(q: number): string {
  if (Number.isInteger(q)) return String(q);
  return q.toFixed(3).replace(/0+$/, "").replace(/\.$/, "");
}

function expiryColor(date: string): string {
  const today = new Date();
  const d = new Date(date);
  const days = Math.round((d.getTime() - today.getTime()) / (1000 * 60 * 60 * 24));
  if (days < 0) return "font-semibold text-[var(--critical)]";
  if (days < 30) return "font-semibold text-orange-700 dark:text-orange-300";
  if (days < 90) return "text-amber-700 dark:text-amber-300";
  return "text-muted-foreground";
}

// ---------- Movements tab ----------

function MovementsTab({ branchId, warehouseId }: { branchId: string; warehouseId: string }) {
  const list = useQuery(
    movementListOptions(branchId, { warehouseId: warehouseId || undefined }),
  );

  if (list.isLoading) return <div className="empty-state">Yükleniyor...</div>;
  if ((list.data ?? []).length === 0) return <div className="empty-state">Henüz hareket yok.</div>;

  const columns: Column<StockMovement>[] = [
    {
      key: "at",
      header: "Tarih",
      cell: (r) => (
        <div>
          <div className="text-sm">{new Date(r.performed_at).toLocaleDateString("tr-TR")}</div>
          <div className="text-xs text-muted-foreground">{new Date(r.performed_at).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" })}</div>
        </div>
      ),
    },
    { key: "no", header: "No", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.movement_no}</code> },
    {
      key: "kind",
      header: "Tür",
      cell: (r) => (
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${MOVEMENT_COLORS[r.kind]}`}>
          {MOVEMENT_LABELS[r.kind]}
        </span>
      ),
    },
    {
      key: "med",
      header: "İlaç",
      cell: (r) => (
        <div>
          <div className="font-medium">{r.medication_name}</div>
          <div className="text-xs text-muted-foreground">{r.warehouse_name}</div>
        </div>
      ),
    },
    {
      key: "lot",
      header: "Lot / SKT",
      cell: (r) => (
        <div className="text-xs">
          {r.lot_no ? <code className="rounded bg-muted px-1 py-0.5">{r.lot_no}</code> : <span className="text-muted-foreground">—</span>}
          {r.expiry_date && <span className="ml-2 text-muted-foreground">{r.expiry_date}</span>}
        </div>
      ),
    },
    {
      key: "qty",
      header: "Miktar",
      cell: (r) => (
        <div className="font-mono text-sm">
          {formatQty(r.quantity)}
          {r.unit_price != null && (
            <div className="text-xs text-muted-foreground">{formatTl(r.unit_price)} /birim</div>
          )}
        </div>
      ),
      className: "text-right",
    },
    {
      key: "ctp",
      header: "Karşı taraf / Not",
      cell: (r) => (
        <div className="text-xs text-muted-foreground">
          {r.counterparty ?? ""}
          {r.notes && (r.counterparty ? " · " : "") + r.notes}
        </div>
      ),
    },
  ];

  return (
    <DataTable<StockMovement>
      rows={list.data ?? []}
      rowKey={(r) => r.id}
      columns={columns}
    />
  );
}

// ---------- Drawers ----------

function NewWarehouseSheet({
  open,
  onClose,
  branchId,
}: {
  open: boolean;
  onClose: () => void;
  branchId: string;
}) {
  const qc = useQueryClient();
  const [form, setForm] = useState<CreateWarehouseInput>({
    code: "",
    name: "",
    kind: "pharmacy",
  });
  const create = useMutation({
    mutationFn: () => createWarehouse(form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: depoKeys.warehouses(branchId) });
      setForm({ code: "", name: "", kind: "pharmacy" });
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Depo">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="w-name" label="Ad" required>
          <TextInput id="w-name" required value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder="Eczane Ana Deposu" />
        </Field>
        <Field id="w-code" label="Kod" required>
          <TextInput id="w-code" required value={form.code}
            onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase().replace(/[^A-Z0-9_-]/g, "") })}
            placeholder="ECZ-1" />
        </Field>
        <Field id="w-kind" label="Tür">
          <SelectInput id="w-kind" value={form.kind ?? "pharmacy"}
            onChange={(e) => setForm({ ...form, kind: e.target.value as WarehouseKind })}
          >
            {Object.entries(KIND_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <Field id="w-loc" label="Konum">
          <TextInput id="w-loc" value={form.location ?? ""}
            onChange={(e) => setForm({ ...form, location: e.target.value })} />
        </Field>
        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız. Kod zaten kayıtlı olabilir.</p>}
        {create.isSuccess && <p className="text-sm text-emerald-700">Eklendi.</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">Kapat</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !form.name || !form.code}>
            <span className="inline-flex items-center gap-1">
              <Plus className="h-4 w-4" /> {create.isPending ? "Ekleniyor..." : "Ekle"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function ReceiveSheet({
  open,
  onClose,
  branchId,
  defaultWarehouseId,
}: {
  open: boolean;
  onClose: () => void;
  branchId: string;
  defaultWarehouseId: string;
}) {
  const qc = useQueryClient();
  const warehouses = useQuery(warehouseListOptions(branchId));
  const [medication, setMedication] = useState<Medication | null>(null);
  const [warehouseId, setWarehouseId] = useState(defaultWarehouseId);
  const [lotNo, setLotNo] = useState("");
  const [expiry, setExpiry] = useState("");
  const [quantity, setQuantity] = useState("");
  const [unitPrice, setUnitPrice] = useState("");
  const [counterparty, setCounterparty] = useState("");
  const [notes, setNotes] = useState("");

  const create = useMutation({
    mutationFn: () =>
      receiveStock({
        warehouse_id: warehouseId,
        medication_id: medication!.id,
        lot_no: lotNo.trim(),
        expiry_date: expiry || undefined,
        quantity: Number(quantity),
        unit_price: unitPrice ? Number(unitPrice) : undefined,
        counterparty: counterparty.trim() || undefined,
        notes: notes.trim() || undefined,
      }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: depoKeys.all(branchId) });
      setMedication(null);
      setLotNo("");
      setExpiry("");
      setQuantity("");
      setUnitPrice("");
      setCounterparty("");
      setNotes("");
      onClose();
    },
  });

  const canSubmit = medication && warehouseId && Number(quantity) > 0 && !create.isPending;

  return (
    <SideSheet open={open} onClose={onClose} title="Mal Girişi">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="r-med" label="İlaç" required>
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

        <Field id="r-wh" label="Depo" required>
          <SelectInput
            id="r-wh"
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

        <div className="grid grid-cols-2 gap-3">
          <Field id="r-lot" label="Lot No">
            <TextInput id="r-lot" value={lotNo} onChange={(e) => setLotNo(e.target.value)} placeholder="ör. L240312" />
          </Field>
          <Field id="r-exp" label="SKT">
            <TextInput id="r-exp" type="date" value={expiry} onChange={(e) => setExpiry(e.target.value)} />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field id="r-qty" label="Miktar" required>
            <TextInput
              id="r-qty"
              type="number"
              min="0"
              step="0.001"
              required
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
            />
          </Field>
          <Field id="r-price" label="Birim alış fiyatı (TRY)">
            <TextInput
              id="r-price"
              type="number"
              min="0"
              step="0.01"
              value={unitPrice}
              onChange={(e) => setUnitPrice(e.target.value)}
            />
          </Field>
        </div>

        <Field id="r-cp" label="Tedarikçi / Karşı taraf">
          <TextInput id="r-cp" value={counterparty} onChange={(e) => setCounterparty(e.target.value)} />
        </Field>

        <Field id="r-notes" label="Notlar">
          <Textarea id="r-notes" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>

        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız.</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            <span className="inline-flex items-center gap-1">
              <PackagePlus className="h-4 w-4" /> {create.isPending ? "Kaydediliyor..." : "Girişi kaydet"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function AdjustSheet({
  row,
  branchId,
  onClose,
}: {
  row: StockRow;
  branchId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [newQty, setNewQty] = useState(row.quantity.toString());
  const [notes, setNotes] = useState("");

  const save = useMutation({
    mutationFn: () =>
      adjustStock({
        warehouse_id: row.warehouse_id,
        medication_id: row.medication_id,
        lot_no: row.lot_no,
        expiry_date: row.expiry_date,
        new_quantity: Number(newQty),
        notes: notes.trim() || undefined,
      }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: depoKeys.all(branchId) });
      onClose();
    },
  });

  const delta = Number(newQty) - row.quantity;

  return (
    <SideSheet open onClose={onClose} title="Stok Düzeltme">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
        <div className="rounded-md border border-border bg-muted/40 p-3">
          <div className="font-medium">{row.medication_name}</div>
          <div className="text-xs text-muted-foreground">
            {row.warehouse_name}
            {row.lot_no && ` · Lot ${row.lot_no}`}
            {row.expiry_date && ` · SKT ${row.expiry_date}`}
          </div>
          <div className="mt-2 text-sm">
            Mevcut: <span className="font-mono font-medium">{formatQty(row.quantity)}</span>
          </div>
        </div>

        <Field id="a-qty" label="Yeni miktar" required>
          <TextInput
            id="a-qty"
            type="number"
            min="0"
            step="0.001"
            required
            value={newQty}
            onChange={(e) => setNewQty(e.target.value)}
          />
        </Field>
        {!Number.isNaN(delta) && delta !== 0 && (
          <p className={`text-sm ${delta > 0 ? "text-emerald-700" : "text-rose-700"}`}>
            Delta: {delta > 0 ? "+" : ""}{formatQty(delta)}
          </p>
        )}

        <Field id="a-notes" label="Düzeltme nedeni" required>
          <Textarea
            id="a-notes"
            rows={3}
            required
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Sayım farkı, hatalı giriş, vb."
          />
        </Field>

        {save.isError && (
          <p className="text-sm text-[var(--critical)]">Kayıt başarısız.</p>
        )}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={save.isPending || !notes.trim() || delta === 0}>
            {save.isPending ? "Kaydediliyor..." : "Kaydet"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
