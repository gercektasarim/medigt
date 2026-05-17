"use client";

import type { ReactNode } from "react";
import { CoreProvider } from "@medigt/core/provider";
import { I18nProvider } from "@medigt/core/i18n";
import { webStorage } from "./storage";
import { useNextNavigationAdapter } from "./navigation";
import { AuthBootstrap } from "./auth-bootstrap";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "";
const WS_URL = process.env.NEXT_PUBLIC_WS_URL || (typeof window !== "undefined" ? `${window.location.origin.replace(/^http/, "ws")}/ws` : "");

export function Providers({ children }: { children: ReactNode }) {
  const navigation = useNextNavigationAdapter();
  return (
    <CoreProvider apiBaseUrl={API_URL} wsUrl={WS_URL} storage={webStorage} navigation={navigation}>
      <I18nProvider defaultLocale="tr">
        <AuthBootstrap>{children}</AuthBootstrap>
      </I18nProvider>
    </CoreProvider>
  );
}
