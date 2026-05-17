"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { doktorListOptions } from "@medigt/core/doktor";
import {
  cancelRandevu,
  createRandevu,
  randevuDayOptions,
  randevuKeys,
  updateRandevuStatus,
  type CreateRandevuInput,
} from "@medigt/core/randevu";
import type { Appointment, AppointmentStatus, Patient, VisitKind } from "@medigt/core/types";
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
import { HastaSearch } from "./hasta-search";

const STATUS_LABELS: Record<AppointmentStatus, string> = {
  scheduled: "Planlandı",
  arrived: "Geldi",
  in_progress: "Muayenede",
  completed: "Tamamlandı",
  no_show: "Gelmedi",
  cancelled: "İptal",
};

const STATUS_COLORS: Record<AppointmentStatus, string> = {
  scheduled: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  arrived: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
  in_progress: "bg-violet-100 text-violet-900 dark:bg-violet-950/40 dark:text-violet-200",
  completed: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  no_show: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  cancelled: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};

const VISIT_KIND_LABELS: Record<VisitKind, string> = {
  outpatient: "Poliklinik",
  follow_up: "Kontrol",
  emergency: "Acil",
  consultation: "Konsültasyon",
  control: "Takip",
};

function todayISO(): string {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${dd}`;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" });
}

export function RandevuPage() {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const orgId = org?.id ?? "";

  const [date, setDate] = useState(todayISO());
  const [doctorFilter, setDoctorFilter] = useState<string>("");
  const list = useQuery(randevuDayOptions(branchId, date, doctorFilter || undefined));
  const doktorList = useQuery(doktorListOptions(orgId));

  const [createOpen, setCreateOpen] = useState(false);
  const [cancelTarget, setCancelTarget] = useState<Appointment | null>(null);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Randevular"
          subtitle="Günlük randevu listesi. Hastalar geldikçe durumlarını güncelleyin; muayene tamamlandığında poliklinik akışına geçilir."
          actions={
            <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
              <span className="inline-flex items-center gap-1">
                <Plus className="h-4 w-4" /> Yeni Randevu
              </span>
            </PrimaryButton>
          }
        />

        <div className="flex flex-wrap items-end gap-3">
          <Field id="r-date" label="Tarih">
            <TextInput
              id="r-date"
              type="date"
              value={date}
              onChange={(e) => setDate(e.target.value)}
              className="max-w-xs"
            />
          </Field>
          <Field id="r-doc" label="Doktor">
            <SelectInput
              id="r-doc"
              value={doctorFilter}
              onChange={(e) => setDoctorFilter(e.target.value)}
              className="max-w-xs"
            >
              <option value="">Tüm doktorlar</option>
              {(doktorList.data ?? []).map((d) => (
                <option key={d.id} value={d.id}>
                  {d.staff.title ? d.staff.title + " " : ""}{d.staff.first_name} {d.staff.last_name}
                </option>
              ))}
            </SelectInput>
          </Field>
          <SecondaryButton
            type="button"
            onClick={() => setDate(todayISO())}
            className="self-end"
          >
            Bugün
          </SecondaryButton>
        </div>

        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Liste yüklenemedi.</div>
        ) : (
          <DataTable<Appointment>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={columns(setCancelTarget)}
          />
        )}
      </div>

      <CreateRandevuSheet
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        branchId={branchId}
        defaultDate={date}
      />

      <CancelSheet appt={cancelTarget} onClose={() => setCancelTarget(null)} branchId={branchId} date={date} />
    </DashboardLayout>
  );
}

function columns(setCancelTarget: (a: Appointment) => void): Column<Appointment>[] {
  return [
    {
      key: "time",
      header: "Saat",
      cell: (a) => (
        <div className="font-mono text-sm font-medium">{formatTime(a.scheduled_at)}</div>
      ),
    },
    {
      key: "patient",
      header: "Hasta",
      cell: (a) => (
        <div>
          <div className="font-medium">{a.patient_first_name} {a.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">
            MRN {a.patient_mrn}
            {a.patient_phone && ` · ${a.patient_phone}`}
          </div>
        </div>
      ),
    },
    {
      key: "doctor",
      header: "Doktor",
      cell: (a) =>
        a.doctor_first_name ? (
          <span>
            {a.doctor_title ? a.doctor_title + " " : ""}
            {a.doctor_first_name} {a.doctor_last_name}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">— atanmadı</span>
        ),
    },
    {
      key: "kind",
      header: "Tür",
      cell: (a) => <span className="text-sm">{VISIT_KIND_LABELS[a.kind] ?? a.kind}</span>,
    },
    {
      key: "duration",
      header: "Süre",
      cell: (a) => <span className="text-sm text-muted-foreground">{a.duration_minutes} dk</span>,
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
      key: "actions",
      header: "",
      cell: (a) => <RowActions appt={a} onCancel={() => setCancelTarget(a)} />,
      className: "text-right",
    },
  ];
}

function RowActions({ appt, onCancel }: { appt: Appointment; onCancel: () => void }) {
  const qc = useQueryClient();
  const update = useMutation({
    mutationFn: (status: Parameters<typeof updateRandevuStatus>[1]) =>
      updateRandevuStatus(appt.id, status),
    onSuccess: () => qc.invalidateQueries({ queryKey: randevuKeys.all(appt.branch_id) }),
  });

  if (appt.status === "scheduled") {
    return (
      <div className="flex justify-end gap-1">
        <SecondaryButton type="button" onClick={() => update.mutate("arrived")} className="px-2 py-1 text-xs">
          Geldi
        </SecondaryButton>
        <SecondaryButton type="button" onClick={() => update.mutate("no_show")} className="px-2 py-1 text-xs">
          Gelmedi
        </SecondaryButton>
        <SecondaryButton
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onCancel();
          }}
          className="px-2 py-1 text-xs text-[var(--critical)]"
        >
          İptal
        </SecondaryButton>
      </div>
    );
  }
  if (appt.status === "arrived") {
    return (
      <PrimaryButton type="button" onClick={() => update.mutate("in_progress")} className="px-2 py-1 text-xs">
        Muayeneye al
      </PrimaryButton>
    );
  }
  if (appt.status === "in_progress") {
    return (
      <PrimaryButton type="button" onClick={() => update.mutate("completed")} className="px-2 py-1 text-xs">
        Tamamla
      </PrimaryButton>
    );
  }
  return null;
}

function CreateRandevuSheet({
  open,
  onClose,
  branchId,
  defaultDate,
}: {
  open: boolean;
  onClose: () => void;
  branchId: string;
  defaultDate: string;
}) {
  const qc = useQueryClient();
  const org = useHospitalStore((s) => s.organization);
  const doktorList = useQuery(doktorListOptions(org?.id ?? ""));

  const [patient, setPatient] = useState<Patient | null>(null);
  const [doctorId, setDoctorId] = useState("");
  const [date, setDate] = useState(defaultDate);
  const [time, setTime] = useState("09:00");
  const [duration, setDuration] = useState(20);
  const [kind, setKind] = useState<VisitKind>("outpatient");
  const [reason, setReason] = useState("");

  // Reset form on open.
  useMemo(() => {
    if (open) {
      setPatient(null);
      setDate(defaultDate);
      setTime("09:00");
      setDuration(20);
      setKind("outpatient");
      setReason("");
      setDoctorId("");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const create = useMutation({
    mutationFn: (): Promise<Appointment> => {
      // Combine date + time into a local ISO string with timezone offset.
      const [hh, mm] = time.split(":");
      const local = new Date(`${date}T${hh}:${mm}:00`);
      const scheduledAt = local.toISOString();
      const input: CreateRandevuInput = {
        patient_id: patient!.id,
        scheduled_at: scheduledAt,
        duration_minutes: duration,
        kind,
        reason: reason.trim() || undefined,
        doctor_id: doctorId || undefined,
      };
      return createRandevu(input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: randevuKeys.all(branchId) });
      onClose();
    },
  });

  const canSubmit = !!patient && !!date && !!time && !create.isPending;

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Randevu">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="r-patient" label="Hasta" required hint="Hasta arama: ad, TC, telefon, MRN">
          {patient ? (
            <div className="flex items-center justify-between rounded-md border border-border bg-muted/40 px-3 py-2">
              <div>
                <div className="font-medium">{patient.first_name} {patient.last_name}</div>
                <div className="text-xs text-muted-foreground">
                  MRN {patient.mrn}
                  {patient.identifier_masked && ` · ${patient.identifier_masked}`}
                </div>
              </div>
              <button
                type="button"
                onClick={() => setPatient(null)}
                className="text-xs text-muted-foreground hover:underline"
              >
                Değiştir
              </button>
            </div>
          ) : (
            <HastaSearch onPick={setPatient} />
          )}
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field id="r-date2" label="Tarih" required>
            <TextInput id="r-date2" type="date" required value={date} onChange={(e) => setDate(e.target.value)} />
          </Field>
          <Field id="r-time" label="Saat" required>
            <TextInput id="r-time" type="time" required value={time} onChange={(e) => setTime(e.target.value)} step={300} />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field id="r-dur" label="Süre (dk)" required>
            <TextInput
              id="r-dur"
              type="number"
              min={5}
              max={480}
              step={5}
              value={String(duration)}
              onChange={(e) => setDuration(Number(e.target.value) || 20)}
            />
          </Field>
          <Field id="r-kind" label="Tür">
            <SelectInput id="r-kind" value={kind} onChange={(e) => setKind(e.target.value as VisitKind)}>
              {Object.entries(VISIT_KIND_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
        </div>

        <Field id="r-doctor" label="Doktor" hint="Boş bırakılırsa daha sonra atanır">
          <SelectInput id="r-doctor" value={doctorId} onChange={(e) => setDoctorId(e.target.value)}>
            <option value="">— Sonra atanacak —</option>
            {(doktorList.data ?? []).map((d) => (
              <option key={d.id} value={d.id}>
                {d.staff.title ? d.staff.title + " " : ""}{d.staff.first_name} {d.staff.last_name}
                {d.specializations.length > 0 && ` — ${d.specializations[0]!.name}`}
              </option>
            ))}
          </SelectInput>
        </Field>

        <Field id="r-reason" label="Şikayet / Sebep">
          <Textarea id="r-reason" rows={2} value={reason} onChange={(e) => setReason(e.target.value)} />
        </Field>

        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız.</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            {create.isPending ? "Kaydediliyor..." : "Randevuyu oluştur"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function CancelSheet({
  appt,
  onClose,
  branchId,
  date,
}: {
  appt: Appointment | null;
  onClose: () => void;
  branchId: string;
  date: string;
}) {
  const qc = useQueryClient();
  const [reason, setReason] = useState("");

  const cancel = useMutation({
    mutationFn: () => cancelRandevu(appt!.id, reason),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: randevuKeys.day(branchId, date) });
      onClose();
      setReason("");
    },
  });

  if (!appt) return null;
  return (
    <SideSheet open onClose={onClose} title="Randevuyu iptal et">
      <div className="space-y-4">
        <div className="rounded-md border border-border bg-muted/40 px-3 py-2 text-sm">
          <div className="font-medium">{appt.patient_first_name} {appt.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">
            {formatTime(appt.scheduled_at)} · {VISIT_KIND_LABELS[appt.kind]} · {appt.duration_minutes} dk
          </div>
        </div>
        <Field id="c-reason" label="İptal sebebi" hint="Opsiyonel ama tavsiye edilir.">
          <Textarea id="c-reason" rows={3} value={reason} onChange={(e) => setReason(e.target.value)} />
        </Field>
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">Vazgeç</SecondaryButton>
          <PrimaryButton
            type="button"
            onClick={() => cancel.mutate()}
            disabled={cancel.isPending}
            className="flex-1 bg-[var(--critical)] hover:bg-[var(--critical)]/90"
          >
            {cancel.isPending ? "İptal ediliyor..." : "İptal et"}
          </PrimaryButton>
        </div>
      </div>
    </SideSheet>
  );
}
