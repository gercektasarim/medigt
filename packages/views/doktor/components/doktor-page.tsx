"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { bransListOptions } from "@medigt/core/brans";
import {
  createDoktor,
  doktorKeys,
  doktorListOptions,
  type CreateDoktorInput,
} from "@medigt/core/doktor";
import type { Doctor } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import { Field, PrimaryButton, SecondaryButton, SelectInput, TextInput } from "../../common/form-fields";

export function DoktorPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const list = useQuery(doktorListOptions(orgId));
  const [open, setOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Doktorlar"
          subtitle="Hastanede çalışan hekimler ve uzmanlık alanları."
          actions={
            <PrimaryButton type="button" onClick={() => setOpen(true)}>
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Doktor</span>
            </PrimaryButton>
          }
        />
        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Liste yüklenemedi.</div>
        ) : (
          <DataTable<Doctor>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={doktorColumns}
          />
        )}
      </div>

      <CreateDoktorSheet open={open} onClose={() => setOpen(false)} orgId={orgId} />
    </DashboardLayout>
  );
}

const doktorColumns: Column<Doctor>[] = [
  {
    key: "name",
    header: "Ad Soyad",
    cell: (r) => (
      <span className="font-medium">
        {r.staff.title ? r.staff.title + " " : ""}{r.staff.first_name} {r.staff.last_name}
      </span>
    ),
  },
  {
    key: "specs",
    header: "Branş",
    cell: (r) =>
      r.specializations.length === 0 ? (
        <span className="text-xs text-muted-foreground">—</span>
      ) : (
        <span className="text-sm">{r.specializations.map((s) => s.name).join(", ")}</span>
      ),
  },
  { key: "diploma", header: "Diploma No", cell: (r) => r.diploma_no ?? "—" },
  { key: "medula", header: "Medula Kodu", cell: (r) => r.medula_doctor_code ?? "—" },
  {
    key: "accepts",
    header: "Hasta Alıyor",
    cell: (r) =>
      r.is_accepting_patients ? (
        <span className="success-badge">Evet</span>
      ) : (
        <span className="text-xs text-muted-foreground">Hayır</span>
      ),
  },
];

function CreateDoktorSheet({ open, onClose, orgId }: { open: boolean; onClose: () => void; orgId: string }) {
  const qc = useQueryClient();
  const branchList = useQuery(bransListOptions(orgId));
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [title, setTitle] = useState("Uzm. Dr.");
  const [diplomaNo, setDiplomaNo] = useState("");
  const [medulaCode, setMedulaCode] = useState("");
  const [specId, setSpecId] = useState("");
  const [acceptsPatients, setAcceptsPatients] = useState(true);

  const create = useMutation({
    mutationFn: (): Promise<Doctor> => {
      const input: CreateDoktorInput = {
        staff: { first_name: firstName, last_name: lastName, title, employment_type: "full_time" },
        diploma_no: diplomaNo || undefined,
        medula_doctor_code: medulaCode || undefined,
        is_accepting_patients: acceptsPatients,
        specialization_ids: specId ? [specId] : undefined,
        primary_specialization_id: specId || undefined,
      };
      return createDoktor(input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: doktorKeys.all(orgId) });
      setFirstName(""); setLastName(""); setDiplomaNo(""); setMedulaCode(""); setSpecId("");
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Doktor">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <div className="grid grid-cols-2 gap-3">
          <Field id="d-first" label="Ad" required>
            <TextInput id="d-first" required value={firstName} onChange={(e) => setFirstName(e.target.value)} />
          </Field>
          <Field id="d-last" label="Soyad" required>
            <TextInput id="d-last" required value={lastName} onChange={(e) => setLastName(e.target.value)} />
          </Field>
        </div>
        <Field id="d-title" label="Unvan">
          <TextInput id="d-title" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Örn. Uzm. Dr., Op. Dr." />
        </Field>
        <Field id="d-spec" label="Birincil branş">
          <SelectInput id="d-spec" value={specId} onChange={(e) => setSpecId(e.target.value)}>
            <option value="">— Seçiniz —</option>
            {(branchList.data ?? []).map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </SelectInput>
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field id="d-diploma" label="Diploma no">
            <TextInput id="d-diploma" value={diplomaNo} onChange={(e) => setDiplomaNo(e.target.value)} />
          </Field>
          <Field id="d-medula" label="Medula doktor kodu">
            <TextInput id="d-medula" value={medulaCode} onChange={(e) => setMedulaCode(e.target.value)} />
          </Field>
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={acceptsPatients}
            onChange={(e) => setAcceptsPatients(e.target.checked)}
            className="h-4 w-4 rounded border-input"
          />
          Yeni hasta alıyor
        </label>

        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız.</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !firstName || !lastName}>
            {create.isPending ? "Kaydediliyor..." : "Kaydet"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
