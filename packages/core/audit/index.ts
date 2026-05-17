"use client";

import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";

// Audit log viewer — KVKK requirement. The org admin can browse the
// access trail filtered by actor / action / entity / date range.

export type AuditEntry = {
  id: number;
  organization_id?: string;
  branch_id?: string;
  actor_user_id?: string;
  actor_email?: string;
  actor_name?: string;
  action: string;
  entity_type: string;
  entity_id?: string;
  details: Record<string, unknown>;
  ip_address?: string;
  user_agent?: string;
  created_at: string;
};

export type AuditPage = {
  total: number;
  limit: number;
  offset: number;
  items: AuditEntry[];
};

export type AuditFacets = {
  actions: string[];
  entity_types: string[];
};

export type AuditFilter = {
  branch_id?: string;
  actor_user_id?: string;
  action?: string;
  entity_type?: string;
  entity_id?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
};

function toQuery(f: AuditFilter): string {
  const sp = new URLSearchParams();
  if (f.branch_id) sp.set("branch_id", f.branch_id);
  if (f.actor_user_id) sp.set("actor_user_id", f.actor_user_id);
  if (f.action) sp.set("action", f.action);
  if (f.entity_type) sp.set("entity_type", f.entity_type);
  if (f.entity_id) sp.set("entity_id", f.entity_id);
  if (f.from) sp.set("from", f.from);
  if (f.to) sp.set("to", f.to);
  if (f.limit != null) sp.set("limit", String(f.limit));
  if (f.offset != null) sp.set("offset", String(f.offset));
  const s = sp.toString();
  return s ? `?${s}` : "";
}

export const auditKeys = {
  all: (orgId: string) => ["audit", orgId] as const,
  list: (orgId: string, f: AuditFilter) =>
    [...auditKeys.all(orgId), "list", f] as const,
  facets: (orgId: string) => [...auditKeys.all(orgId), "facets"] as const,
};

export function auditListOptions(orgId: string, filter: AuditFilter) {
  return queryOptions({
    queryKey: auditKeys.list(orgId, filter),
    queryFn: () => api().get<AuditPage>(`/api/audit-log${toQuery(filter)}`),
    enabled: !!orgId,
  });
}

export function auditFacetsOptions(orgId: string) {
  return queryOptions({
    queryKey: auditKeys.facets(orgId),
    queryFn: () => api().get<AuditFacets>(`/api/audit-log/facets`),
    enabled: !!orgId,
  });
}
