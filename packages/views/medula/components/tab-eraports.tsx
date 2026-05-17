"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, FileSignature, Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { doktorListOptions } from "@medigt/core/doktor";
import {
  cancelMedulaEraport,
  createMedulaEraport,
  eraportListOptions,
  medulaKeys,
  type CreateEraportInput,
} from "@medigt/core/medula";
import type {
  MedulaEraport,
  MedulaEraportKind,
  MedulaEraportStatus,
  Patient,
} from "@medigt/core/types";
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
import { HastaSearch } from "../../randevu/components/hasta-search";
import {
  ERAPORT_KIND_LABELS,
  ERAPORT_STATUS_COLORS,
  ERAPORT_STATUS_LABELS,
} from "./labels";

export function MedulaEraportsTab() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const [statusFilter, setStatusFilter] = useState<MedulaEraportStatus | "">("");
  const list = useQuery(eraportListOptions(branchId, statusFilter || undefined));
  const [createOpen, setCreateOpen] = useState(false);
  const [cancelTarget, setCancelTarget] = useState<MedulaEraport | null>(null);

  const columns: Column<MedulaEraport>[] = [
    {
      key: "at",
      header: "Tarih",
      cell: (r) => new Date(r.requested_at).toLocaleString("tr-TR"),
    },
    {
      key: "patient",
      header: "Hasta",
      cell: (r) => (
        <div>
          <div className="font-medium">{r.patient_first_name} {r.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">MRN {r.patient_mrn}</div>
        </div>
      ),
    },
    {
      key: "kind",
      header: "Rapor Türü",
      cell: (r) => ERAPORT_KIND_LABELS[r.kind] ?? r.kind,
    },
    {
      key: "validity",
      header: "Geçerlilik",
      cell: (r) => (
        <div className="text-xs">
          <div>{r.valid_from}</div>
          {r.valid_to ? <div className="text-muted-foreground">→ {r.valid_to}</div> : <div className="text-muted-foreground">Açık</div>}
        </div>
      ),
    },
    {
      key: "codes",
      header: "Kod Sayısı",
      cell: (r) => (
        <div className="text-xs text-muted-foreground">
          {r.diagnoses_icd10.length} tanı · {r.drug_codes.length} ilaç
        </div>
      ),
    },
    {
      key: "status",
      header: "Durum",
      cell: (r) => (
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${ERAPORT_STATUS_COLORS[r.status]}`}>
          {ERAPORT_STATUS_LABELS[r.status]}
        </span>
      ),
    },
    {
      key: "no",
      header: "Rapor No",
      cell: (r) => r.eraport_no ? (
        <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.eraport_no}</code>
      ) : <span className="text-xs text-muted-foreground">—</span>,
    },
    {
      key: "actions",
      header: "",
      cell: (r) =>
        r.status === "approved" || r.status === "submitted" ? (
          <button
            type="button"
            onClick={() => setCancelTarget(r)}
            className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs text-[var(--critical)] hover:bg-muted"
          >
            <Ban className="h-3.5 w-3.5" /> İptal
          </button>
        ) : null,
      className: "text-right",
    },
  ];

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Field id="er-status" label="Durum">
          <SelectInput
            id="er-status"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as MedulaEraportStatus | "")}
            className="max-w-xs"
          >
            <option value="">Tümü</option>
            {Object.entries(ERAPORT_STATUS_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
          <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni e-Rapor</span>
        </PrimaryButton>
      </div>

      {list.isLoading ? (
        <div className="empty-state">Yükleniyor...</div>
      ) : (list.data ?? []).length === 0 ? (
        <div className="empty-state">e-Rapor kaydı yok.</div>
      ) : (
        <DataTable<MedulaEraport>
          rows={list.data ?? []}
          rowKey={(r) => r.id}
          columns={columns}
        />
      )}

      <CreateEraportSheet open={createOpen} onClose={() => setCreateOpen(false)} branchId={branchId} />
      {cancelTarget && (
        <CancelEraportSheet
          eraport={cancelTarget}
          branchId={branchId}
          onClose={() => setCancelTarget(null)}
        />
      )}
    </div>
  );
}

function CreateEraportSheet({
  open,
  onClose,
  branchId,
}: {
  open: boolean;
  onClose: () => void;
  branchId: string;
}) {
  const qc = useQueryClient();
  const org = useHospitalStore((s) => s.organization);
  const doctors = useQuery(doktorListOptions(org?.id ?? ""));

  const [patient, setPatient] = useState<Patient | null>(null);
  const [doctorId, setDoctorId] = useState("");
  const [kind, setKind] = useState<MedulaEraportKind>("chronic_drug");
  const [diagnoses, setDiagnoses] = useState("");
  const [drugs, setDrugs] = useState("");
  const [validFrom, setValidFrom] = useState("");
  const [validTo, setValidTo] = useState("");
  const [reportText, setReportText] = useState("");

  const create = useMutation({
    mutationFn: () => {
      const input: CreateEraportInput = {
        patient_id: patient!.id,
        doctor_id: doctorId || undefined,
        kind,
        diagnoses_icd10: diagnoses.split(/[,\s]+/).filter((s) => s.trim().length > 0),
        drug_codes: drugs.split(/[,\s]+/).filter((s) => s.trim().length > 0),
        valid_from: validFrom,
        valid_to: validTo || undefined,
        report_text: reportText.trim() || undefined,
      };
      return createMedulaEraport(input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: medulaKeys.all(branchId) });
      setPatient(null);
      setDoctorId("");
      setKind("chronic_drug");
      setDiagnoses("");
      setDrugs("");
      setValidFrom("");
      setValidTo("");
      setReportText("");
      onClose();
    },
  });

  const canSubmit = !!patient && !!validFrom && !!diagnoses.trim() && !create.isPending;

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni e-Rapor">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="er-patient" label="Hasta" required>
          {patient ? (
            <div className="flex items-center justify-between rounded-md border border-border bg-muted/40 px-3 py-2">
              <div>
                <div className="font-medium">{patient.first_name} {patient.last_name}</div>
                <div className="text-xs text-muted-foreground">MRN {patient.mrn}</div>
              </div>
              <button type="button" onClick={() => setPatient(null)} className="text-xs text-muted-foreground hover:underline">
                Değiştir
              </button>
            </div>
          ) : (
            <HastaSearch onPick={setPatient} />
          )}
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field id="er-doctor" label="Düzenleyen doktor">
            <SelectInput id="er-doctor" value={doctorId} onChange={(e) => setDoctorId(e.target.value)}>
              <option value="">— Atanmadı —</option>
              {(doctors.data ?? []).map((d) => (
                <option key={d.id} value={d.id}>
                  {d.staff.title ? d.staff.title + " " : ""}{d.staff.first_name} {d.staff.last_name}
                </option>
              ))}
            </SelectInput>
          </Field>
          <Field id="er-kind" label="Rapor türü">
            <SelectInput id="er-kind" value={kind} onChange={(e) => setKind(e.target.value as MedulaEraportKind)}>
              {Object.entries(ERAPORT_KIND_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
        </div>

        <Field id="er-diag" label="ICD-10 tanı kodları (virgülle ayrılmış)" required>
          <TextInput
            id="er-diag"
            required
            value={diagnoses}
            onChange={(e) => setDiagnoses(e.target.value)}
            placeholder="E11.9, I10"
          />
        </Field>
        <Field id="er-drugs" label="ATC ilaç kodları (virgülle ayrılmış)" hint="Kronik ilaç raporu için zorunlu">
          <TextInput
            id="er-drugs"
            value={drugs}
            onChange={(e) => setDrugs(e.target.value)}
            placeholder="A10BA02, C09AA02"
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field id="er-from" label="Geçerlilik başı" required>
            <TextInput id="er-from" type="date" required value={validFrom} onChange={(e) => setValidFrom(e.target.value)} />
          </Field>
          <Field id="er-to" label="Geçerlilik sonu">
            <TextInput id="er-to" type="date" value={validTo} onChange={(e) => setValidTo(e.target.value)} />
          </Field>
        </div>

        <Field id="er-text" label="Rapor metni">
          <Textarea id="er-text" rows={4} value={reportText} onChange={(e) => setReportText(e.target.value)} />
        </Field>

        {create.isError && <p className="text-sm text-[var(--critical)]">{(create.error as Error)?.message}</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            <span className="inline-flex items-center gap-1">
              <FileSignature className="h-4 w-4" /> {create.isPending ? "Gönderiliyor..." : "Raporu gönder"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function CancelEraportSheet({
  eraport,
  branchId,
  onClose,
}: {
  eraport: MedulaEraport;
  branchId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [reason, setReason] = useState("");
  const cancel = useMutation({
    mutationFn: () => cancelMedulaEraport(eraport.id, reason || undefined),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: medulaKeys.all(branchId) });
      onClose();
    },
  });
  return (
    <SideSheet open onClose={onClose} title={`e-Rapor İptal · ${eraport.eraport_no ?? eraport.id.slice(0, 8)}`}>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); cancel.mutate(); }}>
        <Field id="erc-reason" label="İptal nedeni">
          <Textarea id="erc-reason" rows={3} value={reason} onChange={(e) => setReason(e.target.value)} />
        </Field>
        {cancel.isError && <p className="text-sm text-[var(--critical)]">{(cancel.error as Error)?.message}</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">Vazgeç</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={cancel.isPending}>
            {cancel.isPending ? "Gönderiliyor..." : "İptal et"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
