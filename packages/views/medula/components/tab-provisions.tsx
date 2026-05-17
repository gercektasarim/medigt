"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, CheckCircle2, Clock3, LockKeyhole, Plus, XCircle } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { kurumListOptions } from "@medigt/core/kurum";
import {
  cancelMedulaProvision,
  closeMedulaTakip,
  createMedulaProvision,
  medulaKeys,
  medulaProvisionListOptions,
  type CreateProvisionInput,
} from "@medigt/core/medula";
import type {
  MedulaProvision,
  MedulaProvisionStatus,
  MedulaProvisionType,
  Patient,
} from "@medigt/core/types";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  TextInput,
  Textarea,
} from "../../common/form-fields";
import { HastaSearch } from "../../randevu/components/hasta-search";
import {
  PROVISION_STATUS_COLORS,
  PROVISION_STATUS_LABELS,
  PROVISION_TYPE_LABELS,
} from "./labels";

export function MedulaProvisionsTab() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const [statusFilter, setStatusFilter] = useState<MedulaProvisionStatus | "">("");
  const list = useQuery(medulaProvisionListOptions(branchId, statusFilter || undefined));
  const [createOpen, setCreateOpen] = useState(false);
  const [cancelTarget, setCancelTarget] = useState<MedulaProvision | null>(null);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Field id="prov-status" label="Durum">
          <SelectInput
            id="prov-status"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as MedulaProvisionStatus | "")}
            className="max-w-xs"
          >
            <option value="">Tümü</option>
            {Object.entries(PROVISION_STATUS_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
          <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Provizyon</span>
        </PrimaryButton>
      </div>

      {list.isLoading ? (
        <div className="empty-state">Yükleniyor...</div>
      ) : (list.data ?? []).length === 0 ? (
        <div className="empty-state">Provizyon kaydı yok.</div>
      ) : (
        <DataTable<MedulaProvision>
          rows={list.data ?? []}
          rowKey={(r) => r.id}
          columns={columns(branchId, setCancelTarget)}
        />
      )}

      <CreateProvisionSheet open={createOpen} onClose={() => setCreateOpen(false)} branchId={branchId} />
      {cancelTarget && (
        <CancelProvisionSheet
          provision={cancelTarget}
          branchId={branchId}
          onClose={() => setCancelTarget(null)}
        />
      )}
    </div>
  );
}

function columns(
  branchId: string,
  onCancel: (p: MedulaProvision) => void,
): Column<MedulaProvision>[] {
  return [
    {
      key: "at",
      header: "Tarih",
      cell: (r) => (
        <div className="text-xs">
          <div>{new Date(r.requested_at).toLocaleDateString("tr-TR")}</div>
          <div className="text-muted-foreground">
            {new Date(r.requested_at).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" })}
          </div>
        </div>
      ),
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
      key: "institution",
      header: "Kurum",
      cell: (r) => r.institution_name ?? <span className="text-xs text-muted-foreground">—</span>,
    },
    { key: "type", header: "Tür", cell: (r) => PROVISION_TYPE_LABELS[r.provision_type] ?? r.provision_type },
    {
      key: "status",
      header: "Durum",
      cell: (r) => (
        <span className={`inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium ${PROVISION_STATUS_COLORS[r.status]}`}>
          {r.status === "pending" || r.status === "in_progress" ? <Clock3 className="h-3 w-3" /> :
            r.status === "completed" ? <CheckCircle2 className="h-3 w-3" /> :
            <XCircle className="h-3 w-3" />}
          {PROVISION_STATUS_LABELS[r.status]}
        </span>
      ),
    },
    {
      key: "takip",
      header: "Takip No",
      cell: (r) =>
        r.takip_no ? (
          <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.takip_no}</code>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
    {
      key: "actions",
      header: "",
      cell: (r) => <ProvisionRowActions provision={r} branchId={branchId} onCancel={onCancel} />,
      className: "text-right",
    },
  ];
}

function ProvisionRowActions({
  provision,
  branchId,
  onCancel,
}: {
  provision: MedulaProvision;
  branchId: string;
  onCancel: (p: MedulaProvision) => void;
}) {
  const qc = useQueryClient();
  const closeTakip = useMutation({
    mutationFn: () => closeMedulaTakip(provision.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: medulaKeys.all(branchId) }),
  });
  if (provision.status !== "completed") return null;
  return (
    <div className="flex justify-end gap-1">
      <button
        type="button"
        onClick={() => closeTakip.mutate()}
        disabled={closeTakip.isPending}
        className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted disabled:opacity-50"
        title="SGK takip kapat"
      >
        <LockKeyhole className="h-3.5 w-3.5" /> Kapat
      </button>
      <button
        type="button"
        onClick={() => onCancel(provision)}
        className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs text-[var(--critical)] hover:bg-muted"
        title="Provizyonu iptal et"
      >
        <Ban className="h-3.5 w-3.5" /> İptal
      </button>
    </div>
  );
}

// ---------- Create drawer ----------

function CreateProvisionSheet({
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
  const institutions = useQuery(kurumListOptions(org?.id ?? "", true));

  const [patient, setPatient] = useState<Patient | null>(null);
  const [institutionId, setInstitutionId] = useState("");
  const [provType, setProvType] = useState<MedulaProvisionType>("normal");
  const [branchCode, setBranchCode] = useState("");

  const create = useMutation({
    mutationFn: () => {
      const input: CreateProvisionInput = {
        patient_id: patient!.id,
        institution_id: institutionId || undefined,
        provision_type: provType,
        branch_code: branchCode.trim() || undefined,
      };
      return createMedulaProvision(input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: medulaKeys.all(branchId) });
      setPatient(null);
      setInstitutionId("");
      setProvType("normal");
      setBranchCode("");
      onClose();
    },
  });

  const canSubmit = !!patient && !create.isPending;

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Provizyon">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="mp-patient" label="Hasta" required>
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
        <Field id="mp-inst" label="Kurum">
          <SelectInput id="mp-inst" value={institutionId} onChange={(e) => setInstitutionId(e.target.value)}>
            <option value="">— Seçiniz —</option>
            {(institutions.data ?? []).map((i) => (
              <option key={i.id} value={i.id}>{i.name}</option>
            ))}
          </SelectInput>
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field id="mp-type" label="Tür">
            <SelectInput id="mp-type" value={provType} onChange={(e) => setProvType(e.target.value as MedulaProvisionType)}>
              {Object.entries(PROVISION_TYPE_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="mp-branch" label="Branş Kodu">
            <TextInput id="mp-branch" value={branchCode} onChange={(e) => setBranchCode(e.target.value)} placeholder="SGK kodu" />
          </Field>
        </div>
        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız: {(create.error as Error)?.message}</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            {create.isPending ? "Gönderiliyor..." : "Kuyruğa gönder"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

// ---------- Cancel drawer ----------

function CancelProvisionSheet({
  provision,
  branchId,
  onClose,
}: {
  provision: MedulaProvision;
  branchId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [reason, setReason] = useState("");
  const cancel = useMutation({
    mutationFn: () => cancelMedulaProvision(provision.id, reason || undefined),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: medulaKeys.all(branchId) });
      onClose();
    },
  });
  return (
    <SideSheet open onClose={onClose} title={`Provizyon İptali · ${provision.takip_no ?? provision.id.slice(0, 8)}`}>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); cancel.mutate(); }}>
        <p className="text-sm text-muted-foreground">
          İptal isteği outbox'a yazılır. Worker SGK'ya gönderir; sonuç tabloya düşene kadar
          provizyon "Tamamlandı" durumunda görünmeye devam edebilir.
        </p>
        <Field id="cancel-reason" label="İptal nedeni">
          <Textarea id="cancel-reason" rows={3} value={reason} onChange={(e) => setReason(e.target.value)} />
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
