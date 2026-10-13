"use client";

import { createContext, useContext } from "react";
import type { Locale } from "./config";
import { tr, type Dictionary } from "./dictionaries/tr";
import { en } from "./dictionaries/en";

// Sözlükler fonksiyon içerdiği için sunucudan istemciye prop olarak geçirilemez; istemci tarafı kendi kopyasını seçer.
const dictionaries: Record<Locale, Dictionary> = { tr, en };

interface I18nValue {
  locale: Locale;
  d: Dictionary;
}

const I18nContext = createContext<I18nValue | null>(null);

export function I18nProvider({ locale, children }: { locale: Locale; children: React.ReactNode }) {
  return <I18nContext.Provider value={{ locale, d: dictionaries[locale] }}>{children}</I18nContext.Provider>;
}

function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used within I18nProvider");
  return ctx;
}

export const useDict = () => useI18n().d;
export const useLocale = () => useI18n().locale;
