"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { bransKeys, bransListOptions, createBrans, type CreateBransInput } from "@medigt/core/brans";
import type { Specialization } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import { Field, PrimaryButton, SecondaryButton, TextInput } from "../../common/form-fields";

export function BransPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const list = useQuery(bransListOptions(orgId));
  const [open, setOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Branşlar"
          subtitle="Hastane bünyesinde aktif tıbbi uzmanlık alanları. Sistem kataloğu + size özel branşlar."
          actions={
            <PrimaryButton onClick={() => setOpen(true)} type="button">
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Branş</span>
            </PrimaryButton>
          }
        />
        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Branşlar yüklenemedi.</div>
        ) : (
          <DataTable<Specialization>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={branscolumns}
          />
        )}
      </div>

      <CreateBransSheet open={open} onClose={() => setOpen(false)} orgId={orgId} />
    </DashboardLayout>
  );
}

const branscolumns: Column<Specialization>[] = [
  { key: "code", header: "Kod", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.code}</code> },
  { key: "name", header: "Ad", cell: (r) => <span className="font-medium">{r.name}</span> },
  {
    key: "source",
    header: "Kaynak",
    cell: (r) =>
      r.is_system ? (
        <span className="text-xs text-muted-foreground">Sistem</span>
      ) : (
        <span className="text-xs text-[var(--brand)]">Özel</span>
      ),
  },
];

function CreateBransSheet({ open, onClose, orgId }: { open: boolean; onClose: () => void; orgId: string }) {
  const qc = useQueryClient();
  const [form, setForm] = useState<CreateBransInput>({ code: "", name: "" });

  const create = useMutation({
    mutationFn: () => createBrans(form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: bransKeys.list(orgId) });
      setForm({ code: "", name: "" });
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Branş">
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <Field id="brans-name" label="Branş adı" required>
          <TextInput
            id="brans-name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
          />
        </Field>
        <Field id="brans-code" label="Kod" required hint="Büyük harf + alt çizgi. Örn. PSIKOLOJI">
          <TextInput
            id="brans-code"
            required
            value={form.code}
            onChange={(e) =>
              setForm({ ...form, code: e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, "") })
            }
          />
        </Field>
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
