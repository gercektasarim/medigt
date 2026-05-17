"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRightLeft, Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { doktorListOptions } from "@medigt/core/doktor";
import {
  branchesQueryOptions,
  createMedulaReferral,
  medulaKeys,
  referralListOptions,
  type CreateReferralInput,
} from "@medigt/core/medula";
import type {
  MedulaReferral,
  MedulaReferralStatus,
  MedulaReferralType,
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
  REFERRAL_STATUS_COLORS,
  REFERRAL_STATUS_LABELS,
  REFERRAL_TYPE_LABELS,
} from "./labels";

export function MedulaReferralsTab() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const [statusFilter, setStatusFilter] = useState<MedulaReferralStatus | "">("");
  const list = useQuery(referralListOptions(branchId, statusFilter || undefined));
  const [createOpen, setCreateOpen] = useState(false);

  const columns: Column<MedulaReferral>[] = [
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
      key: "target",
      header: "Hedef",
      cell: (r) => (
        <div>
          <div className="font-medium">{r.target_provider_name ?? r.target_provider_code}</div>
          <div className="text-xs text-muted-foreground">
            <code className="rounded bg-muted px-1">{r.target_provider_code}</code>
            {r.target_branch_code && ` · Branş ${r.target_branch_code}`}
          </div>
        </div>
      ),
    },
    { key: "type", header: "Tür", cell: (r) => REFERRAL_TYPE_LABELS[r.referral_type] ?? r.referral_type },
    {
      key: "status",
      header: "Durum",
      cell: (r) => (
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${REFERRAL_STATUS_COLORS[r.status]}`}>
          {REFERRAL_STATUS_LABELS[r.status]}
        </span>
      ),
    },
    {
      key: "sevk",
      header: "Sevk No",
      cell: (r) => r.sevk_no ? (
        <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.sevk_no}</code>
      ) : <span className="text-xs text-muted-foreground">—</span>,
    },
    {
      key: "reason",
      header: "Gerekçe",
      cell: (r) => <span className="text-xs text-muted-foreground">{r.reason}</span>,
    },
  ];

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Field id="ref-status" label="Durum">
          <SelectInput
            id="ref-status"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as MedulaReferralStatus | "")}
            className="max-w-xs"
          >
            <option value="">Tümü</option>
            {Object.entries(REFERRAL_STATUS_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
          <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Sevk</span>
        </PrimaryButton>
      </div>

      {list.isLoading ? (
        <div className="empty-state">Yükleniyor...</div>
      ) : (list.data ?? []).length === 0 ? (
        <div className="empty-state">Sevk kaydı yok.</div>
      ) : (
        <DataTable<MedulaReferral>
          rows={list.data ?? []}
          rowKey={(r) => r.id}
          columns={columns}
        />
      )}

      <CreateReferralSheet open={createOpen} onClose={() => setCreateOpen(false)} branchId={branchId} />
    </div>
  );
}

function CreateReferralSheet({
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
  const branches = useQuery(branchesQueryOptions());

  const [patient, setPatient] = useState<Patient | null>(null);
  const [doctorId, setDoctorId] = useState("");
  const [targetCode, setTargetCode] = useState("");
  const [targetName, setTargetName] = useState("");
  const [branchCode, setBranchCode] = useState("");
  const [reason, setReason] = useState("");
  const [diagnosis, setDiagnosis] = useState("");
  const [refType, setRefType] = useState<MedulaReferralType>("normal");

  const create = useMutation({
    mutationFn: () => {
      const input: CreateReferralInput = {
        patient_id: patient!.id,
        referring_doctor_id: doctorId || undefined,
        target_provider_code: targetCode.trim(),
        target_provider_name: targetName.trim() || undefined,
        target_branch_code: branchCode || undefined,
        reason: reason.trim(),
        diagnosis_icd10: diagnosis.trim() || undefined,
        referral_type: refType,
      };
      return createMedulaReferral(input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: medulaKeys.all(branchId) });
      setPatient(null);
      setDoctorId("");
      setTargetCode("");
      setTargetName("");
      setBranchCode("");
      setReason("");
      setDiagnosis("");
      onClose();
    },
  });

  const canSubmit = !!patient && !!targetCode.trim() && !!reason.trim() && !create.isPending;

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Sevk">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="rf-patient" label="Hasta" required>
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

        <Field id="rf-doctor" label="Sevk eden doktor">
          <SelectInput id="rf-doctor" value={doctorId} onChange={(e) => setDoctorId(e.target.value)}>
            <option value="">— Atanmadı —</option>
            {(doctors.data ?? []).map((d) => (
              <option key={d.id} value={d.id}>
                {d.staff.title ? d.staff.title + " " : ""}{d.staff.first_name} {d.staff.last_name}
              </option>
            ))}
          </SelectInput>
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field id="rf-target-code" label="Hedef tesis kodu" required>
            <TextInput id="rf-target-code" required value={targetCode} onChange={(e) => setTargetCode(e.target.value)} placeholder="SGK kodu" />
          </Field>
          <Field id="rf-target-name" label="Hedef tesis adı">
            <TextInput id="rf-target-name" value={targetName} onChange={(e) => setTargetName(e.target.value)} />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field id="rf-branch" label="Branş">
            <SelectInput id="rf-branch" value={branchCode} onChange={(e) => setBranchCode(e.target.value)}>
              <option value="">— Seçiniz —</option>
              {(branches.data ?? []).map((b) => (
                <option key={b.code} value={b.code}>{b.code} · {b.name}</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="rf-type" label="Tür">
            <SelectInput id="rf-type" value={refType} onChange={(e) => setRefType(e.target.value as MedulaReferralType)}>
              {Object.entries(REFERRAL_TYPE_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
        </div>

        <Field id="rf-diag" label="Tanı (ICD-10)">
          <TextInput id="rf-diag" value={diagnosis} onChange={(e) => setDiagnosis(e.target.value)} placeholder="E11.9" />
        </Field>
        <Field id="rf-reason" label="Gerekçe" required>
          <Textarea id="rf-reason" rows={3} required value={reason} onChange={(e) => setReason(e.target.value)} />
        </Field>

        {create.isError && <p className="text-sm text-[var(--critical)]">{(create.error as Error)?.message}</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            <span className="inline-flex items-center gap-1">
              <ArrowRightLeft className="h-4 w-4" /> {create.isPending ? "Gönderiliyor..." : "Sevki gönder"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
