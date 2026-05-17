"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { BedDouble, Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  admissionListOptions,
  admit,
  bedMapOptions,
  wardListOptions,
  yatisKeys,
  type AdmitInput,
} from "@medigt/core/yatis";
import { doktorListOptions } from "@medigt/core/doktor";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import type {
  Admission,
  AdmissionKind,
  AdmissionStatus,
  Patient,
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
import { HastaSearch } from "../../randevu/components/hasta-search";

const KIND_LABELS: Record<AdmissionKind, string> = {
  planned: "Planlı",
  emergency: "Acil",
  transfer_in: "Transfer",
  newborn: "Yenidoğan",
};

const STATUS_LABELS: Record<AdmissionStatus, string> = {
  active: "Yatıyor",
  discharged: "Taburcu",
};

const STATUS_COLORS: Record<AdmissionStatus, string> = {
  active: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  discharged: "bg-zinc-100 text-zinc-800 dark:bg-zinc-900 dark:text-zinc-200",
};

export function YatisPage() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const [showDischarged, setShowDischarged] = useState(false);
  const list = useQuery(
    admissionListOptions(branchId, {
      status: showDischarged ? undefined : "active",
    }),
  );
  const [admitOpen, setAdmitOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Yatış"
          subtitle="Yatan hastalar — aktif yatışlar, transfer, taburcu işlemleri."
          actions={
            <PrimaryButton type="button" onClick={() => setAdmitOpen(true)}>
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Yatış</span>
            </PrimaryButton>
          }
        />

        <label className="inline-flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={showDischarged}
            onChange={(e) => setShowDischarged(e.target.checked)}
            className="h-4 w-4 rounded border-input"
          />
          Taburcu olanları da göster
        </label>

        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Liste yüklenemedi.</div>
        ) : (list.data ?? []).length === 0 ? (
          <div className="empty-state">Yatan hasta yok.</div>
        ) : (
          <DataTable<Admission>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={columns()}
          />
        )}
      </div>

      <AdmitSheet open={admitOpen} onClose={() => setAdmitOpen(false)} branchId={branchId} />
    </DashboardLayout>
  );
}

function columns(): Column<Admission>[] {
  return [
    {
      key: "no",
      header: "Yatış No",
      cell: (a) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{a.admission_no}</code>,
    },
    {
      key: "patient",
      header: "Hasta",
      cell: (a) => (
        <div>
          <div className="font-medium">{a.patient_first_name} {a.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">MRN {a.patient_mrn}</div>
        </div>
      ),
    },
    {
      key: "ward",
      header: "Servis · Yatak",
      cell: (a) => (
        <div>
          <div className="font-medium">{a.ward_name}</div>
          <div className="text-xs text-muted-foreground">
            {a.ward_code}
            {a.bed_code ? ` · Yatak ${a.bed_code}` : " · yatak atanmadı"}
          </div>
        </div>
      ),
    },
    {
      key: "doctor",
      header: "Yatış Hekimi",
      cell: (a) =>
        a.doctor_first_name ? (
          <span>
            {a.doctor_title ? a.doctor_title + " " : ""}
            {a.doctor_first_name} {a.doctor_last_name}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
    {
      key: "kind",
      header: "Tür",
      cell: (a) => KIND_LABELS[a.kind] ?? a.kind,
    },
    {
      key: "admitted",
      header: "Yatış",
      cell: (a) => (
        <span className="text-xs text-muted-foreground">
          {new Date(a.admitted_at).toLocaleString("tr-TR", {
            day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit",
          })}
        </span>
      ),
    },
    {
      key: "status",
      header: "Durum",
      cell: (a) => (
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${STATUS_COLORS[a.status]}`}>
          {STATUS_LABELS[a.status]}
        </span>
      ),
    },
    {
      key: "open",
      header: "",
      cell: (a) => <OpenLink admissionId={a.id} />,
      className: "text-right",
    },
  ];
}

function OpenLink({ admissionId }: { admissionId: string }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const nav = useNavigation();
  return (
    <button
      type="button"
      onClick={() =>
        nav.push(paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "").yatis.detail(admissionId))
      }
      className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
    >
      <BedDouble className="h-3.5 w-3.5" /> Aç
    </button>
  );
}

// ---------- Admit drawer ----------

function AdmitSheet({ open, onClose, branchId }: { open: boolean; onClose: () => void; branchId: string }) {
  const qc = useQueryClient();
  const org = useHospitalStore((s) => s.organization);
  const wards = useQuery(wardListOptions(branchId));
  const bedMap = useQuery(bedMapOptions(branchId));
  const doctors = useQuery(doktorListOptions(org?.id ?? ""));

  const [patient, setPatient] = useState<Patient | null>(null);
  const [wardId, setWardId] = useState("");
  const [bedId, setBedId] = useState("");
  const [doctorId, setDoctorId] = useState("");
  const [kind, setKind] = useState<AdmissionKind>("planned");
  const [complaint, setComplaint] = useState("");
  const [diagnosis, setDiagnosis] = useState("");

  // Reset on open
  useEffect(() => {
    if (open) {
      setPatient(null);
      setWardId("");
      setBedId("");
      setDoctorId("");
      setKind("planned");
      setComplaint("");
      setDiagnosis("");
    }
  }, [open]);

  // Free beds in the selected ward.
  const freeBeds =
    (bedMap.data ?? []).filter(
      (e) => e.ward_id === wardId && e.bed.status === "free" && e.bed.is_active,
    );

  const create = useMutation({
    mutationFn: (): Promise<unknown> => {
      const input: AdmitInput = {
        patient_id: patient!.id,
        ward_id: wardId,
        bed_id: bedId || undefined,
        admitting_doctor_id: doctorId || undefined,
        kind,
        chief_complaint: complaint.trim() || undefined,
        admission_diagnosis: diagnosis.trim() || undefined,
      };
      return admit(input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: yatisKeys.all(branchId) });
      onClose();
    },
  });

  const errorMsg = (() => {
    if (!create.isError) return null;
    const err = create.error as { code?: string; message?: string } | undefined;
    if (err?.code === "bed_unavailable") return "Seçilen yatak boş değil.";
    if (err?.code === "already_admitted") return "Bu hasta zaten yatıyor.";
    if (err?.code === "wrong_ward") return "Seçilen yatak bu serviste değil.";
    return err?.message ?? "Yatış oluşturulamadı.";
  })();

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Yatış">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="adm-patient" label="Hasta" required>
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
          <Field id="adm-ward" label="Servis" required>
            <SelectInput
              id="adm-ward"
              required
              value={wardId}
              onChange={(e) => {
                setWardId(e.target.value);
                setBedId("");
              }}
            >
              <option value="">— Seçiniz —</option>
              {(wards.data ?? []).map((w) => (
                <option key={w.id} value={w.id}>{w.name} ({w.code})</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="adm-bed" label="Yatak" hint={wardId ? "Sadece boş yataklar listelenir." : "Önce servis seçin."}>
            <SelectInput
              id="adm-bed"
              value={bedId}
              onChange={(e) => setBedId(e.target.value)}
              disabled={!wardId || freeBeds.length === 0}
            >
              <option value="">— Sonra atanacak —</option>
              {freeBeds.map((e) => (
                <option key={e.bed.id} value={e.bed.id}>
                  {e.bed.code}{e.bed.kind !== "standard" && ` (${e.bed.kind})`}
                </option>
              ))}
            </SelectInput>
          </Field>
        </div>

        <Field id="adm-doctor" label="Yatış hekimi">
          <SelectInput
            id="adm-doctor"
            value={doctorId}
            onChange={(e) => setDoctorId(e.target.value)}
          >
            <option value="">— Atanmadı —</option>
            {(doctors.data ?? []).map((d) => (
              <option key={d.id} value={d.id}>
                {d.staff.title ? d.staff.title + " " : ""}{d.staff.first_name} {d.staff.last_name}
              </option>
            ))}
          </SelectInput>
        </Field>

        <Field id="adm-kind" label="Yatış türü" required>
          <SelectInput
            id="adm-kind"
            value={kind}
            onChange={(e) => setKind(e.target.value as AdmissionKind)}
          >
            {Object.entries(KIND_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>

        <Field id="adm-cc" label="Şikayet">
          <Textarea
            id="adm-cc"
            rows={2}
            value={complaint}
            onChange={(e) => setComplaint(e.target.value)}
          />
        </Field>

        <Field id="adm-dx" label="Yatış tanısı">
          <TextInput
            id="adm-dx"
            value={diagnosis}
            onChange={(e) => setDiagnosis(e.target.value)}
            placeholder="Örn. Pnömoni"
          />
        </Field>

        {errorMsg && <p className="text-sm text-[var(--critical)]">{errorMsg}</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !patient || !wardId}>
            {create.isPending ? "Yatış oluşturuluyor..." : "Yatışı oluştur"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
