"use server";

import { revalidatePath } from "next/cache";
import { ApiError } from "@/lib/api/server";
import { requireAdmin } from "@/lib/auth/session";
import { adjustAdminStock, receiveAdminStock } from "@/lib/data/admin-stock";
import { defaultLocale, isLocale } from "@/lib/i18n/config";
import { getDictionary } from "@/lib/i18n/get-dictionary";

export type AdminStockActionState = { status: "idle" } | { status: "ok" } | { status: "error"; message: string };

const MAX_REASON = 200;
const MAX_QUANTITY = 1_000_000;

type Messages = {
  invalidQuantity: string;
  invalidDelta: string;
  reasonRequired: string;
  reasonTooLong: string;
  belowReserved: string;
  notFound: string;
  generic: string;
};

function str(form: FormData, key: string): string {
  const v = form.get(key);
  return typeof v === "string" ? v.trim() : "";
}

/** Parses a whole number; NaN for anything else (decimals, empty, exponent notation). */
function wholeNumber(raw: string): number {
  return /^[+-]?\d{1,7}$/.test(raw) ? Number(raw) : NaN;
}

function done(locale: string, variantId: string): AdminStockActionState {
  revalidatePath(`/${locale}/admin/stock`);
  revalidatePath(`/${locale}/admin/stock/${variantId}`);
  revalidatePath(`/${locale}/admin`);
  return { status: "ok" };
}

/** Stock entry (goods received). Re-checks the admin role on every call: server actions are public endpoints. */
export async function receiveStockAction(_prev: AdminStockActionState, form: FormData): Promise<AdminStockActionState> {
  const rawLocale = str(form, "locale");
  const locale = isLocale(rawLocale) ? rawLocale : defaultLocale;
  const variantId = str(form, "variantId");
  const { token } = await requireAdmin(locale, `/${locale}/admin/stock/${variantId}`);
  const m = (await getDictionary(locale)).admin.stock.messages;

  if (!variantId) return { status: "error", message: m.generic };
  const quantity = wholeNumber(str(form, "quantity"));
  if (!Number.isInteger(quantity) || quantity < 1 || quantity > MAX_QUANTITY) return { status: "error", message: m.invalidQuantity };
  const reason = str(form, "reason");
  if (reason.length > MAX_REASON) return { status: "error", message: m.reasonTooLong };

  try {
    await receiveAdminStock(token, variantId, reason ? { quantity, reason } : { quantity });
  } catch (e) {
    return { status: "error", message: errorMessage(e, m) };
  }
  return done(locale, variantId);
}

/** Signed correction of on_hand with a mandatory reason. */
export async function adjustStockAction(_prev: AdminStockActionState, form: FormData): Promise<AdminStockActionState> {
  const rawLocale = str(form, "locale");
  const locale = isLocale(rawLocale) ? rawLocale : defaultLocale;
  const variantId = str(form, "variantId");
  const { token } = await requireAdmin(locale, `/${locale}/admin/stock/${variantId}`);
  const m = (await getDictionary(locale)).admin.stock.messages;

  if (!variantId) return { status: "error", message: m.generic };
  const delta = wholeNumber(str(form, "delta"));
  if (!Number.isInteger(delta) || delta === 0 || Math.abs(delta) > MAX_QUANTITY) return { status: "error", message: m.invalidDelta };
  const reason = str(form, "reason");
  if (!reason) return { status: "error", message: m.reasonRequired };
  if (reason.length > MAX_REASON) return { status: "error", message: m.reasonTooLong };

  try {
    await adjustAdminStock(token, variantId, { delta, reason });
  } catch (e) {
    return { status: "error", message: errorMessage(e, m) };
  }
  return done(locale, variantId);
}

function errorMessage(e: unknown, m: Messages): string {
  if (e instanceof ApiError) {
    if (e.status === 409) return m.belowReserved;
    if (e.status === 404) return m.notFound;
    if (e.status === 422 || e.status === 400) return m.invalidQuantity;
  }
  return m.generic;
}
