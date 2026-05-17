"use client";

// The single place in the web app where next/navigation is imported.
// Everything else in packages/views/ uses useNavigation() from @medigt/core.

import { useRouter, usePathname } from "next/navigation";
import { useMemo } from "react";
import type { NavigationAdapter } from "@medigt/core/navigation";

export function useNextNavigationAdapter(): NavigationAdapter {
  const router = useRouter();
  const pathname = usePathname();

  return useMemo<NavigationAdapter>(() => ({
    push: (path) => router.push(path),
    replace: (path) => router.replace(path),
    back: () => router.back(),
    pathname: pathname ?? "/",
  }), [router, pathname]);
}
