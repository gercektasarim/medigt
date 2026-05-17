import { api } from "../api/client";
import type { User } from "../types/user";

export type SendCodeInput = { email: string };
export type SendCodeResult = { sent: boolean };

export function sendLoginCode(input: SendCodeInput): Promise<SendCodeResult> {
  return api().post<SendCodeResult>("/api/auth/send-code", input);
}

export type VerifyCodeInput = { email: string; code: string };
export type VerifyCodeResult = {
  access_token: string;
  refresh_token: string;
  user: User;
  is_new_user: boolean;
};

export function verifyLoginCode(input: VerifyCodeInput): Promise<VerifyCodeResult> {
  return api().post<VerifyCodeResult>("/api/auth/verify-code", input);
}

export type RefreshInput = { refresh_token: string };
export type RefreshResult = { access_token: string; user: User };

export function refreshAccessToken(input: RefreshInput): Promise<RefreshResult> {
  return api().post<RefreshResult>("/api/auth/refresh", input);
}

export function logout(): Promise<void> {
  return api().post<void>("/api/auth/logout");
}
