import { NextResponse, type NextRequest } from "next/server";
import { LOCALE_COOKIE, defaultLocale, isLocale, locales } from "@/lib/i18n/config";

function preferredLocale(request: NextRequest) {
  const cookie = request.cookies.get(LOCALE_COOKIE)?.value;
  if (isLocale(cookie)) return cookie;
  const accept = request.headers.get("accept-language")?.toLowerCase() ?? "";
  const first = accept.split(",")[0]?.trim().slice(0, 2);
  return isLocale(first) ? first : defaultLocale;
}

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasLocale = locales.some((l) => pathname === `/${l}` || pathname.startsWith(`/${l}/`));

  if (!hasLocale) {
    const url = request.nextUrl.clone();
    url.pathname = `/${preferredLocale(request)}${pathname === "/" ? "" : pathname}`;
    return NextResponse.redirect(url);
  }

  const res = NextResponse.next();
  res.cookies.set(LOCALE_COOKIE, pathname.split("/")[1], { path: "/", maxAge: 60 * 60 * 24 * 365 });
  return res;
}

export const config = {
  matcher: ["/((?!_next|media|favicon.ico|.*\\..*).*)"],
};
