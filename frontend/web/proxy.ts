import { NextResponse, type NextRequest } from "next/server";
import { LOCALE_COOKIE, defaultLocale, isLocale, locales } from "@/lib/i18n/config";
import { ACCESS_COOKIE, REFRESH_COOKIE, cookieOptions, type TokenCookies } from "@/lib/auth/cookies";

const API_BASE_URL = (process.env.API_BASE_URL ?? "http://localhost:18080").replace(/\/+$/, "");

function preferredLocale(request: NextRequest) {
  const cookie = request.cookies.get(LOCALE_COOKIE)?.value;
  if (isLocale(cookie)) return cookie;
  const accept = request.headers.get("accept-language")?.toLowerCase() ?? "";
  const first = accept.split(",")[0]?.trim().slice(0, 2);
  return isLocale(first) ? first : defaultLocale;
}

/** Trades the refresh cookie for a new token pair when the short-lived access cookie has expired. */
async function renewTokens(refreshToken: string): Promise<TokenCookies | null> {
  try {
    const res = await fetch(`${API_BASE_URL}/api/v1/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
      cache: "no-store",
    });
    if (!res.ok) return null;
    const json = (await res.json()) as { success: boolean; data: TokenCookies };
    return json.success ? json.data : null;
  } catch {
    return null;
  }
}

const PROTECTED = /^\/(account|checkout|order-confirmation)(\/|$)/;

export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasLocale = locales.some((l) => pathname === `/${l}` || pathname.startsWith(`/${l}/`));

  if (!hasLocale) {
    const url = request.nextUrl.clone();
    url.pathname = `/${preferredLocale(request)}${pathname === "/" ? "" : pathname}`;
    return NextResponse.redirect(url);
  }

  // Pages for signed-in customers: visitors without any session cookie go to the sign-in page right away
  // (the pages themselves still verify the token with the identity service).
  const rest = pathname.replace(/^\/[a-z]{2}/, "");
  if (PROTECTED.test(rest) && !request.cookies.get(ACCESS_COOKIE)?.value && !request.cookies.get(REFRESH_COOKIE)?.value) {
    const url = request.nextUrl.clone();
    url.pathname = `/${pathname.split("/")[1]}/login`;
    url.search = `?next=${encodeURIComponent(pathname + request.nextUrl.search)}`;
    return NextResponse.redirect(url);
  }

  // Renew the session before the page renders, and let this very request see the new cookies.
  const refresh = request.cookies.get(REFRESH_COOKIE)?.value;
  const isDocument = request.headers.get("sec-fetch-dest") === "document" || request.headers.get("accept")?.includes("text/html");
  let renewed: TokenCookies | null = null;
  if (refresh && !request.cookies.get(ACCESS_COOKIE)?.value && isDocument) {
    renewed = await renewTokens(refresh);
  }

  let res: NextResponse;
  if (renewed) {
    const headers = new Headers(request.headers);
    const jar = request.cookies.getAll().filter((c) => c.name !== ACCESS_COOKIE && c.name !== REFRESH_COOKIE);
    headers.set(
      "cookie",
      [...jar.map((c) => `${c.name}=${c.value}`), `${ACCESS_COOKIE}=${renewed.access_token}`, `${REFRESH_COOKIE}=${renewed.refresh_token}`].join("; "),
    );
    res = NextResponse.next({ request: { headers } });
    res.cookies.set(ACCESS_COOKIE, renewed.access_token, cookieOptions(renewed.access_expires_at));
    res.cookies.set(REFRESH_COOKIE, renewed.refresh_token, cookieOptions(renewed.refresh_expires_at));
  } else {
    res = NextResponse.next();
  }
  res.cookies.set(LOCALE_COOKIE, pathname.split("/")[1], { path: "/", maxAge: 60 * 60 * 24 * 365 });
  return res;
}

export const config = {
  matcher: ["/((?!_next|media|api|favicon.ico|.*\\..*).*)"],
};
