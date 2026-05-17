"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Banknote,
  FileText,
  LockKeyhole,
  Plus,
  Receipt,
  Undo2,
  Wallet,
} from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { paths } from "@medigt/core/paths";
import { AppLink } from "@medigt/core/navigation";
import {
  closeRegister,
  myRegisterOptions,
  openRegister,
  recordMovement,
  registerListOptions,
  registerMovementsOptions,
  vezneKeys,
  zReportOptions,
  type RecordMovementInput,
} from "@medigt/core/vezne";
import { formatTl } from "@medigt/core/utils";
import type {
  CashMovement,
  CashMovementKind,
  CashRegister,
  PaymentMethod,
  ZReport,
} from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import { AdvanceSheet } from "./advance-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";

const KIND_LABELS: Record<CashMovementKind, string> = {
  opening: "Açılış",
  income: "Tahsilat",
  expense: "Gider",
  refund: "İade",
  closing: "Kapanış",
  transfer_in: "Transfer Giriş",
  transfer_out: "Transfer Çıkış",
};

const KIND_COLORS: Record<CashMovementKind, string> = {
  opening: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  income: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  expense: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
  refund: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
  closing: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  transfer_in: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  transfer_out: "bg-orange-100 text-orange-900 dark:bg-orange-950/40 dark:text-orange-200",
};

const METHOD_LABELS: Record<PaymentMethod, string> = {
  cash: "Nakit",
  card: "Kart",
  transfer: "Havale",
  mobile: "Mobil",
  other: "Diğer",
};

export function VeznePage() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const my = useQuery(myRegisterOptions(branchId));
  const [openOpen, setOpenOpen] = useState(false);

  if (my.isLoading) {
    return (
      <DashboardLayout>
        <div className="page-shell">Yükleniyor...</div>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Vezne"
          subtitle="Kasiyer kasası: aç → tahsilat / gider / iade kaydet → Z raporu ile kapat."
          actions={
            <div className="flex items-center gap-2">
              <AppLink
                to={paths.hospital(useHospitalStore.getState().organization?.slug ?? "")
                  .branch(useHospitalStore.getState().branch?.slug ?? "")
                  .vezne.fiyatGuncelleme()}
                className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted"
              >
                <Receipt className="h-4 w-4" /> Fiyat güncelle
              </AppLink>
              {!my.data && (
                <PrimaryButton type="button" onClick={() => setOpenOpen(true)}>
                  <span className="inline-flex items-center gap-1">
                    <Plus className="h-4 w-4" /> Kasa Aç
                  </span>
                </PrimaryButton>
              )}
            </div>
          }
        />

        {my.data ? (
          <ActiveRegister branchId={branchId} register={my.data} />
        ) : (
          <SessionsList branchId={branchId} />
        )}
      </div>

      <OpenRegisterSheet
        open={openOpen}
        onClose={() => setOpenOpen(false)}
        branchId={branchId}
      />
    </DashboardLayout>
  );
}

// ---------- Active register pane ----------

function ActiveRegister({ branchId, register }: { branchId: string; register: CashRegister }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const z = useQuery(zReportOptions(branchId, register.id));
  const [moveOpen, setMoveOpen] = useState<CashMovementKind | null>(null);
  const [closeOpen, setCloseOpen] = useState(false);
  const [advanceOpen, setAdvanceOpen] = useState(false);
  const zRaporHref = paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "").vezne.zRapor(register.id);

  const summary = z.data;
  const expected = summary?.expected_close ?? register.opening_balance;
  const cashIncome = summary?.total_income ?? 0;
  const cashExpense = summary?.total_expense ?? 0;
  const cashRefund = summary?.total_refund ?? 0;

  return (
    <>
      <section className="rounded-lg border border-border bg-card p-4">
        <header className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="text-sm font-semibold">
              Açık Kasa · <code className="rounded bg-muted px-1.5 py-0.5">{register.register_no}</code>
            </div>
            <div className="text-xs text-muted-foreground">
              {register.cashier_name} · Açılış {new Date(register.opened_at).toLocaleString("tr-TR")}
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <SecondaryButton type="button" onClick={() => setMoveOpen("income")}>
              <span className="inline-flex items-center gap-1"><ArrowDownToLine className="h-4 w-4" /> Tahsilat</span>
            </SecondaryButton>
            <SecondaryButton type="button" onClick={() => setMoveOpen("expense")}>
              <span className="inline-flex items-center gap-1"><ArrowUpFromLine className="h-4 w-4" /> Gider</span>
            </SecondaryButton>
            <SecondaryButton type="button" onClick={() => setMoveOpen("refund")}>
              <span className="inline-flex items-center gap-1"><Undo2 className="h-4 w-4" /> İade</span>
            </SecondaryButton>
            <SecondaryButton type="button" onClick={() => setAdvanceOpen(true)}>
              <span className="inline-flex items-center gap-1"><Wallet className="h-4 w-4" /> Avans</span>
            </SecondaryButton>
            <AppLink
              to={zRaporHref}
              className="inline-flex items-center justify-center gap-1 rounded-md border border-input bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted"
            >
              <FileText className="h-4 w-4" /> Z Raporu
            </AppLink>
            <PrimaryButton type="button" onClick={() => setCloseOpen(true)}>
              <span className="inline-flex items-center gap-1"><LockKeyhole className="h-4 w-4" /> Kasayı Kapat</span>
            </PrimaryButton>
          </div>
        </header>

        <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat label="Açılış" value={formatTl(register.opening_balance)} />
          <Stat label="Tahsilat (nakit)" value={formatTl(cashIncome)} tone="positive" />
          <Stat label="Gider / İade (nakit)" value={formatTl(cashExpense + cashRefund)} tone="negative" />
          <Stat label="Beklenen Kasa" value={formatTl(expected)} tone="strong" />
        </div>
      </section>

      <ZByMethod z={summary} />

      <section>
        <h2 className="mb-2 flex items-center gap-2 text-sm font-semibold">
          <Receipt className="h-4 w-4" /> Hareketler
        </h2>
        <Movements branchId={branchId} registerId={register.id} />
      </section>

      {moveOpen && (
        <MovementSheet
          kind={moveOpen}
          branchId={branchId}
          registerId={register.id}
          onClose={() => setMoveOpen(null)}
        />
      )}
      {closeOpen && (
        <CloseRegisterSheet
          branchId={branchId}
          register={register}
          expected={expected}
          onClose={() => setCloseOpen(false)}
        />
      )}
      {advanceOpen && (
        <AdvanceSheet
          branchId={branchId}
          registerId={register.id}
          onClose={() => setAdvanceOpen(false)}
        />
      )}
    </>
  );
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: "positive" | "negative" | "strong" }) {
  const cls =
    tone === "positive"
      ? "text-emerald-700 dark:text-emerald-300"
      : tone === "negative"
        ? "text-rose-700 dark:text-rose-300"
        : tone === "strong"
          ? "text-foreground font-semibold"
          : "text-foreground";
  return (
    <div className="rounded-md border border-border bg-background p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={`mt-1 font-mono text-lg ${cls}`}>{value}</div>
    </div>
  );
}

function ZByMethod({ z }: { z: ZReport | undefined }) {
  if (!z || z.by_kind_method.length === 0) return null;
  const grouped = new Map<PaymentMethod, { income: number; expense: number; refund: number }>();
  for (const r of z.by_kind_method) {
    const slot = grouped.get(r.method) ?? { income: 0, expense: 0, refund: 0 };
    if (r.kind === "income") slot.income += r.total;
    if (r.kind === "expense") slot.expense += r.total;
    if (r.kind === "refund") slot.refund += r.total;
    grouped.set(r.method, slot);
  }
  return (
    <section>
      <h2 className="mb-2 flex items-center gap-2 text-sm font-semibold">
        <Banknote className="h-4 w-4" /> Ödeme Yöntemine Göre
      </h2>
      <div className="grid grid-cols-2 gap-2 rounded-md border border-border bg-card p-3 text-sm sm:grid-cols-5">
        {(["cash", "card", "transfer", "mobile", "other"] as PaymentMethod[]).map((m) => {
          const slot = grouped.get(m);
          const net = (slot?.income ?? 0) - (slot?.expense ?? 0) - (slot?.refund ?? 0);
          return (
            <div key={m} className="rounded-md border border-border bg-background p-2">
              <div className="text-xs text-muted-foreground">{METHOD_LABELS[m]}</div>
              <div className="mt-1 font-mono text-sm">{formatTl(net)}</div>
              {slot && (
                <div className="mt-1 text-[10px] text-muted-foreground">
                  +{formatTl(slot.income)} / -{formatTl(slot.expense + slot.refund)}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function Movements({ branchId, registerId }: { branchId: string; registerId: string }) {
  const list = useQuery(registerMovementsOptions(branchId, registerId));
  if (list.isLoading) return <div className="empty-state">Yükleniyor...</div>;
  if ((list.data ?? []).length === 0)
    return <div className="empty-state">Henüz hareket yok.</div>;

  const columns: Column<CashMovement>[] = [
    {
      key: "at",
      header: "Saat",
      cell: (r) => (
        <span className="font-mono text-xs">
          {new Date(r.performed_at).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" })}
        </span>
      ),
    },
    { key: "no", header: "No", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.movement_no}</code> },
    {
      key: "kind",
      header: "Tür",
      cell: (r) => (
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${KIND_COLORS[r.kind]}`}>
          {KIND_LABELS[r.kind]}
        </span>
      ),
    },
    { key: "method", header: "Yöntem", cell: (r) => METHOD_LABELS[r.method] ?? r.method },
    {
      key: "amount",
      header: "Tutar",
      cell: (r) => (
        <span className={"font-mono font-medium " + (r.kind === "expense" || r.kind === "refund" || r.kind === "transfer_out" ? "text-rose-700 dark:text-rose-300" : "")}>
          {formatTl(r.amount)}
        </span>
      ),
      className: "text-right",
    },
    {
      key: "cp",
      header: "Karşı taraf / Açıklama",
      cell: (r) => (
        <div className="text-xs text-muted-foreground">
          {r.counterparty ?? ""}
          {r.description && (r.counterparty ? " · " : "") + r.description}
        </div>
      ),
    },
  ];

  return <DataTable<CashMovement> rows={list.data ?? []} rowKey={(r) => r.id} columns={columns} />;
}

// ---------- Past sessions list (when no open register) ----------

function SessionsList({ branchId }: { branchId: string }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const list = useQuery(registerListOptions(branchId));
  if (list.isLoading) return <div className="empty-state">Yükleniyor...</div>;
  if ((list.data ?? []).length === 0)
    return <div className="empty-state">Henüz hiç kasa oturumu yok. Üstten "Kasa Aç" ile başlayın.</div>;

  const zRaporHref = (id: string) =>
    paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "").vezne.zRapor(id);

  const columns: Column<CashRegister>[] = [
    { key: "no", header: "No", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.register_no}</code> },
    {
      key: "cashier",
      header: "Kasiyer",
      cell: (r) => <span className="font-medium">{r.cashier_name}</span>,
    },
    {
      key: "status",
      header: "Durum",
      cell: (r) => (
        <span className={
          r.status === "open"
            ? "rounded bg-emerald-100 px-2 py-0.5 text-xs text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200"
            : "rounded bg-slate-200 px-2 py-0.5 text-xs text-slate-800 dark:bg-slate-700/60 dark:text-slate-200"
        }>
          {r.status === "open" ? "Açık" : "Kapalı"}
        </span>
      ),
    },
    { key: "open", header: "Açılış", cell: (r) => new Date(r.opened_at).toLocaleString("tr-TR") },
    { key: "close", header: "Kapanış", cell: (r) => r.closed_at ? new Date(r.closed_at).toLocaleString("tr-TR") : "—" },
    {
      key: "delta",
      header: "Açılış → Sayım",
      cell: (r) => (
        <span className="text-xs">
          {formatTl(r.opening_balance)} → {r.declared_balance != null ? formatTl(r.declared_balance) : "—"}
        </span>
      ),
      className: "text-right",
    },
    {
      key: "z",
      header: "",
      cell: (r) => (
        <AppLink
          to={zRaporHref(r.id)}
          className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
        >
          <FileText className="h-3.5 w-3.5" /> Z Raporu
        </AppLink>
      ),
      className: "text-right",
    },
  ];

  return (
    <section>
      <h2 className="mb-2 flex items-center gap-2 text-sm font-semibold">
        <Wallet className="h-4 w-4" /> Geçmiş Oturumlar
      </h2>
      <DataTable<CashRegister> rows={list.data ?? []} rowKey={(r) => r.id} columns={columns} />
    </section>
  );
}

// ---------- Drawers ----------

function OpenRegisterSheet({
  open,
  onClose,
  branchId,
}: {
  open: boolean;
  onClose: () => void;
  branchId: string;
}) {
  const qc = useQueryClient();
  const [opening, setOpening] = useState("");
  const [notes, setNotes] = useState("");
  const create = useMutation({
    mutationFn: () => openRegister({ opening_balance: Number(opening) || 0, notes: notes.trim() || undefined }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: vezneKeys.all(branchId) });
      setOpening("");
      setNotes("");
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Kasa Aç">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="o-balance" label="Açılış bakiyesi (TRY)" required>
          <TextInput
            id="o-balance"
            type="number"
            min="0"
            step="0.01"
            required
            value={opening}
            onChange={(e) => setOpening(e.target.value)}
            placeholder="0,00"
          />
        </Field>
        <Field id="o-notes" label="Notlar">
          <Textarea id="o-notes" rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>
        {create.isError && (
          <p className="text-sm text-[var(--critical)]">
            Kasa açılamadı. Zaten açık bir kasanız olabilir.
          </p>
        )}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending}>
            {create.isPending ? "Açılıyor..." : "Kasayı aç"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function MovementSheet({
  kind,
  branchId,
  registerId,
  onClose,
}: {
  kind: CashMovementKind;
  branchId: string;
  registerId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [method, setMethod] = useState<PaymentMethod>("cash");
  const [amount, setAmount] = useState("");
  const [counterparty, setCounterparty] = useState("");
  const [description, setDescription] = useState("");

  const save = useMutation({
    mutationFn: (): Promise<{ movement_no: string }> => {
      const input: RecordMovementInput = {
        kind,
        method,
        amount: Number(amount),
        counterparty: counterparty.trim() || undefined,
        description: description.trim() || undefined,
      };
      return recordMovement(registerId, input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: vezneKeys.all(branchId) });
      onClose();
    },
  });

  return (
    <SideSheet open onClose={onClose} title={KIND_LABELS[kind]}>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
        <Field id="m-method" label="Ödeme yöntemi">
          <SelectInput
            id="m-method"
            value={method}
            onChange={(e) => setMethod(e.target.value as PaymentMethod)}
          >
            {Object.entries(METHOD_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <Field id="m-amount" label="Tutar (TRY)" required>
          <TextInput
            id="m-amount"
            type="number"
            min="0"
            step="0.01"
            required
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="0,00"
          />
        </Field>
        <Field id="m-cp" label="Karşı taraf / Hasta">
          <TextInput id="m-cp" value={counterparty} onChange={(e) => setCounterparty(e.target.value)} />
        </Field>
        <Field id="m-desc" label="Açıklama">
          <Textarea id="m-desc" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>
        {save.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız.</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={save.isPending || !amount || Number(amount) <= 0}>
            {save.isPending ? "Kaydediliyor..." : "Kaydet"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

// TR banknot + bozuk para denominasyonları — kasiyere kuponları girip
// otomatik toplam hesaplaması yapmasını sağlar.
const TRY_DENOMS = [200, 100, 50, 20, 10, 5, 1, 0.5, 0.25, 0.1, 0.05, 0.01];

function CloseRegisterSheet({
  branchId,
  register,
  expected,
  onClose,
}: {
  branchId: string;
  register: CashRegister;
  expected: number;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [declared, setDeclared] = useState(expected.toFixed(2));
  const [notes, setNotes] = useState("");
  const [counts, setCounts] = useState<Record<string, string>>({});
  const [useCounter, setUseCounter] = useState(false);

  // Live sum of denomination counts. Updating any field rolls back into
  // `declared` so the form stays the single source of truth.
  const setCount = (denom: number, raw: string) => {
    const next = { ...counts, [String(denom)]: raw };
    setCounts(next);
    const total = TRY_DENOMS.reduce((s, d) => {
      const c = Number(next[String(d)]) || 0;
      return s + c * d;
    }, 0);
    setDeclared(total.toFixed(2));
  };

  const save = useMutation({
    mutationFn: () => closeRegister(register.id, { declared_balance: Number(declared), notes: notes.trim() || undefined }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: vezneKeys.all(branchId) });
      onClose();
    },
  });

  const variance = Number(declared) - expected;

  return (
    <SideSheet open onClose={onClose} title={`Kasa Kapat · ${register.register_no}`}>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
        <div className="rounded-md border border-border bg-muted/40 p-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Beklenen kasa (nakit)</span>
            <span className="font-mono font-medium">{formatTl(expected)}</span>
          </div>
        </div>

        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={useCounter}
            onChange={(e) => setUseCounter(e.target.checked)}
          />
          <span>Banknot sayımı yardımcısı kullan</span>
        </label>

        {useCounter && (
          <div className="grid grid-cols-3 gap-2 rounded-md border border-border p-3">
            {TRY_DENOMS.map((d) => (
              <Field
                key={d}
                id={`d-${d}`}
                label={d >= 1 ? `${d} ₺` : `${(d * 100).toFixed(0)} kr`}
              >
                <TextInput
                  id={`d-${d}`}
                  type="number"
                  min="0"
                  step="1"
                  value={counts[String(d)] ?? ""}
                  onChange={(e) => setCount(d, e.target.value)}
                  placeholder="0"
                />
              </Field>
            ))}
          </div>
        )}

        <Field id="c-declared" label="Sayımda görünen (TRY)" required>
          <TextInput
            id="c-declared"
            type="number"
            min="0"
            step="0.01"
            required
            value={declared}
            onChange={(e) => setDeclared(e.target.value)}
          />
        </Field>

        {!Number.isNaN(variance) && variance !== 0 && (
          <div
            className={
              "rounded-md border p-3 text-sm " +
              (variance > 0
                ? "border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200"
                : "border-rose-200 bg-rose-50 text-rose-900 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-200")
            }
          >
            Fark: <span className="font-mono font-medium">{variance > 0 ? "+" : ""}{formatTl(variance)}</span>
            {variance < 0 && " — kasada eksik var. Sayımı kontrol edin."}
            {variance > 0 && " — kasada fazla var. Açıklamayı not edin."}
          </div>
        )}

        <Field id="c-notes" label="Notlar">
          <Textarea id="c-notes" rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>

        {save.isError && <p className="text-sm text-[var(--critical)]">Kapanış başarısız.</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={save.isPending}>
            <span className="inline-flex items-center gap-1">
              <LockKeyhole className="h-4 w-4" /> {save.isPending ? "Kapatılıyor..." : "Kasayı kapat"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
