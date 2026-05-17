import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Appointment } from "../types/appointment";

export const randevuKeys = {
  all: (branchId: string) => ["randevu", branchId] as const,
  day: (branchId: string, date: string, doctorId?: string) =>
    [...randevuKeys.all(branchId), "day", date, doctorId ?? ""] as const,
};

// List appointments for a given local-day. Backend accepts YYYY-MM-DD and
// treats `to` exclusive end-of-day, so passing the same date returns one day.
export function randevuDayOptions(branchId: string, date: string, doctorId?: string) {
  return queryOptions({
    queryKey: randevuKeys.day(branchId, date, doctorId),
    queryFn: () => {
      const params = new URLSearchParams();
      params.set("from", date);
      params.set("to", date);
      if (doctorId) params.set("doctor_id", doctorId);
      return api().get<Appointment[]>(`/api/appointments?${params.toString()}`);
    },
    enabled: !!branchId && !!date,
  });
}
