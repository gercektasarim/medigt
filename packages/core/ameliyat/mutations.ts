import { api } from "../api/client";
import type {
  AnesthesiaType,
  OperatingRoom,
  Surgery,
  SurgeryPriority,
  SurgeryTeamMember,
} from "../types/surgery";

export type CreateOperatingRoomInput = {
  code: string;
  name: string;
  floor?: string;
  notes?: string;
};

export function createOperatingRoom(input: CreateOperatingRoomInput): Promise<OperatingRoom> {
  return api().post<OperatingRoom>("/api/operating-rooms", input);
}

export type CreateSurgeryInput = {
  patient_id: string;
  operating_room_id: string;
  primary_surgeon_id?: string;
  admission_id?: string;
  priority?: SurgeryPriority;
  procedure_name: string;
  procedure_codes?: string[];
  indication?: string;
  anesthesia_type?: AnesthesiaType;
  scheduled_at: string;       // RFC3339
  estimated_minutes?: number;
  team?: SurgeryTeamMember[];
};

export function createSurgery(input: CreateSurgeryInput): Promise<Surgery> {
  return api().post<Surgery>("/api/surgeries", input);
}

export function updateSurgeryStatus(
  id: string,
  status: "in_progress" | "completed" | "cancelled",
): Promise<Surgery> {
  return api().post<Surgery>(`/api/surgeries/${encodeURIComponent(id)}/status`, { status });
}

export type SaveOpNoteInput = {
  op_note?: string;
  complications?: string;
  blood_loss_ml?: number;
  specimen_sent?: boolean;
};

export function saveOpNote(id: string, input: SaveOpNoteInput): Promise<Surgery> {
  return api().patch<Surgery>(`/api/surgeries/${encodeURIComponent(id)}/op-note`, input);
}
