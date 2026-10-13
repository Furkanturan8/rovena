import "server-only";
import { cache } from "react";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api/server";
import type { ApiAuthResult, ApiTokens, ApiUser } from "@/lib/api/types";
import type { Locale } from "@/lib/i18n/config";
import { ACCESS_COOKIE, REFRESH_COOKIE, cookieOptions } from "./cookies";

/** Stores a token pair. Only callable where cookies are writable: server actions and route handlers. */
export async function setSession(tokens: ApiTokens) {
  const jar = await cookies();
  jar.set(ACCESS_COOKIE, tokens.access_token, cookieOptions(tokens.access_expires_at));
  jar.set(REFRESH_COOKIE, tokens.refresh_token, cookieOptions(tokens.refresh_expires_at));
}

export async function clearSession() {
  const jar = await cookies();
  jar.delete(ACCESS_COOKIE);
  jar.delete(REFRESH_COOKIE);
}

/** The access token of the signed-in customer. The proxy renews it from the refresh cookie before a page renders. */
export async function getAccessToken(): Promise<string | null> {
  return (await cookies()).get(ACCESS_COOKIE)?.value ?? null;
}

/** The signed-in user, or null. One identity call per request, however many components ask. */
export const getCurrentUser = cache(async (): Promise<ApiUser | null> => {
  const token = await getAccessToken();
  if (!token) return null;
  try {
    return await apiFetch<ApiUser>("/users/me", { token });
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null;
    throw e;
  }
});

/** For pages that need a customer: sends visitors to the sign-in page and back. */
export async function requireUser(locale: Locale, next: string): Promise<{ user: ApiUser; token: string }> {
  const token = await getAccessToken();
  const user = await getCurrentUser();
  if (!token || !user) redirect(`/${locale}/login?next=${encodeURIComponent(next)}`);
  return { user, token };
}

export type { ApiAuthResult };
