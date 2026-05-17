"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createPersonel,
  personelKeys,
  personelListOptions,
  type CreatePersonelInput,
} from "@medigt/core/personel";
import type { EmploymentType, StaffMember } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import { Field, PrimaryButton, SecondaryButton, SelectInput, TextInput } from "../../common/form-fields";

const EMPLOYMENT_LABELS: Record<EmploymentType, string> = {
  full_time: "Tam zamanlı",
  part_time: "Yarı zamanlı",
  contract: "Sözleşmeli",
  consultant: "Danışman",
  intern: "Stajyer",
};

export function PersonelPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const list = useQuery(personelListOptions(orgId));
  const [open, setOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Personel"
          subtitle="Hastanenin tüm çalışanları — doktor, hemşire, idari personel."
          actions={
            <PrimaryButton type="button" onClick={() => setOpen(true)}>
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Personel</span>
            </PrimaryButton>
          }
        />
        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Liste yüklenemedi.</div>
        ) : (
          <DataTable<StaffMember>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={personelColumns}
          />
        )}
      </div>

      <CreatePersonelSheet open={open} onClose={() => setOpen(false)} orgId={orgId} />
    </DashboardLayout>
  );
}

const personelColumns: Column<StaffMember>[] = [
  { key: "emp", header: "Sicil", cell: (r) => <span className="text-xs text-muted-foreground">{r.employee_no ?? "—"}</span> },
  {
    key: "name",
    header: "Ad Soyad",
    cell: (r) => (
      <span className="font-medium">
        {r.title ? r.title + " " : ""}{r.first_name} {r.last_name}
      </span>
    ),
  },
  { key: "employment", header: "Çalışma türü", cell: (r) => EMPLOYMENT_LABELS[r.employment_type] ?? r.employment_type },
  { key: "phone", header: "Telefon", cell: (r) => r.phone ?? "—" },
  { key: "email", header: "E-posta", cell: (r) => r.email ?? "—" },
  {
    key: "status",
    header: "Durum",
    cell: (r) =>
      r.is_active ? (
        <span className="success-badge">Aktif</span>
      ) : (
        <span className="text-xs text-muted-foreground">Pasif</span>
      ),
  },
];

function CreatePersonelSheet({ open, onClose, orgId }: { open: boolean; onClose: () => void; orgId: string }) {
  const qc = useQueryClient();
  const [form, setForm] = useState<CreatePersonelInput>({
    first_name: "",
    last_name: "",
    employment_type: "full_time",
  });

  const create = useMutation({
    mutationFn: () => createPersonel(form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: personelKeys.all(orgId) });
      setForm({ first_name: "", last_name: "", employment_type: "full_time" });
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Personel">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <div className="grid grid-cols-2 gap-3">
          <Field id="p-first" label="Ad" required>
            <TextInput id="p-first" required value={form.first_name} onChange={(e) => setForm({ ...form, first_name: e.target.value })} />
          </Field>
          <Field id="p-last" label="Soyad" required>
            <TextInput id="p-last" required value={form.last_name} onChange={(e) => setForm({ ...form, last_name: e.target.value })} />
          </Field>
        </div>
        <Field id="p-title" label="Unvan" hint="Örn. Uzm. Dr., Hemşire, Sekreter">
          <TextInput id="p-title" value={form.title ?? ""} onChange={(e) => setForm({ ...form, title: e.target.value })} />
        </Field>
        <Field id="p-emp" label="Sicil no">
          <TextInput id="p-emp" value={form.employee_no ?? ""} onChange={(e) => setForm({ ...form, employee_no: e.target.value })} />
        </Field>
        <Field id="p-type" label="Çalışma türü" required>
          <SelectInput
            id="p-type"
            value={form.employment_type}
            onChange={(e) => setForm({ ...form, employment_type: e.target.value as EmploymentType })}
          >
            {Object.entries(EMPLOYMENT_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field id="p-phone" label="Telefon">
            <TextInput id="p-phone" value={form.phone ?? ""} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
          </Field>
          <Field id="p-email" label="E-posta">
            <TextInput id="p-email" type="email" value={form.email ?? ""} onChange={(e) => setForm({ ...form, email: e.target.value })} />
          </Field>
        </div>
        <Field id="p-hire" label="İşe başlama" hint="YYYY-MM-DD">
          <TextInput id="p-hire" type="date" value={form.hire_date ?? ""} onChange={(e) => setForm({ ...form, hire_date: e.target.value })} />
        </Field>

        {create.isError && (
          <p className="text-sm text-[var(--critical)]">Kayıt başarısız. Sicil no zaten kayıtlı olabilir.</p>
        )}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !form.first_name || !form.last_name}>
            {create.isPending ? "Kaydediliyor..." : "Kaydet"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
