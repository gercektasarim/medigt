"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Building2, FileSearch, Pill, Search, Stethoscope, User } from "lucide-react";
import {
  branchesQueryOptions,
  doctorQueryOptions,
  drugPaymentQueryOptions,
  eraportQueryOptions,
  takipQueryOptions,
  treatmentTypesQueryOptions,
} from "@medigt/core/medula";
import { DataTable, type Column } from "../../common/data-table";
import {
  Field,
  PrimaryButton,
  TextInput,
} from "../../common/form-fields";
import type {
  MedulaCodeName,
} from "@medigt/core/types";

export function MedulaQueriesTab() {
  return (
    <div className="space-y-6">
      <TakipQueryCard />
      <EraportQueryCard />
      <DoctorQueryCard />
      <DrugPaymentQueryCard />
      <BranchesCard />
      <TreatmentTypesCard />
    </div>
  );
}

function QueryShell({
  icon,
  title,
  description,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  description?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-lg border border-border bg-card p-4">
      <header className="mb-2 flex items-center gap-2">
        <span className="text-muted-foreground">{icon}</span>
        <div>
          <h3 className="text-sm font-semibold">{title}</h3>
          {description && <p className="text-xs text-muted-foreground">{description}</p>}
        </div>
      </header>
      {children}
    </section>
  );
}

function TakipQueryCard() {
  const [draft, setDraft] = useState("");
  const [applied, setApplied] = useState("");
  const q = useQuery({ ...takipQueryOptions(applied), enabled: !!applied });
  return (
    <QueryShell
      icon={<FileSearch className="h-4 w-4" />}
      title="Takip Sorgulama"
      description="Takip numarası ile mevcut SGK takip kaydının durumunu okur."
    >
      <form
        className="flex items-end gap-2"
        onSubmit={(e) => { e.preventDefault(); setApplied(draft.trim()); }}
      >
        <Field id="q-takip" label="Takip No">
          <TextInput id="q-takip" value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="TKPxxxxxx" />
        </Field>
        <PrimaryButton type="submit" disabled={!draft.trim() || q.isFetching}>
          <span className="inline-flex items-center gap-1"><Search className="h-4 w-4" /> {q.isFetching ? "Sorgulanıyor..." : "Sorgula"}</span>
        </PrimaryButton>
      </form>
      {q.isError && <p className="mt-2 text-sm text-[var(--critical)]">{(q.error as Error)?.message}</p>}
      {q.data && (
        <pre className="mt-3 max-h-60 overflow-auto rounded-md border border-border bg-muted/40 p-3 text-xs">
          {JSON.stringify(q.data, null, 2)}
        </pre>
      )}
    </QueryShell>
  );
}

function EraportQueryCard() {
  const [draft, setDraft] = useState("");
  const [applied, setApplied] = useState("");
  const q = useQuery({ ...eraportQueryOptions(applied), enabled: !!applied });
  return (
    <QueryShell
      icon={<FileSearch className="h-4 w-4" />}
      title="e-Rapor Sorgulama"
      description="Rapor numarası ile SGK'da kayıtlı rapor detayını çeker."
    >
      <form
        className="flex items-end gap-2"
        onSubmit={(e) => { e.preventDefault(); setApplied(draft.trim()); }}
      >
        <Field id="q-eraport" label="Rapor No">
          <TextInput id="q-eraport" value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="RPRxxxxxx" />
        </Field>
        <PrimaryButton type="submit" disabled={!draft.trim() || q.isFetching}>
          <span className="inline-flex items-center gap-1"><Search className="h-4 w-4" /> {q.isFetching ? "Sorgulanıyor..." : "Sorgula"}</span>
        </PrimaryButton>
      </form>
      {q.isError && <p className="mt-2 text-sm text-[var(--critical)]">{(q.error as Error)?.message}</p>}
      {q.data && (
        <pre className="mt-3 max-h-60 overflow-auto rounded-md border border-border bg-muted/40 p-3 text-xs">
          {JSON.stringify(q.data, null, 2)}
        </pre>
      )}
    </QueryShell>
  );
}

function DoctorQueryCard() {
  const [draft, setDraft] = useState("");
  const [applied, setApplied] = useState("");
  const q = useQuery({ ...doctorQueryOptions(applied), enabled: applied.length === 11 });
  return (
    <QueryShell
      icon={<User className="h-4 w-4" />}
      title="Doktor Sorgulama"
      description="TC ile SGK kayıtlı doktor bilgisi (Medula kodu, branş)."
    >
      <form
        className="flex items-end gap-2"
        onSubmit={(e) => { e.preventDefault(); setApplied(draft.trim()); }}
      >
        <Field id="q-dr" label="TC Kimlik No">
          <TextInput
            id="q-dr"
            value={draft}
            onChange={(e) => setDraft(e.target.value.replace(/\D/g, "").slice(0, 11))}
            placeholder="11 haneli"
          />
        </Field>
        <PrimaryButton type="submit" disabled={draft.length !== 11 || q.isFetching}>
          <span className="inline-flex items-center gap-1"><Search className="h-4 w-4" /> {q.isFetching ? "Sorgulanıyor..." : "Sorgula"}</span>
        </PrimaryButton>
      </form>
      {q.isError && <p className="mt-2 text-sm text-[var(--critical)]">{(q.error as Error)?.message}</p>}
      {q.data && (
        <div className="mt-3 grid grid-cols-2 gap-2 text-sm">
          <Cell label="Tam ad" value={q.data.full_name} />
          <Cell label="Medula kodu" value={q.data.medula_doctor_code} />
          <Cell label="Branş" value={`${q.data.branch_code} · ${q.data.branch_name}`} />
          <Cell label="Aktif mi?" value={q.data.is_active ? "Evet" : "Hayır"} />
        </div>
      )}
    </QueryShell>
  );
}

function DrugPaymentQueryCard() {
  const [draft, setDraft] = useState("");
  const [applied, setApplied] = useState("");
  const q = useQuery({ ...drugPaymentQueryOptions(applied), enabled: !!applied });
  return (
    <QueryShell
      icon={<Pill className="h-4 w-4" />}
      title="İlaç Ödeme Sorgulama"
      description="Barkod ile SGK'nın ilacı geri ödemesi ve hasta katılım payı."
    >
      <form
        className="flex items-end gap-2"
        onSubmit={(e) => { e.preventDefault(); setApplied(draft.trim()); }}
      >
        <Field id="q-drug" label="Barkod">
          <TextInput
            id="q-drug"
            value={draft}
            onChange={(e) => setDraft(e.target.value.replace(/\D/g, ""))}
            placeholder="GTIN / karekod"
          />
        </Field>
        <PrimaryButton type="submit" disabled={!draft.trim() || q.isFetching}>
          <span className="inline-flex items-center gap-1"><Search className="h-4 w-4" /> {q.isFetching ? "Sorgulanıyor..." : "Sorgula"}</span>
        </PrimaryButton>
      </form>
      {q.isError && <p className="mt-2 text-sm text-[var(--critical)]">{(q.error as Error)?.message}</p>}
      {q.data && (
        <div className="mt-3 grid grid-cols-2 gap-2 text-sm">
          <Cell label="İlaç" value={q.data.drug_name} />
          <Cell label="Barkod" value={q.data.barcode} />
          <Cell label="Geri ödeme" value={q.data.is_reimbursed ? "Var" : "Yok"} />
          <Cell label="Hasta katılım %" value={`%${q.data.patient_share_pct.toFixed(1)}`} />
        </div>
      )}
    </QueryShell>
  );
}

function BranchesCard() {
  const q = useQuery(branchesQueryOptions());
  const columns: Column<MedulaCodeName>[] = [
    { key: "code", header: "Kod", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.code}</code> },
    { key: "name", header: "Branş", cell: (r) => r.name },
  ];
  return (
    <QueryShell
      icon={<Building2 className="h-4 w-4" />}
      title="SGK Branş Kodları"
      description="SGK'nın tanımlı branş listesi (24 saat önbelleklenir)."
    >
      {q.isLoading ? (
        <p className="text-sm text-muted-foreground">Yükleniyor...</p>
      ) : (
        <DataTable<MedulaCodeName> rows={q.data ?? []} rowKey={(r) => r.code} columns={columns} />
      )}
    </QueryShell>
  );
}

function TreatmentTypesCard() {
  const q = useQuery(treatmentTypesQueryOptions());
  const columns: Column<MedulaCodeName>[] = [
    { key: "code", header: "Kod", cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.code}</code> },
    { key: "name", header: "Tedavi Türü", cell: (r) => r.name },
  ];
  return (
    <QueryShell
      icon={<Stethoscope className="h-4 w-4" />}
      title="SGK Tedavi Türleri"
      description="Provizyon ve fatura gönderiminde kullanılan tedavi türü kodları."
    >
      {q.isLoading ? (
        <p className="text-sm text-muted-foreground">Yükleniyor...</p>
      ) : (
        <DataTable<MedulaCodeName> rows={q.data ?? []} rowKey={(r) => r.code} columns={columns} />
      )}
    </QueryShell>
  );
}

function Cell({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="font-medium">{value}</div>
    </div>
  );
}
