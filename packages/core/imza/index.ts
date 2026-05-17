"use client";

import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { DigitalSignature } from "../types";

export const imzaKeys = {
  all: () => ["imza"] as const,
  mine: () => [...imzaKeys.all(), "mine"] as const,
  detail: (id: string) => [...imzaKeys.all(), "detail", id] as const,
};

// ---------- Queries ----------

export function mySignaturesOptions() {
  return queryOptions({
    queryKey: imzaKeys.mine(),
    queryFn: () => api().get<DigitalSignature[]>("/api/signatures/mine"),
  });
}

export function signatureOptions(id: string) {
  return queryOptions({
    queryKey: imzaKeys.detail(id),
    queryFn: () => api().get<DigitalSignature>(`/api/signatures/${encodeURIComponent(id)}`),
    enabled: !!id,
  });
}

// ---------- Mutations ----------

export type InitSignInput = {
  target_table: string;
  target_id: string;
  document_kind: string;
  // Caller hashes the document client-side (SHA-256 hex) OR sends raw bytes.
  document_hash?: string;
  document_bytes?: string; // base64 — for small docs
};

export function initSignature(input: InitSignInput): Promise<DigitalSignature> {
  return api().post<DigitalSignature>("/api/signatures", input);
}

export function pollSignature(id: string): Promise<DigitalSignature> {
  return api().post<DigitalSignature>(`/api/signatures/${encodeURIComponent(id)}/poll`, {});
}

export function cancelSignature(id: string): Promise<{ ok: boolean }> {
  return api().post<{ ok: boolean }>(`/api/signatures/${encodeURIComponent(id)}/cancel`, {});
}

// ---------- Helpers ----------

/** SHA-256 hex of a string, computed via Web Crypto. Use for canonical
 *  document hashing on the client; server re-derives when document_bytes
 *  is provided. */
export async function sha256Hex(text: string): Promise<string> {
  const buf = new TextEncoder().encode(text);
  const digest = await crypto.subtle.digest("SHA-256", buf);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}
