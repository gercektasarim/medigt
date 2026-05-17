import type { Timestamps, Uuid } from "./common";

export type User = {
  id: Uuid;
  email: string;
  name: string;
  phone?: string;
  avatar_url?: string;
  totp_enabled: boolean;
  national_id_last4?: string;
} & Timestamps;

export type AuthSession = {
  user: User;
  access_token: string;
  refresh_token: string;
  expires_at: string;
};
