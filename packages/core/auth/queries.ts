import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Branch, Organization } from "../types/hospital";
import type { User } from "../types/user";

export const meKeys = {
  all: () => ["me"] as const,
};

export type MeResponse = {
  user: User;
  organizations: Organization[];
  branches: Branch[];
};

export function meOptions(enabled = true) {
  return queryOptions({
    queryKey: meKeys.all(),
    queryFn: () => api().get<MeResponse>("/api/me"),
    enabled,
    staleTime: 60 * 1000,
    retry: false,
  });
}
