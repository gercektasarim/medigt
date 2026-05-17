"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createKurum,
  kurumKeys,
  kurumListOptions,
  type CreateKurumInput,
} from "@medigt/core/kurum";
import type { ExternalInstitution, InstitutionKind } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import { Field, PrimaryButton, SecondaryButton, SelectInput, TextInput } from "../../common/form-fields";

const KIND_LABELS: Record<InstitutionKind, string> = {
  sgk: "SGK",
  private_insurance: "Özel sigorta",
  corporate: "Kurumsal anlaşma",
  foreign_insurance: "Yabancı sigorta",
  oop: "Cepten ödeme",
  other: "Diğer",
};

export function KurumPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const list = useQuery(kurumListOptions(orgId));
  const [open, setOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Anlaşmalı Kurumlar"
          subtitle="SGK, özel sağlık sigortaları ve kurumsal anlaşmalar — faturalama ve sevk için kullanılır."
          actions={
            <PrimaryButton type="button" onClick={() => setOpen(true)}>
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Kurum</span>
            </PrimaryButton>
          }
        />
        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Liste yüklenemedi.</div>
        ) : (
          <DataTable<ExternalInstitution>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={kurumColumns}
          />
        )}
      </div>

      <CreateKurumSheet open={open} onClose={() => setOpen(false)} orgId={orgId} />
    </DashboardLayout>
  );
}

const kurumColumns: Column<ExternalInstitution>[] = [
  { key: "code", header: "Kod", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.code}</code> },
  { key: "name", header: "Kurum", cell: (r) => <span className="font-medium">{r.name}</span> },
  { key: "kind", header: "Tür", cell: (r) => KIND_LABELS[r.kind] ?? r.kind },
  { key: "tax", header: "VKN", cell: (r) => r.tax_id ?? "—" },
  { key: "phone", header: "Telefon", cell: (r) => r.phone ?? "—" },
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

function CreateKurumSheet({ open, onClose, orgId }: { open: boolean; onClose: () => void; orgId: string }) {
  const qc = useQueryClient();
  const [form, setForm] = useState<CreateKurumInput>({ code: "", name: "", kind: "private_insurance" });

  const create = useMutation({
    mutationFn: () => createKurum(form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: kurumKeys.all(orgId) });
      setForm({ code: "", name: "", kind: "private_insurance" });
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Kurum">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="k-name" label="Kurum adı" required>
          <TextInput id="k-name" required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        </Field>
        <Field id="k-code" label="Kod" required hint="Büyük harf + alt çizgi. Örn. AXA_SIGORTA">
          <TextInput
            id="k-code"
            required
            value={form.code}
            onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, "") })}
          />
        </Field>
        <Field id="k-kind" label="Tür" required>
          <SelectInput
            id="k-kind"
            value={form.kind}
            onChange={(e) => setForm({ ...form, kind: e.target.value as InstitutionKind })}
          >
            {Object.entries(KIND_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field id="k-tax" label="VKN">
            <TextInput id="k-tax" value={form.tax_id ?? ""} onChange={(e) => setForm({ ...form, tax_id: e.target.value })} />
          </Field>
          <Field id="k-phone" label="Telefon">
            <TextInput id="k-phone" value={form.phone ?? ""} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
          </Field>
        </div>
        <Field id="k-email" label="E-posta">
          <TextInput id="k-email" type="email" value={form.email ?? ""} onChange={(e) => setForm({ ...form, email: e.target.value })} />
        </Field>
        <Field id="k-contract" label="Sözleşme no">
          <TextInput id="k-contract" value={form.contract_no ?? ""} onChange={(e) => setForm({ ...form, contract_no: e.target.value })} />
        </Field>

        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız.</p>}
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
