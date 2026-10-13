import "server-only";
import { notFound } from "next/navigation";
import { isLocale, type Locale } from "./config";
import { getDictionary } from "./get-dictionary";
import type { Dictionary } from "./dictionaries/tr";

/** Sayfa params'ından geçerli dili ve sözlüğü döner; geçersizse 404. */
export async function getI18n(params: Promise<{ locale: string }>): Promise<{ locale: Locale; d: Dictionary }> {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  return { locale, d: await getDictionary(locale) };
}
