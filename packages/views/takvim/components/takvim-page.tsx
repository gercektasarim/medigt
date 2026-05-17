"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ChevronLeft,
  ChevronRight,
  Calendar as CalendarIcon,
  CalendarClock,
  Filter,
} from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { addDays, startOfWeek, takvimWeekOptions, todayLocal } from "@medigt/core/takvim";
import { doktorListOptions } from "@medigt/core/doktor";
import { paths } from "@medigt/core/paths";
import { useNavigation } from "@medigt/core/navigation";
import type { Appointment, AppointmentStatus } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import {
  Field,
  SecondaryButton,
  SelectInput,
} from "../../common/form-fields";

// Takvim — appointment week-grid. Monday-start, 08:00–20:00 hour rows.
// Each appointment block sits in the column for its day and snaps to
// the slot containing its scheduled_at. Click → open the visit
// (if started) or the appointment row in the randevu list.

const HOUR_START = 8;
const HOUR_END = 20; // exclusive — last visible slot is 19:00–20:00
const DAY_LABELS = ["Pzt", "Sal", "Çar", "Per", "Cum", "Cmt", "Paz"];

const STATUS_STYLES: Record<AppointmentStatus, string> = {
  scheduled:
    "bg-[var(--accent-soft)] text-[var(--brand)] border-[color-mix(in_oklch,var(--brand)_40%,transparent)]",
  arrived:
    "bg-[color-mix(in_oklch,var(--warning)_14%,transparent)] text-[var(--warning)] border-[color-mix(in_oklch,var(--warning)_40%,transparent)]",
  in_progress:
    "bg-[color-mix(in_oklch,var(--info)_14%,transparent)] text-[var(--info)] border-[color-mix(in_oklch,var(--info)_40%,transparent)]",
  completed:
    "bg-[color-mix(in_oklch,var(--success)_12%,transparent)] text-[var(--success)] border-[color-mix(in_oklch,var(--success)_40%,transparent)]",
  no_show:
    "bg-muted text-muted-foreground border-border",
  cancelled:
    "bg-muted text-muted-foreground border-border line-through opacity-60",
};

export function TakvimPage() {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const orgId = org?.id ?? "";
  const nav = useNavigation();

  const [weekStart, setWeekStart] = useState<string>(() => startOfWeek(todayLocal()));
  const [doctorId, setDoctorId] = useState<string>("");

  const doctors = useQuery(doktorListOptions(orgId));
  const appts = useQuery(takvimWeekOptions(branchId, weekStart, doctorId || undefined));

  // Bucket appointments into [day-index][hour-slot] for O(1) lookup.
  const byDayHour = useMemo(() => {
    const grid: Record<number, Record<number, Appointment[]>> = {};
    for (let d = 0; d < 7; d++) {
      const row: Record<number, Appointment[]> = {};
      for (let h = HOUR_START; h < HOUR_END; h++) row[h] = [];
      grid[d] = row;
    }
    const startMs = new Date(weekStart + "T00:00:00").getTime();
    for (const a of appts.data ?? []) {
      const dt = new Date(a.scheduled_at);
      const dayIdx = Math.floor((dt.getTime() - startMs) / 86_400_000);
      if (dayIdx < 0 || dayIdx > 6) continue;
      const hour = dt.getHours();
      if (hour < HOUR_START || hour >= HOUR_END) continue;
      grid[dayIdx]![hour]!.push(a);
    }
    return grid;
  }, [appts.data, weekStart]);

  // Visible week label e.g. "12 May – 18 May 2026"
  const weekLabel = useMemo(() => {
    const end = addDays(weekStart, 6);
    const fmt = (ymd: string) =>
      new Date(ymd + "T00:00:00").toLocaleDateString("tr-TR", {
        day: "2-digit",
        month: "short",
      });
    return `${fmt(weekStart)} – ${fmt(end)} ${weekStart.slice(0, 4)}`;
  }, [weekStart]);

  const goToVisit = (a: Appointment) => {
    const base = paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "");
    // If a visit was started for this appointment, the poliklinik
    // detail page shows everything. Otherwise the randevu list is the
    // natural next step (no per-appointment detail page exists).
    if (a.status === "in_progress" || a.status === "completed") {
      nav.push(base.randevu.list()); // placeholder until visit deep-link is added
      return;
    }
    nav.push(base.randevu.list());
  };

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Takvim"
          subtitle="Haftalık randevu görünümü. Saat-saat blok hizalama, status renkleri ve doktor filtresiyle."
          actions={
            <div className="flex flex-wrap items-center gap-2">
              <SecondaryButton type="button" onClick={() => setWeekStart(startOfWeek(todayLocal()))}>
                <span className="inline-flex items-center gap-1">
                  <CalendarClock className="h-3.5 w-3.5" /> Bugün
                </span>
              </SecondaryButton>
              <SecondaryButton type="button" onClick={() => setWeekStart(addDays(weekStart, -7))}>
                <ChevronLeft className="h-4 w-4" />
              </SecondaryButton>
              <div className="min-w-[10rem] text-center text-sm font-medium">{weekLabel}</div>
              <SecondaryButton type="button" onClick={() => setWeekStart(addDays(weekStart, 7))}>
                <ChevronRight className="h-4 w-4" />
              </SecondaryButton>
            </div>
          }
        />

        <section className="surface-card-muted flex flex-wrap items-end gap-3 p-3">
          <div className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
            <Filter className="h-3.5 w-3.5" /> Filtre
          </div>
          <Field id="t-doctor" label="Doktor" hint="Boş bırakılırsa tüm doktorlar">
            <SelectInput
              id="t-doctor"
              value={doctorId}
              onChange={(e) => setDoctorId(e.target.value)}
              className="min-w-[14rem]"
            >
              <option value="">Tüm doktorlar</option>
              {(doctors.data ?? []).map((d) => (
                <option key={d.id} value={d.id}>
                  {[d.staff?.title, d.staff?.first_name, d.staff?.last_name].filter(Boolean).join(" ")}
                </option>
              ))}
            </SelectInput>
          </Field>
        </section>

        <div className="surface-card overflow-hidden">
          {/* Header row — day names + dates */}
          <div className="grid grid-cols-[4rem_repeat(7,minmax(0,1fr))] border-b border-border bg-muted/40 text-xs">
            <div className="border-r border-border p-2 text-center text-muted-foreground">
              <CalendarIcon className="mx-auto h-3.5 w-3.5" />
            </div>
            {Array.from({ length: 7 }).map((_, i) => {
              const dayDate = new Date(weekStart + "T00:00:00");
              dayDate.setDate(dayDate.getDate() + i);
              const isToday = dayDate.toDateString() === new Date().toDateString();
              return (
                <div
                  key={i}
                  className={
                    "border-r border-border p-2 text-center last:border-r-0 " +
                    (isToday ? "text-[var(--brand)] font-semibold" : "text-muted-foreground")
                  }
                >
                  <div className="text-[10px] uppercase tracking-wider">{DAY_LABELS[i]}</div>
                  <div className="text-sm font-mono">{dayDate.toLocaleDateString("tr-TR", { day: "2-digit", month: "2-digit" })}</div>
                </div>
              );
            })}
          </div>

          {/* Body — hour rows */}
          <div className="max-h-[70vh] overflow-y-auto">
            {Array.from({ length: HOUR_END - HOUR_START }).map((_, hi) => {
              const hour = HOUR_START + hi;
              return (
                <div
                  key={hour}
                  className="grid grid-cols-[4rem_repeat(7,minmax(0,1fr))] border-b border-border/60 last:border-b-0"
                >
                  <div className="border-r border-border bg-muted/30 px-2 py-2 text-right text-[11px] font-mono text-muted-foreground">
                    {String(hour).padStart(2, "0")}:00
                  </div>
                  {Array.from({ length: 7 }).map((_, di) => {
                    const cell = byDayHour[di]?.[hour] ?? [];
                    return (
                      <div
                        key={di}
                        className="relative min-h-[3.5rem] border-r border-border/60 last:border-r-0"
                      >
                        {cell.map((a) => (
                          <button
                            key={a.id}
                            type="button"
                            onClick={() => goToVisit(a)}
                            className={
                              "m-1 block w-[calc(100%-0.5rem)] truncate rounded-md border px-2 py-1 text-left text-[11px] leading-tight transition hover:brightness-105 " +
                              STATUS_STYLES[a.status]
                            }
                            title={`${a.patient_first_name ?? ""} ${a.patient_last_name ?? ""} — ${a.reason ?? "Randevu"}`}
                          >
                            <div className="font-mono text-[10px] opacity-80">
                              {new Date(a.scheduled_at).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" })}
                            </div>
                            <div className="truncate font-semibold">
                              {a.patient_first_name ?? "—"} {a.patient_last_name ?? ""}
                            </div>
                            {a.reason && <div className="truncate opacity-80">{a.reason}</div>}
                          </button>
                        ))}
                      </div>
                    );
                  })}
                </div>
              );
            })}
          </div>
        </div>

        {/* Status legend */}
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <span className="text-muted-foreground">Durumlar:</span>
          <span className="chip-accent">Planlandı</span>
          <span className="chip-warning">Geldi</span>
          <span className="chip-info">Devam ediyor</span>
          <span className="chip-success">Tamamlandı</span>
          <span className="chip">Gelmedi / İptal</span>
        </div>
      </div>
    </DashboardLayout>
  );
}
