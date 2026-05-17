"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Calculator, ChevronRight, Percent, Plus, Users } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { bransListOptions } from "@medigt/core/brans";
import {
  bulkCreateCommissionRules,
  commissionRulesOptions,
  createCommissionRule,
  hakedisItemsOptions,
  hakedisKeys,
  hakedisSummaryOptions,
  previewBulkCommissionRules,
  type BulkCommissionRuleInput,
  type BulkCommissionRuleResult,
  type CreateCommissionRuleInput,
  type HakedisRange,
} from "@medigt/core/hakedis";
import { formatTl } from "@medigt/core/utils";
import type {
  CommissionRule,
  HakedisItem,
  HakedisSummary,
  ServiceCategoryKey,
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

const CATEGORY_LABELS: Record<ServiceCategoryKey, string> = {
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

function monthStart(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-01`;
}

function monthEnd(): string {
  const d = new Date();
  const last = new Date(d.getFullYear(), d.getMonth() + 1, 0);
  return `${last.getFullYear()}-${String(last.getMonth() + 1).padStart(2, "0")}-${String(last.getDate()).padStart(2, "0")}`;
}

export function HakedisPage() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const [range, setRange] = useState<HakedisRange>({ from: monthStart(), to: monthEnd() });
  const summary = useQuery(hakedisSummaryOptions(branchId, range));
  const [openDoctor, setOpenDoctor] = useState<HakedisSummary | null>(null);
  const [rulesFor, setRulesFor] = useState<HakedisSummary | null>(null);
  const [bulkOpen, setBulkOpen] = useState(false);

  const totalEarnings = (summary.data ?? []).reduce((s, r) => s + r.earning_total, 0);
  const totalRevenue = (summary.data ?? []).reduce((s, r) => s + r.gross_revenue, 0);
  const totalCount = (summary.data ?? []).reduce((s, r) => s + r.item_count, 0);

  const columns: Column<HakedisSummary>[] = [
    {
      key: "doctor",
      header: "Doktor",
      cell: (r) => (
        <div>
          <div className="font-medium">
            {r.title ? r.title + " " : ""}{r.first_name} {r.last_name}
          </div>
          <button
            type="button"
            onClick={() => setRulesFor(r)}
            className="text-xs text-muted-foreground hover:underline"
          >
            <Percent className="mr-1 inline h-3 w-3" />Komisyon kuralları
          </button>
        </div>
      ),
    },
    { key: "count", header: "Hizmet", cell: (r) => r.item_count, className: "text-right" },
    {
      key: "rev",
      header: "Brüt Hasılat",
      cell: (r) => <span className="font-mono">{formatTl(r.gross_revenue)}</span>,
      className: "text-right",
    },
    {
      key: "earn",
      header: "Hakediş",
      cell: (r) => <span className="font-mono font-semibold text-emerald-700 dark:text-emerald-300">{formatTl(r.earning_total)}</span>,
      className: "text-right",
    },
    {
      key: "open",
      header: "",
      cell: (r) => (
        <button
          type="button"
          onClick={() => setOpenDoctor(r)}
          className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
        >
          Detay <ChevronRight className="h-3.5 w-3.5" />
        </button>
      ),
      className: "text-right",
    },
  ];

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Hakediş"
          subtitle="Ödenmiş faturalardaki doktor kalemleri komisyon kurallarına göre toplanır. Tarih aralığı seçip detaya inebilirsiniz."
          actions={
            <PrimaryButton type="button" onClick={() => setBulkOpen(true)}>
              <span className="inline-flex items-center gap-1">
                <Users className="h-4 w-4" /> Toplu Kural
              </span>
            </PrimaryButton>
          }
        />

        <div className="flex flex-wrap items-end gap-3">
          <Field id="h-from" label="Başlangıç">
            <TextInput id="h-from" type="date" value={range.from}
              onChange={(e) => setRange({ ...range, from: e.target.value })} />
          </Field>
          <Field id="h-to" label="Bitiş">
            <TextInput id="h-to" type="date" value={range.to}
              onChange={(e) => setRange({ ...range, to: e.target.value })} />
          </Field>
          <SecondaryButton
            type="button"
            onClick={() => setRange({ from: monthStart(), to: monthEnd() })}
          >
            Bu ay
          </SecondaryButton>
          <SecondaryButton
            type="button"
            onClick={() => {
              const d = new Date();
              const prev = new Date(d.getFullYear(), d.getMonth() - 1, 1);
              const from = `${prev.getFullYear()}-${String(prev.getMonth() + 1).padStart(2, "0")}-01`;
              const last = new Date(prev.getFullYear(), prev.getMonth() + 1, 0);
              const to = `${last.getFullYear()}-${String(last.getMonth() + 1).padStart(2, "0")}-${String(last.getDate()).padStart(2, "0")}`;
              setRange({ from, to });
            }}
          >
            Geçen ay
          </SecondaryButton>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <Summary label="Toplam Brüt Hasılat" value={formatTl(totalRevenue)} />
          <Summary label="Toplam Hakediş" value={formatTl(totalEarnings)} tone="emerald" />
          <Summary label="Hizmet Sayısı" value={String(totalCount)} />
        </div>

        {summary.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : (summary.data ?? []).length === 0 ? (
          <div className="empty-state">Bu tarih aralığında ödenmiş kalem yok.</div>
        ) : (
          <DataTable<HakedisSummary>
            rows={summary.data ?? []}
            rowKey={(r) => r.doctor_id}
            columns={columns}
          />
        )}
      </div>

      {openDoctor && (
        <DoctorDetailSheet
          doctor={openDoctor}
          range={range}
          branchId={branchId}
          onClose={() => setOpenDoctor(null)}
        />
      )}
      {rulesFor && (
        <RulesSheet
          doctor={rulesFor}
          onClose={() => setRulesFor(null)}
        />
      )}
      {bulkOpen && (
        <BulkRuleSheet
          onClose={() => setBulkOpen(false)}
        />
      )}
    </DashboardLayout>
  );
}

function Summary({ label, value, tone }: { label: string; value: string; tone?: "emerald" }) {
  return (
    <div className="rounded-md border border-border bg-card p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={"mt-1 font-mono text-lg " + (tone === "emerald" ? "font-semibold text-emerald-700 dark:text-emerald-300" : "")}>
        {value}
      </div>
    </div>
  );
}

function DoctorDetailSheet({
  doctor,
  range,
  branchId,
  onClose,
}: {
  doctor: HakedisSummary;
  range: HakedisRange;
  branchId: string;
  onClose: () => void;
}) {
  const items = useQuery(hakedisItemsOptions(branchId, doctor.doctor_id, range));
  const columns: Column<HakedisItem>[] = [
    {
      key: "at",
      header: "Tarih",
      cell: (r) => <span className="text-xs">{new Date(r.issued_at).toLocaleDateString("tr-TR")}</span>,
    },
    {
      key: "inv",
      header: "Fatura",
      cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.invoice_no}</code>,
    },
    {
      key: "pt",
      header: "Hasta",
      cell: (r) => (
        <div className="text-xs">
          <div className="font-medium">{r.patient_first_name} {r.patient_last_name}</div>
          <div className="text-muted-foreground">MRN {r.patient_mrn}</div>
        </div>
      ),
    },
    {
      key: "svc",
      header: "Hizmet",
      cell: (r) => (
        <div>
          <div className="text-sm font-medium">{r.name}</div>
          <div className="text-xs text-muted-foreground">
            <code className="rounded bg-muted px-1">{r.code}</code>
            {r.category && ` · ${CATEGORY_LABELS[r.category] ?? r.category}`}
          </div>
        </div>
      ),
    },
    {
      key: "tot",
      header: "Tutar",
      cell: (r) => <span className="font-mono text-xs">{formatTl(r.line_total)}</span>,
      className: "text-right",
    },
    {
      key: "pct",
      header: "%",
      cell: (r) => <span className="font-mono text-xs">{r.commission_pct.toFixed(1)}</span>,
      className: "text-right",
    },
    {
      key: "earn",
      header: "Hakediş",
      cell: (r) => <span className="font-mono text-sm font-medium text-emerald-700 dark:text-emerald-300">{formatTl(r.earning)}</span>,
      className: "text-right",
    },
  ];
  return (
    <SideSheet open onClose={onClose} title={`Detay · ${doctor.first_name} ${doctor.last_name}`}>
      <div className="space-y-3">
        <div className="rounded-md border border-border bg-muted/40 p-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Toplam hakediş</span>
            <span className="font-mono font-semibold">{formatTl(doctor.earning_total)}</span>
          </div>
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>Brüt: {formatTl(doctor.gross_revenue)}</span>
            <span>{doctor.item_count} hizmet</span>
          </div>
        </div>
        {items.isLoading ? (
          <p className="text-sm text-muted-foreground">Yükleniyor...</p>
        ) : (items.data ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">Bu aralıkta kalem yok.</p>
        ) : (
          <DataTable<HakedisItem>
            rows={items.data ?? []}
            rowKey={(r) => r.invoice_item_id}
            columns={columns}
          />
        )}
      </div>
    </SideSheet>
  );
}

function RulesSheet({
  doctor,
  onClose,
}: {
  doctor: HakedisSummary;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const rules = useQuery(commissionRulesOptions(doctor.doctor_id));
  const [form, setForm] = useState<CreateCommissionRuleInput>({ commission_pct: 0 });
  const create = useMutation({
    mutationFn: () => createCommissionRule(doctor.doctor_id, form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: hakedisKeys.rules(doctor.doctor_id) });
      await qc.invalidateQueries({ queryKey: ["hakedis"] });
      setForm({ commission_pct: 0 });
    },
  });

  return (
    <SideSheet open onClose={onClose} title={`Komisyon · ${doctor.first_name} ${doctor.last_name}`}>
      <div className="space-y-6">
        <section className="space-y-2">
          <h3 className="text-sm font-semibold">Mevcut kurallar</h3>
          {rules.isLoading ? (
            <p className="text-sm text-muted-foreground">Yükleniyor...</p>
          ) : (rules.data ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">Henüz kural tanımlı değil — alttan ekleyebilirsiniz.</p>
          ) : (
            <ul className="space-y-1 text-sm">
              {(rules.data ?? []).map((r) => (
                <RuleRow key={r.id} rule={r} />
              ))}
            </ul>
          )}
        </section>

        <form
          className="space-y-3 border-t border-border pt-4"
          onSubmit={(e) => { e.preventDefault(); create.mutate(); }}
        >
          <h3 className="text-sm font-semibold flex items-center gap-1">
            <Plus className="h-4 w-4" /> Yeni kural
          </h3>
          <Field id="r-cat" label="Kategori" hint="Boş bırakılırsa tüm kategorilere uygulanır">
            <SelectInput
              id="r-cat"
              value={form.category ?? ""}
              onChange={(e) => setForm({ ...form, category: (e.target.value || undefined) as ServiceCategoryKey | undefined })}
            >
              <option value="">Tüm kategoriler</option>
              {Object.entries(CATEGORY_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="r-pct" label="Komisyon %" required>
            <TextInput
              id="r-pct"
              type="number"
              min="0"
              max="100"
              step="0.1"
              required
              value={String(form.commission_pct)}
              onChange={(e) => setForm({ ...form, commission_pct: Number(e.target.value) || 0 })}
            />
          </Field>
          <Field id="r-from" label="Geçerlilik başlangıcı" hint="Boş bırakılırsa bugün">
            <TextInput
              id="r-from"
              type="date"
              value={form.valid_from ?? ""}
              onChange={(e) => setForm({ ...form, valid_from: e.target.value })}
            />
          </Field>
          <Field id="r-notes" label="Notlar">
            <Textarea
              id="r-notes"
              rows={2}
              value={form.notes ?? ""}
              onChange={(e) => setForm({ ...form, notes: e.target.value })}
            />
          </Field>
          {create.isError && (
            <p className="text-sm text-[var(--critical)]">Kayıt başarısız. Bu tarih için aynı kategoride kural olabilir.</p>
          )}
          {create.isSuccess && (
            <p className="text-sm text-emerald-700 dark:text-emerald-300">Eklendi.</p>
          )}
          <div className="flex gap-2">
            <SecondaryButton type="button" onClick={onClose} className="flex-1">Kapat</SecondaryButton>
            <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || form.commission_pct <= 0}>
              <span className="inline-flex items-center gap-1">
                <Calculator className="h-4 w-4" /> {create.isPending ? "Ekleniyor..." : "Kural ekle"}
              </span>
            </PrimaryButton>
          </div>
        </form>
      </div>
    </SideSheet>
  );
}

// BulkRuleSheet lets an admin apply a single commission rule to many
// doctors at once — either by branş (specialization) selection or by
// explicit doctor picks. Shows a live "kaç doktor etkilenecek?" preview
// before the user clicks "Uygula".
function BulkRuleSheet({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient();
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const branses = useQuery(bransListOptions(orgId));

  const [form, setForm] = useState<BulkCommissionRuleInput>({
    commission_pct: 30,
    specialization_codes: [],
  });
  const [result, setResult] = useState<BulkCommissionRuleResult | null>(null);

  const filterReady =
    (form.specialization_codes?.length ?? 0) > 0 ||
    (form.doctor_ids?.length ?? 0) > 0;

  const preview = useQuery({
    queryKey: [
      "hakedis",
      "bulk-preview",
      orgId,
      (form.specialization_codes ?? []).join(","),
      (form.doctor_ids ?? []).join(","),
    ],
    queryFn: () => previewBulkCommissionRules(form),
    enabled: filterReady,
  });

  const apply = useMutation({
    mutationFn: () => bulkCreateCommissionRules(form),
    onSuccess: async (res) => {
      setResult(res);
      await qc.invalidateQueries({ queryKey: ["hakedis"] });
    },
  });

  const toggleSpec = (code: string) => {
    const set = new Set(form.specialization_codes ?? []);
    if (set.has(code)) set.delete(code);
    else set.add(code);
    setForm({ ...form, specialization_codes: [...set] });
  };

  return (
    <SideSheet open onClose={onClose} title="Toplu Komisyon Kuralı">
      <div className="space-y-5">
        <p className="text-sm text-muted-foreground">
          Seçtiğiniz branşlardaki tüm doktorlara aynı komisyon oranını uygular. Önceden tanımlı aktif kurallar otomatik kapatılır.
        </p>

        <section className="space-y-2">
          <h3 className="text-sm font-semibold">1) Hedef branşlar</h3>
          {branses.isLoading ? (
            <p className="text-sm text-muted-foreground">Yükleniyor...</p>
          ) : (branses.data ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">Branş tanımı yok.</p>
          ) : (
            <div className="grid max-h-48 grid-cols-2 gap-1 overflow-auto rounded-md border border-border p-2">
              {(branses.data ?? []).map((b) => {
                const checked = (form.specialization_codes ?? []).includes(b.code);
                return (
                  <label
                    key={b.id}
                    className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-muted"
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggleSpec(b.code)}
                    />
                    <span>{b.name}</span>
                  </label>
                );
              })}
            </div>
          )}
        </section>

        <section className="space-y-3">
          <h3 className="text-sm font-semibold">2) Kural detayı</h3>
          <Field id="b-cat" label="Kategori" hint="Boş bırakılırsa tüm kategorilere uygulanır">
            <SelectInput
              id="b-cat"
              value={form.category ?? ""}
              onChange={(e) =>
                setForm({
                  ...form,
                  category: (e.target.value || undefined) as ServiceCategoryKey | undefined,
                })
              }
            >
              <option value="">Tüm kategoriler</option>
              {Object.entries(CATEGORY_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="b-pct" label="Komisyon %" required>
            <TextInput
              id="b-pct"
              type="number"
              min="0"
              max="100"
              step="0.1"
              required
              value={String(form.commission_pct)}
              onChange={(e) => setForm({ ...form, commission_pct: Number(e.target.value) || 0 })}
            />
          </Field>
          <Field id="b-from" label="Geçerlilik başlangıcı" hint="Boş bırakılırsa bugün">
            <TextInput
              id="b-from"
              type="date"
              value={form.valid_from ?? ""}
              onChange={(e) => setForm({ ...form, valid_from: e.target.value })}
            />
          </Field>
          <Field id="b-notes" label="Notlar">
            <Textarea
              id="b-notes"
              rows={2}
              value={form.notes ?? ""}
              onChange={(e) => setForm({ ...form, notes: e.target.value })}
            />
          </Field>
        </section>

        <section className="rounded-md border border-border bg-muted/40 p-3 text-sm">
          {!filterReady ? (
            <p className="text-muted-foreground">
              Önizleme için en az bir branş seçin.
            </p>
          ) : preview.isLoading ? (
            <p className="text-muted-foreground">Hesaplanıyor...</p>
          ) : preview.data ? (
            <p>
              <span className="font-semibold">{preview.data.targeted_doctors}</span> doktor etkilenecek.
            </p>
          ) : (
            <p className="text-muted-foreground">Önizleme alınamadı.</p>
          )}
        </section>

        {result && (
          <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200">
            <div className="font-semibold">Tamamlandı</div>
            <div>
              {result.rules_added} kural eklendi · {result.skipped} atlandı (toplam {result.targeted_doctors} doktor hedeflendi).
            </div>
            {result.errors.length > 0 && (
              <details className="mt-2">
                <summary className="cursor-pointer text-xs">Hatalar ({result.errors.length})</summary>
                <ul className="mt-1 list-disc pl-4 text-xs">
                  {result.errors.map((e, i) => <li key={i}>{e}</li>)}
                </ul>
              </details>
            )}
          </div>
        )}

        {apply.isError && (
          <p className="text-sm text-[var(--critical)]">İstek başarısız oldu. Lütfen tekrar deneyin.</p>
        )}

        <div className="flex gap-2 border-t border-border pt-4">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">
            Kapat
          </SecondaryButton>
          <PrimaryButton
            type="button"
            className="flex-1"
            onClick={() => { setResult(null); apply.mutate(); }}
            disabled={!filterReady || apply.isPending || form.commission_pct <= 0}
          >
            <span className="inline-flex items-center gap-1">
              <Calculator className="h-4 w-4" />
              {apply.isPending ? "Uygulanıyor..." : "Uygula"}
            </span>
          </PrimaryButton>
        </div>
      </div>
    </SideSheet>
  );
}

function RuleRow({ rule }: { rule: CommissionRule }) {
  return (
    <li className="flex items-center justify-between rounded-md border border-border px-3 py-2">
      <div>
        <div className="font-medium">
          {rule.category ? CATEGORY_LABELS[rule.category] : "Tüm kategoriler"}
        </div>
        <div className="text-xs text-muted-foreground">
          {rule.valid_from}
          {rule.valid_to ? ` → ${rule.valid_to}` : " → şu an"}
        </div>
      </div>
      <span className="font-mono text-sm font-medium">%{rule.commission_pct.toFixed(1)}</span>
    </li>
  );
}
