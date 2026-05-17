"use client";

// NavigationAdapter — apps (Next.js / Electron / future RN) provide a concrete
// implementation that bridges to their router. Shared code in packages/views/*
// uses useNavigation() and AppLink and never imports next/* or react-router.

import { createContext, useContext, type ReactNode } from "react";

export type NavigationAdapter = {
  push: (path: string) => void;
  replace: (path: string) => void;
  back: () => void;
  pathname: string;
};

const NavigationContext = createContext<NavigationAdapter | null>(null);

export function NavigationProvider({
  adapter,
  children,
}: {
  adapter: NavigationAdapter;
  children: ReactNode;
}) {
  return <NavigationContext.Provider value={adapter}>{children}</NavigationContext.Provider>;
}

export function useNavigation(): NavigationAdapter {
  const ctx = useContext(NavigationContext);
  if (!ctx) throw new Error("useNavigation must be used within NavigationProvider");
  return ctx;
}

export function AppLink({
  to,
  children,
  className,
  onClick,
}: {
  to: string;
  children: ReactNode;
  className?: string;
  onClick?: () => void;
}) {
  const nav = useNavigation();
  return (
    <a
      href={to}
      className={className}
      onClick={(e) => {
        if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
        e.preventDefault();
        onClick?.();
        nav.push(to);
      }}
    >
      {children}
    </a>
  );
}
