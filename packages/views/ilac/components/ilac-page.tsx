"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pill, Plus, Snowflake } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createMedication,
  type CreateMedicationInput,
  ilacKeys,
  medicationListOptions,
} from "@medigt/core/ilac";
import { formatTl } from "@medigt/core/utils";
import type {
  Medication,
  MedicationForm,
  PrescriptionClass,
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
import {
  MEDICATION_FORM_LABELS,
  PRESCRIPTION_CLASS_COLORS,
  PRESCRIPTION_CLASS_LABELS,
} from "./labels";

export function IlacPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const [search, setSearch] = useState("");
  const [form, setForm] = useState<MedicationForm | "">("");
  const [klass, setKlass] = useState<PrescriptionClass | "">("");

  const filter = useMemo(
    () => ({
      q: search || undefined,
      form: form || undefined,
      prescriptionClass: klass || undefined,
      activeOnly: true,
    }),
    [search, form, klass],
  );
  const list = useQuery(medicationListOptions(orgId, filter));
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="İlaç Kataloğu"
          subtitle="Hastanede kullanılan ilaçlar. ATC kodu, form, doz, reçete sınıfı ve fiyat bilgisi."
          actions={
            <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni İlaç</span>
            </PrimaryButton>
          }
        />

        <div className="flex flex-wrap gap-2">
          <TextInput
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Ara: ad, etken madde, ATC, barkod"
            className="max-w-md"
          />
          <SelectInput
            value={form}
            onChange={(e) => setForm(e.target.value as MedicationForm | "")}
            className="max-w-xs"
          >
            <option value="">Tüm formlar</option>
            {Object.entries(MEDICATION_FORM_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
          <SelectInput
            value={klass}
            onChange={(e) => setKlass(e.target.value as PrescriptionClass | "")}
            className="max-w-xs"
          >
            <option value="">Tüm reçete sınıfları</option>
            {Object.entries(PRESCRIPTION_CLASS_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </div>

        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Liste yüklenemedi.</div>
        ) : (list.data ?? []).length === 0 ? (
          <div className="empty-state">İlaç bulunamadı.</div>
        ) : (
          <DataTable<Medication>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={columns}
          />
        )}
      </div>

      <CreateMedicationSheet open={createOpen} onClose={() => setCreateOpen(false)} orgId={orgId} />
    </DashboardLayout>
  );
}

const columns: Column<Medication>[] = [
  {
    key: "name",
    header: "İlaç",
    cell: (r) => (
      <div>
        <div className="font-medium">{r.name}</div>
        {r.generic_name && (
          <div className="text-xs text-muted-foreground">{r.generic_name}</div>
        )}
      </div>
    ),
  },
  {
    key: "form",
    header: "Form / Doz",
    cell: (r) => (
      <div>
        <div className="text-sm">{MEDICATION_FORM_LABELS[r.form] ?? r.form}</div>
        <div className="text-xs text-muted-foreground">
          {r.strength ?? ""}
          {r.strength && r.pack_size && " · "}
          {r.pack_size ?? ""}
        </div>
      </div>
    ),
  },
  {
    key: "atc",
    header: "ATC / Barkod",
    cell: (r) => (
      <div className="text-xs">
        {r.atc_code && <code className="rounded bg-muted px-1 py-0.5">{r.atc_code}</code>}
        {r.atc_code && r.barcode && <span> · </span>}
        {r.barcode && <span className="font-mono text-muted-foreground">{r.barcode}</span>}
      </div>
    ),
  },
  {
    key: "klass",
    header: "Reçete",
    cell: (r) => (
      <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${PRESCRIPTION_CLASS_COLORS[r.prescription_class]}`}>
        {PRESCRIPTION_CLASS_LABELS[r.prescription_class]}
      </span>
    ),
  },
  {
    key: "flags",
    header: "",
    cell: (r) => (
      <div className="flex gap-1 text-xs text-muted-foreground">
        {r.requires_cold_chain && (
          <span className="inline-flex items-center gap-0.5" title="Soğuk zincir">
            <Snowflake className="h-3 w-3" />
          </span>
        )}
        {r.is_controlled && (
          <span title="Kontrole tabi" className="rounded bg-rose-100 px-1 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200">K</span>
        )}
      </div>
    ),
  },
  {
    key: "price",
    header: "Liste",
    cell: (r) =>
      r.list_price != null ? formatTl(r.list_price) : <span className="text-muted-foreground">—</span>,
    className: "text-right",
  },
];

function CreateMedicationSheet({
  open,
  onClose,
  orgId,
}: {
  open: boolean;
  onClose: () => void;
  orgId: string;
}) {
  const qc = useQueryClient();
  const [form, setForm] = useState<CreateMedicationInput>({
    name: "",
    form: "tablet",
    prescription_class: "normal",
  });

  const create = useMutation({
    mutationFn: () => createMedication(form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ilacKeys.all(orgId) });
      setForm({ name: "", form: "tablet", prescription_class: "normal" });
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni İlaç">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="m-name" label="Ticari ad" required>
          <TextInput
            id="m-name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder="Parol 500 mg tablet"
          />
        </Field>

        <Field id="m-generic" label="Etken madde">
          <TextInput
            id="m-generic"
            value={form.generic_name ?? ""}
            onChange={(e) => setForm({ ...form, generic_name: e.target.value })}
            placeholder="Parasetamol"
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field id="m-atc" label="ATC kodu">
            <TextInput
              id="m-atc"
              value={form.atc_code ?? ""}
              onChange={(e) => setForm({ ...form, atc_code: e.target.value.toUpperCase() })}
              placeholder="N02BE01"
            />
          </Field>
          <Field id="m-barcode" label="Barkod">
            <TextInput
              id="m-barcode"
              value={form.barcode ?? ""}
              onChange={(e) => setForm({ ...form, barcode: e.target.value })}
              placeholder="8699508010593"
            />
          </Field>
        </div>

        <div className="grid grid-cols-3 gap-3">
          <Field id="m-form" label="Form">
            <SelectInput
              id="m-form"
              value={form.form}
              onChange={(e) => setForm({ ...form, form: e.target.value as MedicationForm })}
            >
              {Object.entries(MEDICATION_FORM_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="m-strength" label="Doz">
            <TextInput
              id="m-strength"
              value={form.strength ?? ""}
              onChange={(e) => setForm({ ...form, strength: e.target.value })}
              placeholder="500 mg"
            />
          </Field>
          <Field id="m-pack" label="Ambalaj">
            <TextInput
              id="m-pack"
              value={form.pack_size ?? ""}
              onChange={(e) => setForm({ ...form, pack_size: e.target.value })}
              placeholder="20 tablet"
            />
          </Field>
        </div>

        <Field id="m-class" label="Reçete sınıfı">
          <SelectInput
            id="m-class"
            value={form.prescription_class}
            onChange={(e) => setForm({ ...form, prescription_class: e.target.value as PrescriptionClass })}
          >
            {Object.entries(PRESCRIPTION_CLASS_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field id="m-manuf" label="Üretici">
            <TextInput
              id="m-manuf"
              value={form.manufacturer ?? ""}
              onChange={(e) => setForm({ ...form, manufacturer: e.target.value })}
            />
          </Field>
          <Field id="m-price" label="Liste fiyatı (TRY)">
            <TextInput
              id="m-price"
              type="number"
              min="0"
              step="0.01"
              value={form.list_price != null ? String(form.list_price) : ""}
              onChange={(e) =>
                setForm({ ...form, list_price: e.target.value ? Number(e.target.value) : undefined })
              }
            />
          </Field>
        </div>

        <div className="flex flex-wrap gap-4">
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={form.requires_cold_chain ?? false}
              onChange={(e) => setForm({ ...form, requires_cold_chain: e.target.checked })}
              className="h-4 w-4 rounded border-input"
            />
            Soğuk zincir gerektirir
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={form.is_controlled ?? false}
              onChange={(e) => setForm({ ...form, is_controlled: e.target.checked })}
              className="h-4 w-4 rounded border-input"
            />
            Kontrole tabi
          </label>
        </div>

        <Field id="m-notes" label="Notlar">
          <Textarea
            id="m-notes"
            rows={3}
            value={form.notes ?? ""}
            onChange={(e) => setForm({ ...form, notes: e.target.value })}
          />
        </Field>

        {create.isError && (
          <p className="text-sm text-[var(--critical)]">
            Kayıt başarısız. Aynı ada sahip ilaç zaten kayıtlı olabilir.
          </p>
        )}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !form.name}>
            <span className="inline-flex items-center gap-1">
              <Pill className="h-4 w-4" /> {create.isPending ? "Kaydediliyor..." : "Kaydet"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
