import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Doctor } from "../types/people";

export const doktorKeys = {
  all: (orgId: string) => ["doktor", orgId] as const,
  list: (orgId: string) => [...doktorKeys.all(orgId), "list"] as const,
};

export function doktorListOptions(orgId: string) {
  return queryOptions({
    queryKey: doktorKeys.list(orgId),
    queryFn: () => api().get<Doctor[]>("/api/doctors"),
    enabled: !!orgId,
  });
}
