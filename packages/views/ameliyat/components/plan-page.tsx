"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Scissors, Settings } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  ameliyatKeys,
  createOperatingRoom,
  createSurgery,
  operatingRoomListOptions,
  surgeryListOptions,
  type CreateSurgeryInput,
} from "@medigt/core/ameliyat";
import { doktorListOptions } from "@medigt/core/doktor";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import type {
  AnesthesiaType,
  Patient,
  Surgery,
  SurgeryPriority,
  SurgeryStatus,
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

const STATUS_LABELS: Record<SurgeryStatus, string> = {
  scheduled: "Planlandı",
  in_progress: "Ameliyatta",
  completed: "Tamamlandı",
  cancelled: "İptal",
};

const STATUS_COLORS: Record<SurgeryStatus, string> = {
  scheduled: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  in_progress: "bg-violet-100 text-violet-900 dark:bg-violet-950/40 dark:text-violet-200",
  completed: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  cancelled: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};

const PRIORITY_LABELS: Record<SurgeryPriority, string> = {
  elective: "Elektif",
  urgent: "Aciliyet",
  emergency: "Acil",
};

const ANESTHESIA_LABELS: Record<AnesthesiaType, string> = {
  general: "Genel",
  regional: "Bölgesel",
  spinal: "Spinal",
  epidural: "Epidural",
  local: "Lokal",
  sedation: "Sedasyon",
  none: "Anestezisiz",
};

function todayISO(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" });
}

export function AmeliyatPlanPage() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const today = todayISO();
  const [date, setDate] = useState(today);
  const [status, setStatus] = useState<SurgeryStatus | "">("");
  const ors = useQuery(operatingRoomListOptions(branchId));
  const list = useQuery(surgeryListOptions(branchId, {
    from: date, to: date,
    status: status || undefined,
  }));
  const [createOpen, setCreateOpen] = useState(false);
  const [orOpen, setOrOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Ameliyat Planı"
          subtitle="Günlük ameliyat takvimi. Ameliyat ekleyin, durumlarını takip edin, op-notunu kaydedin."
          actions={
            <div className="flex gap-2">
              <SecondaryButton type="button" onClick={() => setOrOpen(true)}>
                <span className="inline-flex items-center gap-1"><Settings className="h-4 w-4" /> Ameliyathane</span>
              </SecondaryButton>
              <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
                <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Ameliyat</span>
              </PrimaryButton>
            </div>
          }
        />

        <div className="flex flex-wrap items-end gap-3">
          <Field id="ame-date" label="Tarih">
            <TextInput
              id="ame-date" type="date"
              value={date} onChange={(e) => setDate(e.target.value)}
              className="max-w-xs"
            />
          </Field>
          <Field id="ame-status" label="Durum">
            <SelectInput
              id="ame-status"
              value={status}
              onChange={(e) => setStatus(e.target.value as SurgeryStatus | "")}
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

        {(ors.data ?? []).length === 0 ? (
          <div className="empty-state">
            Henüz ameliyathane tanımlanmamış. Önce <strong>Ameliyathane</strong> butonundan en az bir oda ekleyin.
          </div>
        ) : list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : (list.data ?? []).length === 0 ? (
          <div className="empty-state">Bu gün için planlı ameliyat yok.</div>
        ) : (
          <DataTable<Surgery>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={columns()}
          />
        )}
      </div>

      <CreateORSheet open={orOpen} onClose={() => setOrOpen(false)} branchId={branchId} />
      <CreateSurgerySheet
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        branchId={branchId}
        defaultDate={date}
      />
    </DashboardLayout>
  );
}

function columns(): Column<Surgery>[] {
  return [
    {
      key: "time",
      header: "Saat",
      cell: (s) => (
        <div>
          <div className="font-mono text-sm font-medium">{formatTime(s.scheduled_at)}</div>
          <div className="text-xs text-muted-foreground">{s.estimated_minutes} dk</div>
        </div>
      ),
    },
    {
      key: "or",
      header: "Salon",
      cell: (s) => (
        <span className="font-medium">{s.operating_room_name}</span>
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
      key: "procedure",
      header: "İşlem",
      cell: (s) => (
        <div>
          <div className="font-medium">{s.procedure_name}</div>
          <div className="text-xs text-muted-foreground">
            Anestezi: {ANESTHESIA_LABELS[s.anesthesia_type] ?? s.anesthesia_type}
          </div>
        </div>
      ),
    },
    {
      key: "surgeon",
      header: "Cerrah",
      cell: (s) =>
        s.surgeon_first_name ? (
          <span>
            {s.surgeon_title ? s.surgeon_title + " " : ""}
            {s.surgeon_first_name} {s.surgeon_last_name}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
    {
      key: "priority",
      header: "Öncelik",
      cell: (s) => (
        <span className={"text-xs " + (s.priority !== "elective" ? "font-semibold text-[var(--critical)]" : "")}>
          {PRIORITY_LABELS[s.priority] ?? s.priority}
        </span>
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
        nav.push(paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "").ameliyat.detail(id))
      }
      className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
    >
      <Scissors className="h-3.5 w-3.5" /> Aç
    </button>
  );
}

function CreateORSheet({ open, onClose, branchId }: { open: boolean; onClose: () => void; branchId: string }) {
  const qc = useQueryClient();
  const [form, setForm] = useState({ code: "", name: "", floor: "" });
  const create = useMutation({
    mutationFn: () => createOperatingRoom(form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ameliyatKeys.all(branchId) });
      setForm({ code: "", name: "", floor: "" });
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Ameliyathane">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="or-name" label="Ad" required>
          <TextInput id="or-name" required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="Ameliyathane 1" />
        </Field>
        <Field id="or-code" label="Kod" required>
          <TextInput
            id="or-code" required
            value={form.code}
            onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase().replace(/[^A-Z0-9_-]/g, "") })}
            placeholder="AM1"
          />
        </Field>
        <Field id="or-floor" label="Kat">
          <TextInput id="or-floor" value={form.floor} onChange={(e) => setForm({ ...form, floor: e.target.value })} />
        </Field>
        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız. Kod zaten kayıtlı olabilir.</p>}
        {create.isSuccess && <p className="text-sm text-emerald-700">Eklendi. Başka oda için tekrar kaydedin.</p>}
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

function CreateSurgerySheet({
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
  const ors = useQuery(operatingRoomListOptions(branchId));
  const doctors = useQuery(doktorListOptions(org?.id ?? ""));

  const [patient, setPatient] = useState<Patient | null>(null);
  const [orId, setOrId] = useState("");
  const [surgeonId, setSurgeonId] = useState("");
  const [procedure, setProcedure] = useState("");
  const [priority, setPriority] = useState<SurgeryPriority>("elective");
  const [anesthesia, setAnesthesia] = useState<AnesthesiaType>("general");
  const [date, setDate] = useState(defaultDate);
  const [time, setTime] = useState("09:00");
  const [duration, setDuration] = useState(60);
  const [indication, setIndication] = useState("");

  useEffect(() => {
    if (open) {
      setPatient(null);
      setOrId("");
      setSurgeonId("");
      setProcedure("");
      setPriority("elective");
      setAnesthesia("general");
      setDate(defaultDate);
      setTime("09:00");
      setDuration(60);
      setIndication("");
    }
  }, [open, defaultDate]);

  const create = useMutation({
    mutationFn: (): Promise<Surgery> => {
      const [hh, mm] = time.split(":");
      const local = new Date(`${date}T${hh}:${mm}:00`);
      const scheduledAt = local.toISOString();
      const surgeon = doctors.data?.find((d) => d.id === surgeonId);
      const team = surgeon
        ? [
            {
              doctor_id: surgeon.id,
              role: "primary_surgeon" as const,
              name:
                (surgeon.staff.title ? surgeon.staff.title + " " : "") +
                `${surgeon.staff.first_name} ${surgeon.staff.last_name}`,
            },
          ]
        : [];
      const input: CreateSurgeryInput = {
        patient_id: patient!.id,
        operating_room_id: orId,
        primary_surgeon_id: surgeonId || undefined,
        priority,
        procedure_name: procedure.trim(),
        anesthesia_type: anesthesia,
        scheduled_at: scheduledAt,
        estimated_minutes: duration,
        indication: indication.trim() || undefined,
        team,
      };
      return createSurgery(input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ameliyatKeys.all(branchId) });
      onClose();
    },
  });

  const canSubmit = patient && orId && procedure.trim().length > 0 && !create.isPending;

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Ameliyat">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="s-patient" label="Hasta" required>
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

        <Field id="s-proc" label="İşlem adı" required>
          <TextInput
            id="s-proc"
            required
            value={procedure}
            onChange={(e) => setProcedure(e.target.value)}
            placeholder="Örn. Laparoskopik kolesistektomi"
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field id="s-or" label="Ameliyathane" required>
            <SelectInput
              id="s-or" required
              value={orId}
              onChange={(e) => setOrId(e.target.value)}
            >
              <option value="">— Seçiniz —</option>
              {(ors.data ?? []).map((o) => (
                <option key={o.id} value={o.id}>{o.name} ({o.code})</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="s-priority" label="Öncelik">
            <SelectInput
              id="s-priority"
              value={priority}
              onChange={(e) => setPriority(e.target.value as SurgeryPriority)}
            >
              {Object.entries(PRIORITY_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
        </div>

        <div className="grid grid-cols-3 gap-3">
          <Field id="s-date" label="Tarih" required>
            <TextInput id="s-date" type="date" required value={date} onChange={(e) => setDate(e.target.value)} />
          </Field>
          <Field id="s-time" label="Saat" required>
            <TextInput id="s-time" type="time" required value={time} onChange={(e) => setTime(e.target.value)} />
          </Field>
          <Field id="s-dur" label="Süre (dk)" required>
            <TextInput
              id="s-dur"
              type="number"
              min={15} step={5}
              value={String(duration)}
              onChange={(e) => setDuration(Number(e.target.value) || 60)}
            />
          </Field>
        </div>

        <Field id="s-surgeon" label="Birincil cerrah">
          <SelectInput
            id="s-surgeon"
            value={surgeonId}
            onChange={(e) => setSurgeonId(e.target.value)}
          >
            <option value="">— Atanmadı —</option>
            {(doctors.data ?? []).map((d) => (
              <option key={d.id} value={d.id}>
                {d.staff.title ? d.staff.title + " " : ""}{d.staff.first_name} {d.staff.last_name}
                {d.specializations.length > 0 && ` — ${d.specializations[0]!.name}`}
              </option>
            ))}
          </SelectInput>
        </Field>

        <Field id="s-anes" label="Anestezi türü">
          <SelectInput
            id="s-anes"
            value={anesthesia}
            onChange={(e) => setAnesthesia(e.target.value as AnesthesiaType)}
          >
            {Object.entries(ANESTHESIA_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>

        <Field id="s-indic" label="Endikasyon">
          <Textarea
            id="s-indic" rows={2}
            value={indication}
            onChange={(e) => setIndication(e.target.value)}
          />
        </Field>

        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız.</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            {create.isPending ? "Kaydediliyor..." : "Ameliyatı planla"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
