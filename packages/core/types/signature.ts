import type { Uuid } from "./common";

export type SignatureProvider = "turkkep" | "eturkce" | "mock";

export type SignatureStatus =
  | "pending"
  | "in_progress"
  | "signed"
  | "cancelled"
  | "failed"
  | "expired";

export type DigitalSignature = {
  id: Uuid;
  signer_user_id: Uuid;
  signer_tc: string;
  signer_full_name: string;
  target_table: string;
  target_id: Uuid;
  document_kind: string;
  document_hash: string;
  provider: SignatureProvider;
  session_id?: string;
  challenge_code?: string;
  status: SignatureStatus;
  error_message?: string;
  certificate_serial?: string;
  certificate_subject?: string;
  initiated_at: string;
  signed_at?: string;
  expires_at: string;
};
