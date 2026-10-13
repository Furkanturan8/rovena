import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import type { Locale } from "@/lib/i18n/config";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

const fmt: Record<Locale, Intl.NumberFormat> = {
  tr: new Intl.NumberFormat("tr-TR", { maximumFractionDigits: 0 }),
  en: new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 }),
};

/** tr: "168.000 ₺", en: "₺168,000" */
export function formatPrice(value: number, locale: Locale = "tr") {
  return locale === "tr" ? `${fmt.tr.format(value)} ₺` : `₺${fmt.en.format(value)}`;
}
