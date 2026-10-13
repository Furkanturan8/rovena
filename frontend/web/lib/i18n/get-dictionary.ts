import "server-only";
import type { Locale } from "./config";
import type { Dictionary } from "./dictionaries/tr";

const dictionaries: Record<Locale, () => Promise<Dictionary>> = {
  tr: () => import("./dictionaries/tr").then((m) => m.tr),
  en: () => import("./dictionaries/en").then((m) => m.en),
};

export const getDictionary = (locale: Locale) => dictionaries[locale]();
