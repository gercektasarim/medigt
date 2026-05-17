import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type { ServiceCatalogItem, ServiceCategory, ServicePrice } from "../types/service";

export type HizmetFilter = {
  category?: ServiceCategory;
  activeOnly?: boolean;
  search?: string;
};

export const hizmetKeys = {
  all: (orgId: string) => ["hizmet", orgId] as const,
  list: (orgId: string, filter: HizmetFilter) => [...hizmetKeys.all(orgId), "list", filter] as const,
  prices: (orgId: string, serviceId: string) => [...hizmetKeys.all(orgId), "prices", serviceId] as const,
};

export function hizmetListOptions(orgId: string, filter: HizmetFilter = {}) {
  return queryOptions({
    queryKey: hizmetKeys.list(orgId, filter),
    queryFn: () => {
      const params = new URLSearchParams();
      if (filter.category) params.set("category", filter.category);
      if (filter.activeOnly) params.set("active", "true");
      if (filter.search) params.set("q", filter.search);
      const qs = params.toString();
      return api().get<ServiceCatalogItem[]>(`/api/services${qs ? `?${qs}` : ""}`);
    },
    enabled: !!orgId,
  });
}

export function hizmetPricesOptions(orgId: string, serviceId: string) {
  return queryOptions({
    queryKey: hizmetKeys.prices(orgId, serviceId),
    queryFn: () => api().get<ServicePrice[]>(`/api/services/${encodeURIComponent(serviceId)}/prices`),
    enabled: !!orgId && !!serviceId,
  });
}
