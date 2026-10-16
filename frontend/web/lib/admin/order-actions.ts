"use server";

import { revalidatePath } from "next/cache";
import { ApiError } from "@/lib/api/server";
import { requireAdmin } from "@/lib/auth/session";
import { deliverAdminOrder, shipAdminOrder } from "@/lib/data/admin-orders";
import { defaultLocale, isLocale } from "@/lib/i18n/config";
import { getDictionary } from "@/lib/i18n/get-dictionary";

export type AdminOrderActionState = { status: "idle" } | { status: "ok" } | { status: "error"; message: string };

const MAX_FIELD = 80;

function str(form: FormData, key: string): string {
  const v = form.get(key);
  return typeof v === "string" ? v.trim() : "";
}

/** confirmed -> shipped. Re-checks the admin role on every call: server actions are public endpoints. */
export async function shipOrderAction(_prev: AdminOrderActionState, form: FormData): Promise<AdminOrderActionState> {
  const rawLocale = str(form, "locale");
  const locale = isLocale(rawLocale) ? rawLocale : defaultLocale;
  const orderId = str(form, "orderId");
  const { token } = await requireAdmin(locale, `/${locale}/admin/orders/${orderId}`);
  const m = (await getDictionary(locale)).admin.orders.messages;

  const carrier = str(form, "carrier");
  const trackingCode = str(form, "trackingCode");
  if (!orderId) return { status: "error", message: m.generic };
  if (!carrier || !trackingCode || carrier.length > MAX_FIELD || trackingCode.length > MAX_FIELD) {
    return { status: "error", message: m.shipInvalid };
  }

  try {
    await shipAdminOrder(token, orderId, { carrier, tracking_code: trackingCode });
  } catch (e) {
    return { status: "error", message: errorMessage(e, m) };
  }
  revalidatePath(`/${locale}/admin/orders`);
  revalidatePath(`/${locale}/admin/orders/${orderId}`);
  revalidatePath(`/${locale}/admin`);
  return { status: "ok" };
}

/** shipped -> delivered. */
export async function deliverOrderAction(_prev: AdminOrderActionState, form: FormData): Promise<AdminOrderActionState> {
  const rawLocale = str(form, "locale");
  const locale = isLocale(rawLocale) ? rawLocale : defaultLocale;
  const orderId = str(form, "orderId");
  const { token } = await requireAdmin(locale, `/${locale}/admin/orders/${orderId}`);
  const m = (await getDictionary(locale)).admin.orders.messages;
  if (!orderId) return { status: "error", message: m.generic };

  try {
    await deliverAdminOrder(token, orderId);
  } catch (e) {
    return { status: "error", message: errorMessage(e, m) };
  }
  revalidatePath(`/${locale}/admin/orders`);
  revalidatePath(`/${locale}/admin/orders/${orderId}`);
  revalidatePath(`/${locale}/admin`);
  return { status: "ok" };
}

function errorMessage(e: unknown, m: { wrongState: string; shipInvalid: string; generic: string }): string {
  if (e instanceof ApiError) {
    if (e.status === 409) return m.wrongState;
    if (e.status === 422) return m.shipInvalid;
  }
  return m.generic;
}
