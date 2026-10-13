export const locales = ["tr", "en"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "tr";
export const LOCALE_COOKIE = "NEXT_LOCALE";

export const isLocale = (v: string | undefined): v is Locale => !!v && (locales as readonly string[]).includes(v);

/** /tr/collection?x=1 → { locale: "tr", rest: "/collection" } */
export function splitLocale(pathname: string): { locale: Locale; rest: string } {
  const [, first, ...tail] = pathname.split("/");
  if (isLocale(first)) return { locale: first, rest: "/" + tail.join("/") };
  return { locale: defaultLocale, rest: pathname };
}

export function withLocale(locale: Locale, href: string): string {
  if (!href.startsWith("/") || href.startsWith("//")) return href;
  const pathPart = href.split(/[?#]/)[0];
  const suffix = href.slice(pathPart.length);
  const { rest } = splitLocale(pathPart);
  return `/${locale}${rest === "/" ? "" : rest}${suffix}`;
}
