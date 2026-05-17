import { api } from "../api/client";
import type { Appointment, AppointmentStatus, VisitKind } from "../types/appointment";

export type CreateRandevuInput = {
  patient_id: string;
  doctor_id?: string;
  department_id?: string;
  scheduled_at: string;       // ISO 8601 with timezone
  duration_minutes?: number;
  kind?: VisitKind;
  reason?: string;
  notes?: string;
};

export function createRandevu(input: CreateRandevuInput): Promise<Appointment> {
  return api().post<Appointment>("/api/appointments", input);
}

export function updateRandevuStatus(id: string, status: Exclude<AppointmentStatus, "scheduled" | "cancelled">): Promise<Appointment> {
  return api().post<Appointment>(`/api/appointments/${encodeURIComponent(id)}/status`, { status });
}

export function cancelRandevu(id: string, reason?: string): Promise<void> {
  return api().post<void>(`/api/appointments/${encodeURIComponent(id)}/cancel`, { reason: reason ?? "" });
}
