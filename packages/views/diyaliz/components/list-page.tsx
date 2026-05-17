"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, Droplet, Plus, Settings } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createDialysisMachine,
  createDialysisSession,
  type CreateDialysisSessionInput,
  dialysisMachineListOptions,
  dialysisSessionListOptions,
  diyalizKeys,
} from "@medigt/core/diyaliz";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import type {
  DialysisModality,
  DialysisSession,
  DialysisStatus,
  Patient,
  VascularAccessType,
} from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  TextInput,
} from "../../common/form-fields";
import { HastaSearch } from "../../randevu/components/hasta-search";

const STATUS_LABELS: Record<DialysisStatus, string> = {
  scheduled: "Planlandı",
  in_progress: "Devam",
  completed: "Tamamlandı",
  cancelled: "İptal",
};

const STATUS_COLORS: Record<DialysisStatus, string> = {
  scheduled: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  in_progress: "bg-violet-100 text-violet-900 dark:bg-violet-950/40 dark:text-violet-200",
  completed: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  cancelled: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};

export const MODALITY_LABELS: Record<DialysisModality, string> = {
  hemodialysis: "Hemodiyaliz",
  hemodiafiltration: "Hemodiyafiltrasyon",
  peritoneal: "Periton Diyalizi",
};

export const ACCESS_LABELS: Record<VascularAccessType, string> = {
  av_fistula: "AV Fistül",
  av_graft: "AV Greft",
  central_catheter: "Santral Kateter",
  peritoneal_catheter: "Periton Kateteri",
  other: "Diğer",
};

function todayISO(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" });
}

export function DiyalizListPage() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const today = todayISO();
  const [date, setDate] = useState(today);
  const [status, setStatus] = useState<DialysisStatus | "">("");
  const machines = useQuery(dialysisMachineListOptions(branchId));
  const list = useQuery(
    dialysisSessionListOptions(branchId, {
      from: date,
      to: date,
      status: status || undefined,
    }),
  );
  const [createOpen, setCreateOpen] = useState(false);
  const [machineOpen, setMachineOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Diyaliz"
          subtitle="Günlük diyaliz seansları. Yeni seans planlayın, pre/post ölçümleri kaydedin."
          actions={
            <div className="flex gap-2">
              <SecondaryButton type="button" onClick={() => setMachineOpen(true)}>
                <span className="inline-flex items-center gap-1"><Settings className="h-4 w-4" /> Cihazlar</span>
              </SecondaryButton>
              <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
                <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Seans</span>
              </PrimaryButton>
            </div>
          }
        />

        <div className="flex flex-wrap items-end gap-3">
          <Field id="dy-date" label="Tarih">
            <TextInput
              id="dy-date"
              type="date"
              value={date}
              onChange={(e) => setDate(e.target.value)}
              className="max-w-xs"
            />
          </Field>
          <Field id="dy-status" label="Durum">
            <SelectInput
              id="dy-status"
              value={status}
              onChange={(e) => setStatus(e.target.value as DialysisStatus | "")}
              className="max-w-xs"
            >
              <option value="">Tümü</option>
              {Object.entries(STATUS_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
          <SecondaryButton
            type="button"
            onClick={() => setDate(today)}
            className="self-end"
          >
            Bugün
          </SecondaryButton>
        </div>

        {(machines.data ?? []).length === 0 ? (
          <div className="empty-state">
            Henüz diyaliz cihazı tanımlanmamış. Önce <strong>Cihazlar</strong> butonundan en az bir cihaz ekleyin.
          </div>
        ) : list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : (list.data ?? []).length === 0 ? (
          <div className="empty-state">Bu gün için planlı seans yok.</div>
        ) : (
          <DataTable<DialysisSession>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={columns()}
          />
        )}
      </div>

      <CreateMachineSheet open={machineOpen} onClose={() => setMachineOpen(false)} branchId={branchId} />
      <CreateSessionSheet
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        branchId={branchId}
        defaultDate={date}
      />
    </DashboardLayout>
  );
}

function columns(): Column<DialysisSession>[] {
  return [
    {
      key: "time",
      header: "Saat",
      cell: (s) => (
        <div>
          <div className="font-mono text-sm font-medium">{formatTime(s.scheduled_at)}</div>
          <div className="text-xs text-muted-foreground">{s.duration_minutes} dk</div>
        </div>
      ),
    },
    {
      key: "machine",
      header: "Cihaz",
      cell: (s) =>
        s.machine_name ? (
          <span className="font-medium">{s.machine_name}</span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
    {
      key: "patient",
      header: "Hasta",
      cell: (s) => (
        <div>
          <div className="font-medium">{s.patient_first_name} {s.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">MRN {s.patient_mrn}</div>
        </div>
      ),
    },
    {
      key: "modality",
      header: "Modalite",
      cell: (s) => (
        <div>
          <div className="text-sm">{MODALITY_LABELS[s.modality] ?? s.modality}</div>
          <div className="text-xs text-muted-foreground">{ACCESS_LABELS[s.vascular_access] ?? s.vascular_access}</div>
        </div>
      ),
    },
    {
      key: "weights",
      header: "Kuru / Hedef UF",
      cell: (s) => (
        <div className="text-xs text-muted-foreground">
          {s.dry_weight_kg ? `${s.dry_weight_kg} kg` : "—"}
          {" · "}
          {s.ultrafiltration_target_ml ? `${s.ultrafiltration_target_ml} ml` : "—"}
        </div>
      ),
    },
    {
      key: "status",
      header: "Durum",
      cell: (s) => (
        <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${STATUS_COLORS[s.status]}`}>
          {STATUS_LABELS[s.status]}
        </span>
      ),
    },
    {
      key: "open",
      header: "",
      cell: (s) => <OpenLink id={s.id} />,
      className: "text-right",
    },
  ];
}

function OpenLink({ id }: { id: string }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const nav = useNavigation();
  return (
    <button
      type="button"
      onClick={() =>
        nav.push(paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "").diyaliz.detail(id))
      }
      className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
    >
      <Activity className="h-3.5 w-3.5" /> Aç
    </button>
  );
}

function CreateMachineSheet({
  open,
  onClose,
  branchId,
}: {
  open: boolean;
  onClose: () => void;
  branchId: string;
}) {
  const qc = useQueryClient();
  const [form, setForm] = useState({ code: "", name: "", manufacturer: "", model: "", location: "" });
  const create = useMutation({
    mutationFn: () =>
      createDialysisMachine({
        code: form.code,
        name: form.name,
        manufacturer: form.manufacturer || undefined,
        model: form.model || undefined,
        location: form.location || undefined,
      }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: diyalizKeys.all(branchId) });
      setForm({ code: "", name: "", manufacturer: "", model: "", location: "" });
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Diyaliz Cihazı">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="dm-name" label="Ad" required>
          <TextInput
            id="dm-name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder="Fresenius 4008S #1"
          />
        </Field>
        <Field id="dm-code" label="Kod" required>
          <TextInput
            id="dm-code"
            required
            value={form.code}
            onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase().replace(/[^A-Z0-9_-]/g, "") })}
            placeholder="HD-01"
          />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field id="dm-manuf" label="Üretici">
            <TextInput
              id="dm-manuf"
              value={form.manufacturer}
              onChange={(e) => setForm({ ...form, manufacturer: e.target.value })}
            />
          </Field>
          <Field id="dm-model" label="Model">
            <TextInput
              id="dm-model"
              value={form.model}
              onChange={(e) => setForm({ ...form, model: e.target.value })}
            />
          </Field>
        </div>
        <Field id="dm-loc" label="Konum">
          <TextInput
            id="dm-loc"
            value={form.location}
            onChange={(e) => setForm({ ...form, location: e.target.value })}
            placeholder="Salon A, 1. yatak"
          />
        </Field>
        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız. Kod zaten kayıtlı olabilir.</p>}
        {create.isSuccess && <p className="text-sm text-emerald-700">Eklendi. Başka cihaz için tekrar kaydedin.</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">Kapat</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !form.code || !form.name}>
            {create.isPending ? "Ekleniyor..." : "Ekle"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function CreateSessionSheet({
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
  const machines = useQuery(dialysisMachineListOptions(branchId));

  const [patient, setPatient] = useState<Patient | null>(null);
  const [machineId, setMachineId] = useState("");
  const [modality, setModality] = useState<DialysisModality>("hemodialysis");
  const [access, setAccess] = useState<VascularAccessType>("av_fistula");
  const [date, setDate] = useState(defaultDate);
  const [time, setTime] = useState("08:00");
  const [duration, setDuration] = useState(240);
  const [dryWeight, setDryWeight] = useState("");
  const [dialyzer, setDialyzer] = useState("");
  const [anticoag, setAnticoag] = useState("");
  const [ufTarget, setUfTarget] = useState("");
  const [bloodFlow, setBloodFlow] = useState("");
  const [dialysate, setDialysate] = useState("");

  useEffect(() => {
    if (open) {
      setPatient(null);
      setMachineId("");
      setModality("hemodialysis");
      setAccess("av_fistula");
      setDate(defaultDate);
      setTime("08:00");
      setDuration(240);
      setDryWeight("");
      setDialyzer("");
      setAnticoag("");
      setUfTarget("");
      setBloodFlow("");
      setDialysate("");
    }
  }, [open, defaultDate]);

  const create = useMutation({
    mutationFn: () => {
      const [hh, mm] = time.split(":");
      const local = new Date(`${date}T${hh}:${mm}:00`);
      const input: CreateDialysisSessionInput = {
        patient_id: patient!.id,
        machine_id: machineId || undefined,
        modality,
        vascular_access: access,
        scheduled_at: local.toISOString(),
        duration_minutes: duration,
        dry_weight_kg: dryWeight ? Number(dryWeight) : undefined,
        dialyzer_type: dialyzer.trim() || undefined,
        anticoagulant: anticoag.trim() || undefined,
        ultrafiltration_target_ml: ufTarget ? Number(ufTarget) : undefined,
        blood_flow_rate: bloodFlow ? Number(bloodFlow) : undefined,
        dialysate_flow_rate: dialysate ? Number(dialysate) : undefined,
      };
      return createDialysisSession(input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: diyalizKeys.all(branchId) });
      onClose();
    },
  });

  const canSubmit = !!patient && !create.isPending;

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Diyaliz Seansı">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="d-patient" label="Hasta" required>
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
          <Field id="d-mod" label="Modalite">
            <SelectInput
              id="d-mod"
              value={modality}
              onChange={(e) => setModality(e.target.value as DialysisModality)}
            >
              {Object.entries(MODALITY_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="d-acc" label="Damar yolu">
            <SelectInput
              id="d-acc"
              value={access}
              onChange={(e) => setAccess(e.target.value as VascularAccessType)}
            >
              {Object.entries(ACCESS_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
        </div>

        <Field id="d-machine" label="Cihaz">
          <SelectInput
            id="d-machine"
            value={machineId}
            onChange={(e) => setMachineId(e.target.value)}
          >
            <option value="">— Atanmadı —</option>
            {(machines.data ?? []).map((m) => (
              <option key={m.id} value={m.id}>{m.name} ({m.code})</option>
            ))}
          </SelectInput>
        </Field>

        <div className="grid grid-cols-3 gap-3">
          <Field id="d-date" label="Tarih" required>
            <TextInput id="d-date" type="date" required value={date} onChange={(e) => setDate(e.target.value)} />
          </Field>
          <Field id="d-time" label="Saat" required>
            <TextInput id="d-time" type="time" required value={time} onChange={(e) => setTime(e.target.value)} />
          </Field>
          <Field id="d-dur" label="Süre (dk)" required>
            <TextInput
              id="d-dur"
              type="number"
              min={30}
              step={15}
              value={String(duration)}
              onChange={(e) => setDuration(Number(e.target.value) || 240)}
            />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field id="d-dry" label="Kuru ağırlık (kg)">
            <TextInput
              id="d-dry"
              type="number"
              step="0.1"
              value={dryWeight}
              onChange={(e) => setDryWeight(e.target.value)}
            />
          </Field>
          <Field id="d-uf" label="Hedef UF (ml)">
            <TextInput
              id="d-uf"
              type="number"
              value={ufTarget}
              onChange={(e) => setUfTarget(e.target.value)}
            />
          </Field>
        </div>

        <Field id="d-dialyzer" label="Diyalizör">
          <TextInput
            id="d-dialyzer"
            value={dialyzer}
            onChange={(e) => setDialyzer(e.target.value)}
            placeholder="Polyflux 17L"
          />
        </Field>

        <Field id="d-anticoag" label="Antikoagülasyon">
          <TextInput
            id="d-anticoag"
            value={anticoag}
            onChange={(e) => setAnticoag(e.target.value)}
            placeholder="Heparin 2000 IU bolus + 1000 IU/sa"
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field id="d-bflow" label="Kan akımı (ml/dk)">
            <TextInput
              id="d-bflow"
              type="number"
              value={bloodFlow}
              onChange={(e) => setBloodFlow(e.target.value)}
            />
          </Field>
          <Field id="d-dflow" label="Diyalizat akımı (ml/dk)">
            <TextInput
              id="d-dflow"
              type="number"
              value={dialysate}
              onChange={(e) => setDialysate(e.target.value)}
            />
          </Field>
        </div>

        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız.</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            <span className="inline-flex items-center gap-1">
              <Droplet className="h-4 w-4" /> {create.isPending ? "Kaydediliyor..." : "Seansı planla"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
