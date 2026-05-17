"use client";

import type { ReactNode } from "react";
import { AppSidebar } from "./app-sidebar";

export function DashboardLayout({ children, extra }: { children: ReactNode; extra?: ReactNode }) {
  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background text-foreground">
      <AppSidebar />
      <main className="flex flex-1 flex-col overflow-y-auto">{children}</main>
      {extra}
    </div>
  );
}
