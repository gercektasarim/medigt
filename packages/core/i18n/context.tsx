"use client";

import { createContext, useContext, useState, type ReactNode } from "react";
import { tr } from "./locales/tr";
import type { Dictionary, Locale } from "./types";

const DICTS: Record<Locale, Dictionary> = {
  tr,
  en: tr, // fallback until EN translations are provided
};

type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: Dictionary;
};

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children, defaultLocale = "tr" }: { children: ReactNode; defaultLocale?: Locale }) {
  const [locale, setLocale] = useState<Locale>(defaultLocale);
  const t = DICTS[locale];
  return <I18nContext.Provider value={{ locale, setLocale, t }}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used within I18nProvider");
  return ctx;
}

export function useT(): Dictionary {
  return useI18n().t;
}
