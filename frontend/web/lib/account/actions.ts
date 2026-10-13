"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";
import { apiFetch, ApiError } from "@/lib/api/server";
import type { ApiAddress, ApiTokens } from "@/lib/api/types";
import { isLocale, defaultLocale, type Locale } from "@/lib/i18n/config";
import { getDictionary } from "@/lib/i18n/get-dictionary";
import { getAccessToken, setSession } from "@/lib/auth/session";
import type { FormState } from "@/lib/auth/actions";

export interface AddressFields {
  title: string;
  fullName: string;
  phone: string;
  line1: string;
  line2: string;
  district: string;
  city: string;
  postalCode: string;
  isDefault: boolean;
}

export type AddressResult = { ok: true; id: string } | { ok: false; message: string };

function localeOf(raw: string): Locale {
  return isLocale(raw) ? raw : defaultLocale;
}

function body(f: AddressFields) {
  return {
    title: f.title.trim(),
    full_name: f.fullName.trim(),
    phone: f.phone.trim(),
    line1: f.line1.trim(),
    line2: f.line2.trim(),
    district: f.district.trim(),
    city: f.city.trim(),
    postal_code: f.postalCode.trim(),
    is_default: f.isDefault,
  };
}

/** Creates (no id) or updates an address of the signed-in customer. */
export async function saveAddressAction(localeRaw: string, id: string | null, fields: AddressFields): Promise<AddressResult> {
  const locale = localeOf(localeRaw);
  const d = await getDictionary(locale);
  const token = await getAccessToken();
  if (!token) redirect(`/${locale}/login`);
  try {
    const saved = id
      ? await apiFetch<ApiAddress>(`/users/me/addresses/${encodeURIComponent(id)}`, { method: "PATCH", token, body: body(fields) })
      : await apiFetch<ApiAddress>("/users/me/addresses", { method: "POST", token, body: body(fields) });
    revalidatePath(`/${locale}/account`, "layout");
    return { ok: true, id: saved.id };
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) return { ok: false, message: d.account.addresses.limit };
    return { ok: false, message: d.account.addresses.error };
  }
}

export async function deleteAddressAction(localeRaw: string, id: string): Promise<{ ok: boolean }> {
  const locale = localeOf(localeRaw);
  const token = await getAccessToken();
  if (!token) return { ok: false };
  try {
    await apiFetch(`/users/me/addresses/${encodeURIComponent(id)}`, { method: "DELETE", token });
    revalidatePath(`/${locale}/account`, "layout");
    return { ok: true };
  } catch {
    return { ok: false };
  }
}

export async function setDefaultAddressAction(localeRaw: string, id: string): Promise<{ ok: boolean }> {
  const locale = localeOf(localeRaw);
  const token = await getAccessToken();
  if (!token) return { ok: false };
  try {
    await apiFetch(`/users/me/addresses/${encodeURIComponent(id)}/default`, { method: "POST", token });
    revalidatePath(`/${locale}/account`, "layout");
    return { ok: true };
  } catch {
    return { ok: false };
  }
}

async function credentialError(e: unknown, locale: Locale): Promise<string> {
  const d = await getDictionary(locale);
  const t = d.account.profile.errors;
  if (e instanceof ApiError) {
    if (e.status === 429) return t.tooMany;
    if (e.status === 409) return t.emailTaken;
    if (e.status === 422 || e.status === 400) {
      if (/current password/i.test(e.message)) return t.wrongPassword;
      if (/differ from the current one/i.test(e.message)) return /email/i.test(e.message) ? t.emailUnchanged : t.samePassword;
      return t.invalid;
    }
  }
  return d.auth.errors.generic;
}

/** Changes the password. Every other session is signed out; this one continues with the new tokens. */
export async function changePasswordAction(_prev: FormState, formData: FormData): Promise<FormState> {
  const locale = localeOf(String(formData.get("locale") ?? ""));
  const token = await getAccessToken();
  if (!token) redirect(`/${locale}/login`);
  try {
    const tokens = await apiFetch<ApiTokens>("/users/me/password", {
      method: "POST",
      token,
      body: { current_password: String(formData.get("current_password") ?? ""), new_password: String(formData.get("new_password") ?? "") },
    });
    await setSession(tokens);
  } catch (e) {
    return { error: await credentialError(e, locale) };
  }
  return { saved: true };
}

export async function changeEmailAction(_prev: FormState, formData: FormData): Promise<FormState> {
  const locale = localeOf(String(formData.get("locale") ?? ""));
  const token = await getAccessToken();
  if (!token) redirect(`/${locale}/login`);
  try {
    await apiFetch("/users/me/email", {
      method: "POST",
      token,
      body: { email: String(formData.get("email") ?? "").trim(), password: String(formData.get("password") ?? "") },
    });
  } catch (e) {
    return { error: await credentialError(e, locale) };
  }
  revalidatePath(`/${locale}/account`, "layout");
  return { saved: true };
}
