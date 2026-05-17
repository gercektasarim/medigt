"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, HeartPulse, Plus, Thermometer } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { hemsireBoardOptions, hemsireKeys, addPatientVitals, type AddPatientVitalsInput } from "@medigt/core/hemsire";
import { wardListOptions } from "@medigt/core/yatis";
import type { InpatientBoardRow } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";

export function HemsireBoardPage() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const [wardId, setWardId] = useState<string>("");
  const wards = useQuery(wardListOptions(branchId));
  const board = useQuery(hemsireBoardOptions(branchId, wardId || undefined));
  const [vitalsFor, setVitalsFor] = useState<InpatientBoardRow | null>(null);

  // Group rows by ward to make the board scan-friendly.
  const groups = new Map<string, InpatientBoardRow[]>();
  for (const r of board.data ?? []) {
    const arr = groups.get(r.ward_id) ?? [];
    arr.push(r);
    groups.set(r.ward_id, arr);
  }

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Hemşire Panosu"
          subtitle="Yatan hastaların güncel vital değerleri. Hızlı vital giriş + son ölçüm zamanı görünür."
        />

        <div className="flex flex-wrap items-end gap-3">
          <Field id="hb-ward" label="Servis filtresi">
            <SelectInput
              id="hb-ward"
              value={wardId}
              onChange={(e) => setWardId(e.target.value)}
              className="max-w-xs"
            >
              <option value="">Tüm servisler</option>
              {(wards.data ?? []).map((w) => (
                <option key={w.id} value={w.id}>{w.name} ({w.code})</option>
              ))}
            </SelectInput>
          </Field>
        </div>

        {board.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : (board.data ?? []).length === 0 ? (
          <div className="empty-state">Yatan hasta yok.</div>
        ) : (
          <div className="space-y-6">
            {Array.from(groups.entries()).map(([wid, rows]) => (
              <section key={wid} className="space-y-2">
                <h2 className="text-sm font-semibold text-muted-foreground">
                  {rows[0]?.ward_name} · {rows.length} hasta
                </h2>
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                  {rows.map((r) => (
                    <PatientCard key={r.admission_id} row={r} onAddVitals={() => setVitalsFor(r)} />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>

      {vitalsFor && (
        <AddVitalsSheet
          row={vitalsFor}
          onClose={() => setVitalsFor(null)}
          branchId={branchId}
        />
      )}
    </DashboardLayout>
  );
}

function PatientCard({
  row,
  onAddVitals,
}: {
  row: InpatientBoardRow;
  onAddVitals: () => void;
}) {
  const isStale = (() => {
    if (!row.vitals_measured_at) return true;
    const hoursSince = (Date.now() - new Date(row.vitals_measured_at).getTime()) / 36e5;
    return hoursSince > 8; // 8 saatten eski ölçüm — pano "stale" sarısı yapacak
  })();

  return (
    <article
      className={
        "rounded-md border bg-card p-4 shadow-sm " +
        (isStale ? "border-amber-300 dark:border-amber-900" : "border-border")
      }
    >
      <header className="mb-2 flex items-start justify-between gap-2">
        <div>
          <div className="font-medium">
            {row.patient_first_name} {row.patient_last_name}
          </div>
          <div className="text-xs text-muted-foreground">
            MRN {row.patient_mrn}
            {row.bed_code && ` · Yatak ${row.bed_code}`}
          </div>
        </div>
        <SecondaryButton type="button" onClick={onAddVitals} className="px-2 py-1 text-xs">
          <span className="inline-flex items-center gap-1">
            <Plus className="h-3.5 w-3.5" /> Vital
          </span>
        </SecondaryButton>
      </header>

      <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-sm">
        <VitalCell
          icon={<Activity className="h-3.5 w-3.5" />}
          label="TA"
          value={
            row.systolic_bp != null && row.diastolic_bp != null
              ? `${row.systolic_bp}/${row.diastolic_bp}`
              : null
          }
          unit="mmHg"
        />
        <VitalCell
          icon={<HeartPulse className="h-3.5 w-3.5" />}
          label="Nabız"
          value={row.pulse != null ? String(row.pulse) : null}
          unit="bpm"
        />
        <VitalCell
          icon={<Thermometer className="h-3.5 w-3.5" />}
          label="Ateş"
          value={row.temperature_c != null ? row.temperature_c.toFixed(1) : null}
          unit="°C"
          alert={row.temperature_c != null && (row.temperature_c >= 38.0 || row.temperature_c <= 36.0)}
        />
        <VitalCell
          icon={<span className="text-xs">O₂</span>}
          label="SpO₂"
          value={row.spo2_percent != null ? String(row.spo2_percent) : null}
          unit="%"
          alert={row.spo2_percent != null && row.spo2_percent < 92}
        />
      </div>

      <footer className="mt-3 border-t border-border pt-2 text-xs text-muted-foreground">
        {row.vitals_measured_at
          ? `Son ölçüm: ${new Date(row.vitals_measured_at).toLocaleString("tr-TR", {
              day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit",
            })}${isStale ? " — yenile" : ""}`
          : "Henüz vital alınmamış."}
        {row.admission_diagnosis && (
          <div className="mt-1 italic">{row.admission_diagnosis}</div>
        )}
      </footer>
    </article>
  );
}

function VitalCell({
  icon,
  label,
  value,
  unit,
  alert,
}: {
  icon: React.ReactNode;
  label: string;
  value: string | null;
  unit: string;
  alert?: boolean;
}) {
  return (
    <div className="flex items-baseline gap-1.5">
      <span className="flex h-4 w-4 items-center justify-center text-muted-foreground">{icon}</span>
      <span className="text-xs text-muted-foreground">{label}</span>
      {value != null ? (
        <span className={"ml-auto font-mono " + (alert ? "font-semibold text-[var(--critical)]" : "")}>
          {value} <span className="text-xs text-muted-foreground">{unit}</span>
        </span>
      ) : (
        <span className="ml-auto text-xs text-muted-foreground">—</span>
      )}
    </div>
  );
}

function AddVitalsSheet({
  row,
  onClose,
  branchId,
}: {
  row: InpatientBoardRow;
  onClose: () => void;
  branchId: string;
}) {
  const qc = useQueryClient();
  const [form, setForm] = useState<Record<string, string>>({});

  const add = useMutation({
    mutationFn: () => {
      const input: AddPatientVitalsInput = {
        systolic_bp: parseInt(form.systolic_bp ?? "", 10) || undefined,
        diastolic_bp: parseInt(form.diastolic_bp ?? "", 10) || undefined,
        pulse: parseInt(form.pulse ?? "", 10) || undefined,
        temperature_c: parseFloat(form.temperature_c ?? "") || undefined,
        spo2_percent: parseInt(form.spo2_percent ?? "", 10) || undefined,
        respiration: parseInt(form.respiration ?? "", 10) || undefined,
        pain_score: parseInt(form.pain_score ?? "", 10) || undefined,
        notes: form.notes?.trim() || undefined,
      };
      return addPatientVitals(row.patient_id, input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: hemsireKeys.all(branchId) });
      onClose();
    },
  });

  const cell = (label: string, name: string, suffix?: string, decimal?: boolean) => (
    <Field id={`v-${name}`} label={label}>
      <div className="flex items-center gap-1">
        <TextInput
          id={`v-${name}`}
          type="number"
          step={decimal ? "0.1" : "1"}
          value={form[name] ?? ""}
          onChange={(e) => setForm({ ...form, [name]: e.target.value })}
        />
        {suffix && <span className="text-xs text-muted-foreground">{suffix}</span>}
      </div>
    </Field>
  );

  return (
    <SideSheet
      open
      onClose={onClose}
      title={`Vital · ${row.patient_first_name} ${row.patient_last_name}`}
    >
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); add.mutate(); }}>
        <div className="rounded-md border border-border bg-muted/40 px-3 py-2 text-xs">
          <div className="font-medium text-foreground">
            {row.patient_first_name} {row.patient_last_name}
          </div>
          <div>MRN {row.patient_mrn}{row.bed_code && ` · Yatak ${row.bed_code}`}</div>
        </div>

        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {cell("TA Sistolik", "systolic_bp", "mmHg")}
          {cell("TA Diastolik", "diastolic_bp", "mmHg")}
          {cell("Nabız", "pulse", "bpm")}
          {cell("Ateş", "temperature_c", "°C", true)}
          {cell("SpO₂", "spo2_percent", "%")}
          {cell("Solunum", "respiration", "/dk")}
          {cell("Ağrı (0-10)", "pain_score")}
        </div>

        <Field id="vnotes" label="Not">
          <Textarea
            id="vnotes"
            rows={2}
            value={form.notes ?? ""}
            onChange={(e) => setForm({ ...form, notes: e.target.value })}
          />
        </Field>

        {add.isError && <p className="text-sm text-[var(--critical)]">Kaydedilemedi.</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={add.isPending}>
            {add.isPending ? "Kaydediliyor..." : "Vital kaydet"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
