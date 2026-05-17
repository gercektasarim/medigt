"use client";

// CoreProvider — single entry point each app wraps its root with.
// Configures the API client, mounts QueryClient, opens WS, and wires the
// Page Visibility API invalidation that mitigates Caliptic's P0 cache-staleness bug.

import { useEffect, useRef, type ReactNode } from "react";
import { QueryClientProvider, useQueryClient } from "@tanstack/react-query";
import { queryClient } from "../query-client";
import { configureApi, ApiClient } from "../api/client";
import { WsClient, type WsMessage } from "../api/ws-client";
import { getAccessToken, useAuthStore } from "../auth/store";
import { getCurrentBranchId, getCurrentOrgId } from "../hospital/store";
import { NavigationProvider, type NavigationAdapter } from "../navigation/adapter";
import { configureStorage, type StorageAdapter } from "./storage";
import { logger } from "../logger";

export type CoreProviderProps = {
  apiBaseUrl: string;
  wsUrl: string;
  storage: StorageAdapter;
  navigation: NavigationAdapter;
  onWsMessage?: (msg: WsMessage) => void;
  children: ReactNode;
};

let _api: ApiClient | null = null;

export function CoreProvider({ apiBaseUrl, wsUrl, storage, navigation, onWsMessage, children }: CoreProviderProps) {
  configureStorage(storage);

  if (!_api) {
    _api = configureApi({ baseUrl: apiBaseUrl });
    _api.setAuthGetter(getAccessToken);
    _api.setOrgGetter(getCurrentOrgId);
    _api.setBranchGetter(getCurrentBranchId);
    _api.setOnUnauthorized(() => {
      useAuthStore.getState().logout();
    });
  }

  return (
    <QueryClientProvider client={queryClient}>
      <NavigationProvider adapter={navigation}>
        <WsAndVisibilityBridge wsUrl={wsUrl} onMessage={onWsMessage} />
        {children}
      </NavigationProvider>
    </QueryClientProvider>
  );
}

function WsAndVisibilityBridge({ wsUrl, onMessage }: { wsUrl: string; onMessage?: (msg: WsMessage) => void }) {
  const qc = useQueryClient();
  const wsRef = useRef<WsClient | null>(null);

  useEffect(() => {
    const ws = new WsClient({
      url: wsUrl,
      getAccessToken,
      onMessage: (msg) => {
        onMessage?.(msg);
      },
      onReconnect: () => {
        logger.info("WS reconnected — invalidating all queries");
        void qc.invalidateQueries();
      },
    });
    wsRef.current = ws;
    ws.connect();

    const onVisibility = () => {
      if (document.visibilityState === "visible") {
        logger.info("Page visible — invalidating active queries");
        void qc.invalidateQueries({ type: "active" });
        ws.connect();
      }
    };
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      document.removeEventListener("visibilitychange", onVisibility);
      ws.close();
      wsRef.current = null;
    };
  }, [wsUrl, qc, onMessage]);

  return null;
}
