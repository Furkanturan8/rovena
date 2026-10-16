"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";
import { apiFetch, ApiError } from "@/lib/api/server";
import type { ApiAuthResult, ApiUser } from "@/lib/api/types";
import { isLocale, defaultLocale, type Locale } from "@/lib/i18n/config";
import { getDictionary } from "@/lib/i18n/get-dictionary";
import { REFRESH_COOKIE } from "./cookies";
import { clearSession, getAccessToken, setSession } from "./session";

export interface FormState {
  error?: string;
  saved?: boolean;
}

function localeOf(formData: FormData): Locale {
  const v = String(formData.get("locale") ?? "");
  return isLocale(v) ? v : defaultLocale;
}

/** Only same-site paths are accepted as a post-login destination (no open redirects). */
function safeNext(raw: FormDataEntryValue | null, fallback: string): string {
  const next = typeof raw === "string" ? raw : "";
  return next.startsWith("/") && !next.startsWith("//") ? next : fallback;
}

async function errorText(e: unknown, locale: Locale, kind: "login" | "register"): Promise<string> {
  const d = await getDictionary(locale);
  if (e instanceof ApiError) {
    if (e.status === 401) return d.auth.errors.invalidCredentials;
    if (e.status === 409) return d.auth.errors.emailTaken;
    if (e.status === 422 || e.status === 400) return kind === "register" ? d.auth.errors.invalidDetails : d.auth.errors.invalidCredentials;
    if (e.status === 429) return d.auth.errors.tooManyAttempts;
  }
  return d.auth.errors.generic;
}

export async function loginAction(_prev: FormState, formData: FormData): Promise<FormState> {
  const locale = localeOf(formData);
  let destination: string;
  try {
    const res = await apiFetch<ApiAuthResult>("/auth/login", {
      method: "POST",
      body: { email: String(formData.get("email") ?? "").trim(), password: String(formData.get("password") ?? "") },
    });
    await setSession(res.tokens);
    const home = res.user.role === "admin" ? `/${locale}/admin` : `/${locale}/account`;
    destination = safeNext(formData.get("next"), home);
  } catch (e) {
    return { error: await errorText(e, locale, "login") };
  }
  redirect(destination);
}

export async function registerAction(_prev: FormState, formData: FormData): Promise<FormState> {
  const locale = localeOf(formData);
  let destination: string;
  try {
    const res = await apiFetch<ApiAuthResult>("/auth/register", {
      method: "POST",
      body: {
        email: String(formData.get("email") ?? "").trim(),
        password: String(formData.get("password") ?? ""),
        first_name: String(formData.get("first_name") ?? "").trim(),
        last_name: String(formData.get("last_name") ?? "").trim(),
      },
    });
    await setSession(res.tokens);
    destination = safeNext(formData.get("next"), `/${locale}/account`);
  } catch (e) {
    return { error: await errorText(e, locale, "register") };
  }
  redirect(destination);
}

export async function logoutAction(formData: FormData) {
  const locale = localeOf(formData);
  const refresh = (await cookies()).get(REFRESH_COOKIE)?.value;
  if (refresh) {
    // Revoking the refresh token is best effort; the cookies are cleared either way.
    await apiFetch("/auth/logout", { method: "POST", body: { refresh_token: refresh } }).catch(() => undefined);
  }
  await clearSession();
  // Admin sign-out sends the admin back to the login page; everyone else lands on the storefront.
  redirect(formData.get("to") === "login" ? `/${locale}/login` : `/${locale}`);
}

export async function updateProfileAction(_prev: FormState, formData: FormData): Promise<FormState> {
  const locale = localeOf(formData);
  const token = await getAccessToken();
  if (!token) redirect(`/${locale}/login`);
  const d = await getDictionary(locale);
  try {
    await apiFetch<ApiUser>("/users/me", {
      method: "PATCH",
      token,
      body: { first_name: String(formData.get("first_name") ?? "").trim(), last_name: String(formData.get("last_name") ?? "").trim() },
    });
  } catch {
    return { error: d.auth.errors.generic };
  }
  revalidatePath(`/${locale}/account`, "layout");
  return { saved: true };
}
