import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { InpatientBoardRow } from "../types/nursing";

export const hemsireKeys = {
  all: (branchId: string) => ["hemsire", branchId] as const,
  board: (branchId: string, wardId: string) =>
    [...hemsireKeys.all(branchId), "board", wardId] as const,
};

export function hemsireBoardOptions(branchId: string, wardId?: string) {
  return queryOptions({
    queryKey: hemsireKeys.board(branchId, wardId ?? ""),
    queryFn: () => {
      const params = new URLSearchParams();
      if (wardId) params.set("ward_id", wardId);
      const qs = params.toString();
      return api().get<InpatientBoardRow[]>(`/api/inpatient-board${qs ? `?${qs}` : ""}`);
    },
    enabled: !!branchId,
    refetchInterval: 60_000, // nurses watch this all day — refresh every 60s
  });
}
