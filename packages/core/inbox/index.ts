"use client";

import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";

// Inbox — aggregates actionable items across the user's domain. Backend
// merges multiple sources (unsigned prescriptions, critical lab results,
// dead Medula outbox rows) and returns a uniform list so the UI can
// render one timeline.

export type InboxKind =
  | "prescription.unsigned"
  | "lab.critical"
  | "medula.dead";

export type InboxSeverity = "info" | "warning" | "critical";

export type InboxItem = {
  id: string;
  kind: InboxKind;
  title: string;
  subtitle?: string;
  severity: InboxSeverity;
  ref: string;
  ref_url: string;
  occurred_at: string;
};

export const inboxKeys = {
  list: (branchId: string) => ["inbox", branchId] as const,
};

export function inboxListOptions(branchId: string) {
  return queryOptions({
    queryKey: inboxKeys.list(branchId),
    queryFn: () => api().get<InboxItem[]>("/api/inbox?limit=100"),
    enabled: !!branchId,
    // Inbox is a poll source — invalidate every 30s so freshly published
    // lab results / signed prescriptions surface without a manual reload.
    refetchInterval: 30_000,
  });
}
