"use client";

import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Appointment } from "../types/appointment";

// Calendar — week-grid view of appointments for the branch. The
// underlying endpoint is the same as randevu/queries day-listing,
// just over a 7-day range. We keep it under its own keyspace so a
// week navigation doesn't busts the day-cached randevu queries.

export const takvimKeys = {
  week: (branchId: string, weekStart: string, doctorId?: string) =>
    ["takvim", branchId, "week", weekStart, doctorId ?? ""] as const,
};

export function takvimWeekOptions(
  branchId: string,
  weekStart: string, // YYYY-MM-DD, Monday
  doctorId?: string,
) {
  return queryOptions({
    queryKey: takvimKeys.week(branchId, weekStart, doctorId),
    queryFn: () => {
      const params = new URLSearchParams();
      params.set("from", weekStart);
      params.set("to", addDays(weekStart, 6));
      if (doctorId) params.set("doctor_id", doctorId);
      return api().get<Appointment[]>(`/api/appointments?${params.toString()}`);
    },
    enabled: !!branchId && !!weekStart,
  });
}

// addDays returns a YYYY-MM-DD string `n` days after the given date.
// Implemented locally so the core package stays date-lib-free.
export function addDays(ymd: string, n: number): string {
  const [y, m, d] = ymd.split("-").map(Number) as [number, number, number];
  const dt = new Date(Date.UTC(y, m - 1, d));
  dt.setUTCDate(dt.getUTCDate() + n);
  return dt.toISOString().slice(0, 10);
}

// startOfWeek returns the Monday of the week containing `ymd`, in
// YYYY-MM-DD format. Turkish weeks start Monday.
export function startOfWeek(ymd: string): string {
  const [y, m, d] = ymd.split("-").map(Number) as [number, number, number];
  const dt = new Date(Date.UTC(y, m - 1, d));
  const dow = dt.getUTCDay(); // 0=Sun..6=Sat
  // Move back to Monday: Mon=1 → 0 back, Tue=2 → 1 back, …, Sun=0 → 6 back.
  const back = (dow + 6) % 7;
  dt.setUTCDate(dt.getUTCDate() - back);
  return dt.toISOString().slice(0, 10);
}

export function todayLocal(): string {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${dd}`;
}
