export type RealtimeScope =
  | "organization"
  | "branch"
  | "user"
  | "patient"
  | "bed_map"
  | "cash_register"
  | "lab_order";

export type RealtimeEventType =
  | "appointment:created"
  | "appointment:status_changed"
  | "visit:created"
  | "visit:status_changed"
  | "lab_result:new"
  | "lab_result:critical"
  | "lab_result:verified"
  | "prescription:signed"
  | "prescription:dispensed"
  | "bed:status_changed"
  | "admission:created"
  | "admission:discharged"
  | "vital_signs:abnormal"
  | "cash_movement:new"
  | "cash_register:opened"
  | "cash_register:closed"
  | "medula:provision:completed"
  | "medula:invoice:rejected"
  | "radiology_report:signed";

export type RealtimeEvent = {
  type: RealtimeEventType;
  scope: RealtimeScope;
  scope_id: string;
  payload: Record<string, unknown>;
  ts: number;
};
