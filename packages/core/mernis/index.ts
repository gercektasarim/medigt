"use client";

import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { MernisLogRow, MernisVerifyInput, MernisVerifyResult } from "../types/integration";

export const mernisKeys = {
  all: (orgId: string) => ["mernis", orgId] as const,
  logs: (orgId: string) => [...mernisKeys.all(orgId), "logs"] as const,
};

export function mernisLogsOptions(orgId: string) {
  return queryOptions({
    queryKey: mernisKeys.logs(orgId),
    queryFn: () => api().get<MernisLogRow[]>("/api/mernis/logs"),
    enabled: !!orgId,
  });
}

export function verifyMernis(input: MernisVerifyInput): Promise<MernisVerifyResult> {
  return api().post<MernisVerifyResult>("/api/mernis/verify", input);
}
