"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createHizmet,
  createHizmetPrice,
  hizmetKeys,
  hizmetListOptions,
  hizmetPricesOptions,
  type CreateHizmetInput,
  type CreateHizmetPriceInput,
} from "@medigt/core/hizmet";
import { kurumListOptions } from "@medigt/core/kurum";
import { formatTl } from "@medigt/core/utils";
import type { ServiceCatalogItem, ServiceCategory } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import { Field, PrimaryButton, SecondaryButton, SelectInput, Textarea, TextInput } from "../../common/form-fields";

const CATEGORY_LABELS: Record<ServiceCategory, string> = {
  consultation: "Muayene",
  lab: "Laboratuvar",
  imaging: "Radyoloji / Görüntüleme",
  procedure: "Girişimsel İşlem",
  surgery: "Ameliyat",
  inpatient: "Yatış / Yatak",
  medication: "İlaç",
  supply: "Sarf Malzeme",
  package: "Paket Hizmet",
  other: "Diğer",
};

export function HizmetPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<ServiceCategory | "">("");
  const filter = useMemo(
    () => ({ search: search || undefined, category: category || undefined }),
    [search, category],
  );
  const list = useQuery(hizmetListOptions(orgId, filter));
  const [createOpen, setCreateOpen] = useState(false);
  const [priceFor, setPriceFor] = useState<ServiceCatalogItem | null>(null);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Hizmet Kataloğu"
          subtitle="Muayene, laboratuvar, görüntüleme ve diğer faturalanabilir hizmetler. Her hizmet için kurum bazlı fiyat verilebilir."
          actions={
            <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Hizmet</span>
            </PrimaryButton>
          }
        />

        <div className="flex flex-wrap gap-2">
          <TextInput
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Ara: ad veya kod"
            className="max-w-xs"
          />
          <SelectInput
            value={category}
            onChange={(e) => setCategory(e.target.value as ServiceCategory | "")}
            className="max-w-xs"
          >
            <option value="">Tüm kategoriler</option>
            {Object.entries(CATEGORY_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </div>

        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Liste yüklenemedi.</div>
        ) : (
          <DataTable<ServiceCatalogItem>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={hizmetColumns}
            onRowClick={(r) => setPriceFor(r)}
          />
        )}
      </div>

      <CreateHizmetSheet
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        orgId={orgId}
      />

      {priceFor && (
        <PriceSheet
          service={priceFor}
          orgId={orgId}
          onClose={() => setPriceFor(null)}
        />
      )}
    </DashboardLayout>
  );
}

const hizmetColumns: Column<ServiceCatalogItem>[] = [
  { key: "code", header: "Kod", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.code}</code> },
  { key: "name", header: "Hizmet", cell: (r) => <span className="font-medium">{r.name}</span> },
  { key: "category", header: "Kategori", cell: (r) => CATEGORY_LABELS[r.category] ?? r.category },
  { key: "sut", header: "SUT", cell: (r) => r.sut_code ?? "—" },
  { key: "unit", header: "Birim", cell: (r) => r.unit },
  { key: "vat", header: "KDV %", cell: (r) => `%${r.vat_rate.toFixed(0)}` },
  {
    key: "base",
    header: "Etiket Fiyat",
    cell: (r) => (r.base_price != null ? formatTl(r.base_price) : <span className="text-muted-foreground">—</span>),
    className: "text-right",
  },
];

function CreateHizmetSheet({ open, onClose, orgId }: { open: boolean; onClose: () => void; orgId: string }) {
  const qc = useQueryClient();
  const [form, setForm] = useState<CreateHizmetInput>({
    code: "",
    name: "",
    category: "consultation",
    vat_rate: 10,
    unit: "adet",
  });

  const create = useMutation({
    mutationFn: () => createHizmet(form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: hizmetKeys.all(orgId) });
      setForm({ code: "", name: "", category: "consultation", vat_rate: 10, unit: "adet" });
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Hizmet">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="h-name" label="Hizmet adı" required>
          <TextInput id="h-name" required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field id="h-code" label="Kod" required hint="Örn. MUAYENE_KARDIYOLOJI">
            <TextInput
              id="h-code"
              required
              value={form.code}
              onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, "") })}
            />
          </Field>
          <Field id="h-sut" label="SUT Kodu">
            <TextInput id="h-sut" value={form.sut_code ?? ""} onChange={(e) => setForm({ ...form, sut_code: e.target.value })} placeholder="örn. 520.011" />
          </Field>
        </div>
        <Field id="h-cat" label="Kategori" required>
          <SelectInput
            id="h-cat"
            value={form.category}
            onChange={(e) => setForm({ ...form, category: e.target.value as ServiceCategory })}
          >
            {Object.entries(CATEGORY_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <div className="grid grid-cols-3 gap-3">
          <Field id="h-unit" label="Birim">
            <TextInput id="h-unit" value={form.unit ?? "adet"} onChange={(e) => setForm({ ...form, unit: e.target.value })} />
          </Field>
          <Field id="h-vat" label="KDV %">
            <TextInput
              id="h-vat"
              type="number"
              min="0"
              max="100"
              step="0.5"
              value={String(form.vat_rate)}
              onChange={(e) => setForm({ ...form, vat_rate: Number(e.target.value) || 0 })}
            />
          </Field>
          <Field id="h-price" label="Etiket fiyat">
            <TextInput
              id="h-price"
              type="number"
              min="0"
              step="0.01"
              value={form.base_price != null ? String(form.base_price) : ""}
              onChange={(e) =>
                setForm({ ...form, base_price: e.target.value ? Number(e.target.value) : undefined })
              }
            />
          </Field>
        </div>
        <Field id="h-desc" label="Açıklama">
          <Textarea
            id="h-desc"
            rows={3}
            value={form.description ?? ""}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
          />
        </Field>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={form.requires_doctor ?? false}
            onChange={(e) => setForm({ ...form, requires_doctor: e.target.checked })}
            className="h-4 w-4 rounded border-input"
          />
          Doktor ataması gerektirir
        </label>
        {create.isError && (
          <p className="text-sm text-[var(--critical)]">Kayıt başarısız. Kod zaten kullanılıyor olabilir.</p>
        )}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !form.name || !form.code}>
            {create.isPending ? "Kaydediliyor..." : "Kaydet"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function PriceSheet({ service, orgId, onClose }: { service: ServiceCatalogItem; orgId: string; onClose: () => void }) {
  const qc = useQueryClient();
  const prices = useQuery(hizmetPricesOptions(orgId, service.id));
  const institutions = useQuery(kurumListOptions(orgId, true));
  const [form, setForm] = useState<CreateHizmetPriceInput>({ price: 0 });

  const create = useMutation({
    mutationFn: () => createHizmetPrice(service.id, form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: hizmetKeys.prices(orgId, service.id) });
      setForm({ price: 0 });
    },
  });

  return (
    <SideSheet open onClose={onClose} title={`Fiyatlar: ${service.name}`}>
      <div className="space-y-6">
        <div className="space-y-2 rounded-md border border-border p-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Kod</span>
            <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{service.code}</code>
          </div>
          {service.sut_code && (
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">SUT</span>
              <span>{service.sut_code}</span>
            </div>
          )}
          {service.base_price != null && (
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Etiket fiyat</span>
              <span className="font-medium">{formatTl(service.base_price)}</span>
            </div>
          )}
        </div>

        <div>
          <h3 className="mb-2 text-sm font-semibold">Mevcut fiyatlar</h3>
          {prices.isLoading ? (
            <p className="text-sm text-muted-foreground">Yükleniyor...</p>
          ) : (prices.data ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">Henüz fiyat tanımlanmamış.</p>
          ) : (
            <ul className="space-y-1 text-sm">
              {(prices.data ?? []).map((p) => {
                const inst = institutions.data?.find((i) => i.id === p.external_institution_id);
                return (
                  <li key={p.id} className="flex items-center justify-between rounded-md border border-border px-3 py-2">
                    <div>
                      <div className="font-medium">{inst?.name ?? "Varsayılan (cepten ödeme)"}</div>
                      <div className="text-xs text-muted-foreground">
                        {new Date(p.valid_from).toLocaleDateString("tr-TR")}
                        {p.valid_to && ` → ${new Date(p.valid_to).toLocaleDateString("tr-TR")}`}
                      </div>
                    </div>
                    <span className="font-mono text-sm">{formatTl(p.price)}</span>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        <form
          className="space-y-3 border-t border-border pt-4"
          onSubmit={(e) => { e.preventDefault(); create.mutate(); }}
        >
          <h3 className="text-sm font-semibold">Yeni fiyat ekle</h3>
          <Field id="pr-inst" label="Kurum" hint="Boş bırakılırsa varsayılan / cepten ödeme">
            <SelectInput
              id="pr-inst"
              value={form.external_institution_id ?? ""}
              onChange={(e) => setForm({ ...form, external_institution_id: e.target.value || undefined })}
            >
              <option value="">Varsayılan (cepten ödeme)</option>
              {(institutions.data ?? []).map((i) => (
                <option key={i.id} value={i.id}>{i.name}</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="pr-price" label="Fiyat (TRY)" required>
            <TextInput
              id="pr-price"
              type="number"
              min="0"
              step="0.01"
              required
              value={form.price ? String(form.price) : ""}
              onChange={(e) => setForm({ ...form, price: Number(e.target.value) || 0 })}
            />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field id="pr-from" label="Geçerli (başlangıç)">
              <TextInput
                id="pr-from"
                type="date"
                value={form.valid_from ?? ""}
                onChange={(e) => setForm({ ...form, valid_from: e.target.value })}
              />
            </Field>
            <Field id="pr-to" label="Geçerli (bitiş)">
              <TextInput
                id="pr-to"
                type="date"
                value={form.valid_to ?? ""}
                onChange={(e) => setForm({ ...form, valid_to: e.target.value })}
              />
            </Field>
          </div>
          {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız.</p>}
          <PrimaryButton type="submit" disabled={create.isPending || form.price <= 0} className="w-full">
            {create.isPending ? "Ekleniyor..." : "Fiyat ekle"}
          </PrimaryButton>
        </form>
      </div>
    </SideSheet>
  );
}
