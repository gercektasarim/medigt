"use client";

import { useModalStore } from "@medigt/core/modals";

// Each modal feature registers a component here as it lands.
// Pattern: open via useModalStore.getState().open("create-hasta", { ... });
// close via useModalStore.getState().close().

export function ModalRegistry() {
  const active = useModalStore((s) => s.active);
  if (!active) return null;

  switch (active.key) {
    // case "create-hasta": return <CreateHastaModal data={active.data} />;
    // case "create-randevu": return <CreateRandevuModal data={active.data} />;
    default:
      return null;
  }
}
