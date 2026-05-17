"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Building2,
  Calculator,
  Calendar,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  ListChecks,
  Percent,
  TrendingUp,
} from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  applyBulkPriceUpdate,
  previewBulkPriceUpdate,
  type BulkPriceAdjustment,
  type BulkPriceFilter,
  type BulkPricePreview,
  type BulkPriceResult,
} from "@medigt/core/hizmet";
import { hizmetListOptions } from "@medigt/core/hizmet";
import { kurumListOptions } from "@medigt/core/kurum";
import { formatTl } from "@medigt/core/utils";
import type { ServiceCategory } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";
import { DataTable, type Column } from "../../common/data-table";

// 4-step wizard for bulk service-price updates.
//
//   1. Kapsam — hangi hizmetler × hangi kurumlar
//   2. Ayar    — yüzde / sabit ekleme / set + valid_from + opsiyonel
//                min/max kırpma + not
//   3. Önizleme — backend resolveTargetRows; "X fiyat etkilenecek,
//                ortalama %Y, toplam ₺Z" özet + tablo
//   4. Uygula  — transactional bulk insert + valid_to stamp
//
// State lives in the component (filter + adjustment); query layer cache
// holds the preview. Step 3 is mounted whenever filter+adjustment is
// "complete enough" so the user can iterate without committing.

type Step = 1 | 2 | 3 | 4;

const CATEGORY_LABELS: Record<ServiceCategory, string> = {
  consultation: "Muayene",
  lab: "Laboratuvar",
  imaging: "Radyoloji / Görüntüleme",
  procedure: "Girişimsel İşlem",
  surgery: "Ameliyat",
  inpatient: "Yatış",
  medication: "İlaç",
  supply: "Sarf",
  package: "Paket",
  other: "Diğer",
};

const KIND_LABELS: Record<"percent" | "fixed" | "set", string> = {
  percent: "% Yüzde uygula",
  fixed: "₺ Sabit ekle/çıkar",
  set: "= Yeni fiyata ata",
};

export function FiyatGuncellemePage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";

  const [step, setStep] = useState<Step>(1);
  const [filter, setFilter] = useState<BulkPriceFilter>({
    category: "",
    service_ids: [],
    institution_ids: [],
    include_oop: true,
  });
  const [adj, setAdj] = useState<BulkPriceAdjustment>({
    kind: "percent",
    amount: 10,
    valid_from: new Date().toISOString().slice(0, 10),
    notes: "",
  });
  const [result, setResult] = useState<BulkPriceResult | null>(null);

  const input = useMemo(() => ({ ...filter, ...adj }), [filter, adj]);

  const preview = useQuery({
    queryKey: ["fiyat-bulk-preview", input],
    queryFn: () => previewBulkPriceUpdate(input),
    enabled: step >= 3 && !!orgId,
  });

  const qc = useQueryClient();
  const apply = useMutation({
    mutationFn: () => applyBulkPriceUpdate(input),
    onSuccess: async (res) => {
      setResult(res);
      await qc.invalidateQueries({ queryKey: ["hizmet"] });
      await qc.invalidateQueries({ queryKey: ["fiyat-bulk-preview"] });
      setStep(4);
    },
  });

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Fiyat Güncelleme Sihirbazı"
          subtitle="Hizmet kataloğundaki fiyatları kuruma göre toplu güncelle. Eski fiyat satırları otomatik kapatılır, yeni satırlar valid_from tarihinden itibaren geçerlidir."
        />

        <Stepper step={step} />

        {step === 1 && (
          <ScopeStep
            orgId={orgId}
            filter={filter}
            onChange={setFilter}
            onNext={() => setStep(2)}
          />
        )}
        {step === 2 && (
          <AdjustmentStep
            adj={adj}
            onChange={setAdj}
            onBack={() => setStep(1)}
            onNext={() => setStep(3)}
          />
        )}
        {step === 3 && (
          <PreviewStep
            preview={preview.data}
            loading={preview.isLoading}
            onBack={() => setStep(2)}
            onApply={() => apply.mutate()}
            applying={apply.isPending}
            applyError={apply.isError}
          />
        )}
        {step === 4 && result && (
          <DoneStep
            result={result}
            onAnother={() => {
              setResult(null);
              setStep(1);
            }}
          />
        )}
      </div>
    </DashboardLayout>
  );
}

function Stepper({ step }: { step: Step }) {
  const labels: { n: Step; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
    { n: 1, label: "Kapsam", icon: ListChecks },
    { n: 2, label: "Ayar", icon: Percent },
    { n: 3, label: "Önizleme", icon: TrendingUp },
    { n: 4, label: "Tamam", icon: CheckCircle2 },
  ];
  return (
    <ol className="flex items-center gap-2">
      {labels.map((s, i) => {
        const Icon = s.icon;
        const active = step === s.n;
        const done = step > s.n;
        return (
          <li
            key={s.n}
            className="step-pill"
            data-state={active ? "active" : done ? "done" : "pending"}
          >
            <Icon className="h-4 w-4" />
            <span>
              {s.n}. {s.label}
            </span>
            {i < labels.length - 1 && <ChevronRight className="ml-1 h-3 w-3 opacity-50" />}
          </li>
        );
      })}
    </ol>
  );
}

// ---------- Step 1: Kapsam ----------

function ScopeStep({
  orgId,
  filter,
  onChange,
  onNext,
}: {
  orgId: string;
  filter: BulkPriceFilter;
  onChange: (f: BulkPriceFilter) => void;
  onNext: () => void;
}) {
  const services = useQuery(hizmetListOptions(orgId, { category: filter.category || undefined }));
  const institutions = useQuery(kurumListOptions(orgId));

  const allServiceIds = (services.data ?? []).map((s) => s.id);

  return (
    <section className="space-y-4 rounded-lg border border-border bg-card p-4">
      <h2 className="text-base font-semibold">1) Hangi fiyatları güncelle?</h2>

      <Field id="f-category" label="Kategori (boş = tümü)">
        <SelectInput
          id="f-category"
          value={filter.category ?? ""}
          onChange={(e) =>
            onChange({ ...filter, category: (e.target.value || "") as ServiceCategory | "" })
          }
        >
          <option value="">Tüm kategoriler</option>
          {(Object.keys(CATEGORY_LABELS) as ServiceCategory[]).map((k) => (
            <option key={k} value={k}>
              {CATEGORY_LABELS[k]}
            </option>
          ))}
        </SelectInput>
      </Field>

      <div>
        <div className="mb-1 flex items-center justify-between text-sm">
          <span className="font-medium">Hizmet seçimi</span>
          <span className="text-xs text-muted-foreground">
            Boş bırakırsanız kategori filtresine uyan TÜM hizmetler etkilenir.
          </span>
        </div>
        {services.isLoading ? (
          <p className="text-sm text-muted-foreground">Yükleniyor...</p>
        ) : (
          <div className="flex items-center gap-2">
            <SecondaryButton
              type="button"
              onClick={() => onChange({ ...filter, service_ids: allServiceIds })}
            >
              Tümünü seç ({allServiceIds.length})
            </SecondaryButton>
            <SecondaryButton
              type="button"
              onClick={() => onChange({ ...filter, service_ids: [] })}
            >
              Hiçbiri (tüm kategori)
            </SecondaryButton>
            <span className="text-xs text-muted-foreground">
              {filter.service_ids?.length ?? 0} seçili
            </span>
          </div>
        )}
        {(services.data ?? []).length > 0 && (
          <div className="mt-2 grid max-h-48 grid-cols-1 gap-1 overflow-auto rounded-md border border-border p-2 sm:grid-cols-2">
            {(services.data ?? []).map((s) => {
              const checked = (filter.service_ids ?? []).includes(s.id);
              return (
                <label
                  key={s.id}
                  className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-muted"
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => {
                      const set = new Set(filter.service_ids ?? []);
                      if (set.has(s.id)) set.delete(s.id);
                      else set.add(s.id);
                      onChange({ ...filter, service_ids: [...set] });
                    }}
                  />
                  <span className="font-mono text-xs text-muted-foreground">{s.code}</span>
                  <span className="truncate">{s.name}</span>
                </label>
              );
            })}
          </div>
        )}
      </div>

      <div>
        <div className="mb-1 flex items-center justify-between text-sm">
          <span className="font-medium">Kurum seçimi</span>
          <label className="flex items-center gap-2 text-xs">
            <input
              type="checkbox"
              checked={filter.include_oop ?? false}
              onChange={(e) => onChange({ ...filter, include_oop: e.target.checked })}
            />
            <span>Cepten/varsayılan fiyatı da dahil et</span>
          </label>
        </div>
        {institutions.isLoading ? (
          <p className="text-sm text-muted-foreground">Yükleniyor...</p>
        ) : (institutions.data ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">Kurum tanımı yok.</p>
        ) : (
          <div className="grid max-h-40 grid-cols-1 gap-1 overflow-auto rounded-md border border-border p-2 sm:grid-cols-2">
            {(institutions.data ?? []).map((inst) => {
              const checked = (filter.institution_ids ?? []).includes(inst.id);
              return (
                <label
                  key={inst.id}
                  className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-muted"
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => {
                      const set = new Set(filter.institution_ids ?? []);
                      if (set.has(inst.id)) set.delete(inst.id);
                      else set.add(inst.id);
                      onChange({ ...filter, institution_ids: [...set] });
                    }}
                  />
                  <Building2 className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>{inst.name}</span>
                </label>
              );
            })}
          </div>
        )}
      </div>

      <div className="flex justify-end">
        <PrimaryButton type="button" onClick={onNext}>
          İleri <ChevronRight className="ml-1 inline h-4 w-4" />
        </PrimaryButton>
      </div>
    </section>
  );
}

// ---------- Step 2: Ayar ----------

function AdjustmentStep({
  adj,
  onChange,
  onBack,
  onNext,
}: {
  adj: BulkPriceAdjustment;
  onChange: (a: BulkPriceAdjustment) => void;
  onBack: () => void;
  onNext: () => void;
}) {
  return (
    <section className="space-y-4 rounded-lg border border-border bg-card p-4">
      <h2 className="text-base font-semibold">2) Nasıl güncellensin?</h2>

      <Field id="a-kind" label="Yöntem">
        <SelectInput
          id="a-kind"
          value={adj.kind}
          onChange={(e) =>
            onChange({ ...adj, kind: e.target.value as BulkPriceAdjustment["kind"] })
          }
        >
          {(Object.keys(KIND_LABELS) as Array<keyof typeof KIND_LABELS>).map((k) => (
            <option key={k} value={k}>
              {KIND_LABELS[k]}
            </option>
          ))}
        </SelectInput>
      </Field>

      <Field
        id="a-amount"
        label={adj.kind === "percent" ? "Yüzde (örn. 10 → %10 zam)" : adj.kind === "set" ? "Yeni fiyat (₺)" : "Eklenecek tutar (₺ — eksi için negatif)"}
        required
      >
        <TextInput
          id="a-amount"
          type="number"
          step={adj.kind === "percent" ? "0.1" : "0.01"}
          value={String(adj.amount)}
          onChange={(e) => onChange({ ...adj, amount: Number(e.target.value) || 0 })}
          required
        />
      </Field>

      <Field id="a-from" label="Geçerlilik başlangıcı">
        <TextInput
          id="a-from"
          type="date"
          value={adj.valid_from ?? ""}
          onChange={(e) => onChange({ ...adj, valid_from: e.target.value })}
        />
      </Field>

      <div className="grid grid-cols-2 gap-3">
        <Field id="a-min" label="Min fiyat (kırp)" hint="0 = sınırsız">
          <TextInput
            id="a-min"
            type="number"
            min="0"
            step="0.01"
            value={String(adj.min_price ?? 0)}
            onChange={(e) =>
              onChange({ ...adj, min_price: Number(e.target.value) || 0 })
            }
          />
        </Field>
        <Field id="a-max" label="Max fiyat (kırp)" hint="0 = sınırsız">
          <TextInput
            id="a-max"
            type="number"
            min="0"
            step="0.01"
            value={String(adj.max_price ?? 0)}
            onChange={(e) =>
              onChange({ ...adj, max_price: Number(e.target.value) || 0 })
            }
          />
        </Field>
      </div>

      <Field id="a-notes" label="Not (yeni fiyat satırlarına yazılır)">
        <Textarea
          id="a-notes"
          rows={2}
          value={adj.notes ?? ""}
          onChange={(e) => onChange({ ...adj, notes: e.target.value })}
          placeholder="örn. 2026 yıl başı zam, SGK tarife güncelleme"
        />
      </Field>

      <div className="flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
        <Calendar className="h-4 w-4 flex-shrink-0" />
        <span>
          Eski fiyat satırlarının <strong>valid_to</strong> tarihi, yeni fiyat
          tarihinden bir gün öncesine otomatik atanır. Geçmiş raporlamada
          süreklilik korunur.
        </span>
      </div>

      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack}>
          <ChevronLeft className="mr-1 inline h-4 w-4" /> Geri
        </SecondaryButton>
        <PrimaryButton type="button" onClick={onNext}>
          Önizle <ChevronRight className="ml-1 inline h-4 w-4" />
        </PrimaryButton>
      </div>
    </section>
  );
}

// ---------- Step 3: Önizleme ----------

function PreviewStep({
  preview,
  loading,
  onBack,
  onApply,
  applying,
  applyError,
}: {
  preview: BulkPricePreview | undefined;
  loading: boolean;
  onBack: () => void;
  onApply: () => void;
  applying: boolean;
  applyError: boolean;
}) {
  if (loading || !preview) {
    return (
      <section className="rounded-lg border border-border bg-card p-4">
        <p className="text-sm text-muted-foreground">Önizleme hesaplanıyor...</p>
      </section>
    );
  }

  const columns: Column<BulkPricePreview["rows"][number]>[] = [
    {
      key: "svc",
      header: "Hizmet",
      cell: (r) => (
        <div>
          <div className="text-sm font-medium">{r.service_name}</div>
          <code className="text-xs text-muted-foreground">{r.service_code}</code>
        </div>
      ),
    },
    {
      key: "inst",
      header: "Kurum",
      cell: (r) => r.institution_name ?? <span className="text-xs italic text-muted-foreground">Cepten/varsayılan</span>,
    },
    {
      key: "old",
      header: "Eski",
      cell: (r) => <span className="font-mono text-xs">{formatTl(r.old_price)}</span>,
      className: "text-right",
    },
    {
      key: "new",
      header: "Yeni",
      cell: (r) => (
        <span
          className={
            "font-mono text-xs font-medium " +
            (r.new_price > r.old_price
              ? "text-emerald-700 dark:text-emerald-300"
              : r.new_price < r.old_price
                ? "text-rose-700 dark:text-rose-300"
                : "")
          }
        >
          {formatTl(r.new_price)}
        </span>
      ),
      className: "text-right",
    },
    {
      key: "delta",
      header: "Δ",
      cell: (r) => {
        if (r.old_price === 0) return "—";
        const pct = ((r.new_price - r.old_price) / r.old_price) * 100;
        return (
          <span className="font-mono text-xs">
            {pct > 0 ? "+" : ""}
            {pct.toFixed(1)}%
          </span>
        );
      },
      className: "text-right",
    },
  ];

  return (
    <section className="space-y-4 rounded-lg border border-border bg-card p-4">
      <h2 className="text-base font-semibold">3) Önizleme</h2>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Stat label="Etkilenen satır" value={String(preview.affected)} />
        <Stat label="Değişen" value={String(preview.changed)} />
        <Stat label="Ortalama Δ%" value={`${preview.avg_pct > 0 ? "+" : ""}${preview.avg_pct.toFixed(1)}%`} />
        <Stat label="Toplam Δ" value={formatTl(preview.total_new - preview.total_old)} tone={preview.total_new >= preview.total_old ? "positive" : "negative"} />
      </div>

      {preview.affected === 0 ? (
        <div className="empty-state">Filtreye uyan fiyat satırı yok. Adım 1'e dönün.</div>
      ) : (
        <DataTable
          rows={preview.rows}
          rowKey={(r) => r.service_id + (r.institution_id ?? "oop")}
          columns={columns}
        />
      )}

      {applyError && (
        <p className="text-sm text-[var(--critical)]">Uygulama başarısız. Tekrar deneyin.</p>
      )}

      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack}>
          <ChevronLeft className="mr-1 inline h-4 w-4" /> Geri
        </SecondaryButton>
        <PrimaryButton
          type="button"
          onClick={onApply}
          disabled={preview.changed === 0 || applying}
        >
          <span className="inline-flex items-center gap-1">
            <Calculator className="h-4 w-4" />
            {applying
              ? "Uygulanıyor..."
              : `Uygula (${preview.changed} satır)`}
          </span>
        </PrimaryButton>
      </div>
    </section>
  );
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: "positive" | "negative" }) {
  const cls =
    tone === "positive"
      ? "text-emerald-700 dark:text-emerald-300"
      : tone === "negative"
        ? "text-rose-700 dark:text-rose-300"
        : "";
  return (
    <div className="rounded-md border border-border bg-background p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={"mt-1 font-mono text-lg " + cls}>{value}</div>
    </div>
  );
}

// ---------- Step 4: Tamam ----------

function DoneStep({
  result,
  onAnother,
}: {
  result: BulkPriceResult;
  onAnother: () => void;
}) {
  return (
    <section className="space-y-4 rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-900 dark:bg-emerald-950/30">
      <h2 className="flex items-center gap-2 text-base font-semibold text-emerald-900 dark:text-emerald-200">
        <CheckCircle2 className="h-5 w-5" /> Güncelleme tamamlandı
      </h2>
      <ul className="space-y-1 text-sm text-emerald-900 dark:text-emerald-200">
        <li>
          <strong>{result.inserted}</strong> yeni fiyat satırı eklendi
        </li>
        <li>
          <strong>{result.skipped}</strong> satır değişmediği için atlandı
        </li>
        <li>
          <strong>{result.affected}</strong> hedef değerlendirildi
        </li>
      </ul>
      <PrimaryButton type="button" onClick={onAnother}>
        Yeni güncelleme başlat
      </PrimaryButton>
    </section>
  );
}
