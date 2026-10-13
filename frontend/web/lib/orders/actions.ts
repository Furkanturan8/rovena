"use server";

import { randomUUID } from "node:crypto";
import { apiFetch, ApiError } from "@/lib/api/server";
import type { ApiAddress, ApiOrder } from "@/lib/api/types";
import { isLocale, defaultLocale } from "@/lib/i18n/config";
import { getDictionary } from "@/lib/i18n/get-dictionary";
import { getAccessToken } from "@/lib/auth/session";

export interface PlaceOrderInput {
  locale: string;
  items: { variantId: string; quantity: number }[];
  /** A payment provider token (never a card number). */
  paymentMethod: string;
  /** One of the customer's saved addresses; the order stores a copy of it. */
  addressId: string;
}

export type PlaceOrderResult = { ok: true; orderId: string } | { ok: false; reason: "auth" | "failed"; message?: string };

/**
 * Turns the browser's cart into the customer's server-side cart and checks out.
 * The order API prices the lines itself from the catalog; prices from the browser are never sent.
 */
export async function placeOrderAction(input: PlaceOrderInput): Promise<PlaceOrderResult> {
  const locale = isLocale(input.locale) ? input.locale : defaultLocale;
  const d = await getDictionary(locale);
  const token = await getAccessToken();
  if (!token) return { ok: false, reason: "auth" };
  if (input.items.length === 0) return { ok: false, reason: "failed", message: d.checkout.errors.emptyCart };

  try {
    const addresses = await apiFetch<ApiAddress[]>("/users/me/addresses", { token });
    const address = addresses.find((a) => a.id === input.addressId);
    if (!address) return { ok: false, reason: "failed", message: d.checkout.selectAddress };

    await apiFetch("/cart", { method: "DELETE", token });
    for (const item of input.items) {
      const quantity = Math.max(1, Math.min(99, Math.floor(item.quantity)));
      await apiFetch(`/cart/items/${encodeURIComponent(item.variantId)}`, { method: "PUT", token, body: { quantity } });
    }
    const order = await apiFetch<ApiOrder>("/checkout", {
      method: "POST",
      token,
      headers: { "Idempotency-Key": randomUUID() },
      body: {
        payment_method: input.paymentMethod,
        shipping_address: {
          full_name: address.full_name,
          phone: address.phone,
          line1: address.line1,
          line2: address.line2,
          district: address.district,
          city: address.city,
          postal_code: address.postal_code,
          country: address.country,
        },
      },
    });
    return { ok: true, orderId: order.id };
  } catch (e) {
    if (e instanceof ApiError) {
      if (e.status === 401) return { ok: false, reason: "auth" };
      if (e.status === 409) return { ok: false, reason: "failed", message: d.checkout.errors.itemUnavailable };
      if (e.status === 503 || e.status === 502) return { ok: false, reason: "failed", message: d.checkout.errors.unavailable };
    }
    return { ok: false, reason: "failed", message: d.checkout.errors.generic };
  }
}

/** Cancels an unpaid order (the order service releases the reserved stock). */
export async function cancelOrderAction(orderId: string, locale: string): Promise<{ ok: boolean }> {
  void locale;
  const token = await getAccessToken();
  if (!token) return { ok: false };
  try {
    await apiFetch(`/orders/${encodeURIComponent(orderId)}/cancel`, { method: "POST", token });
    return { ok: true };
  } catch {
    return { ok: false };
  }
}
