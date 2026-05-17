// Modal registry — Zustand-based. Each app's <ModalRegistry /> reads from
// this store and renders the matching modal component.

import { create } from "zustand";

export type ModalKey =
  | "create-hasta"
  | "create-randevu"
  | "create-yatis"
  | "kabul-tipi-secimi"
  | "taburcu-confirm"
  | "ilac-etkilesim-uyari"
  | "kasa-acma"
  | "kasa-kapama"
  | "create-hospital"
  | "switch-branch";

type ModalState = {
  active: { key: ModalKey; data?: unknown } | null;
  open: (key: ModalKey, data?: unknown) => void;
  close: () => void;
};

export const useModalStore = create<ModalState>((set) => ({
  active: null,
  open: (key, data) => set({ active: { key, data } }),
  close: () => set({ active: null }),
}));
