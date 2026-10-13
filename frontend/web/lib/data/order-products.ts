import "server-only";
import { apiFetch } from "@/lib/api/server";
import { toProduct } from "@/lib/api/mappers";
import type { ApiOrder, ApiProduct } from "@/lib/api/types";
import type { Locale } from "@/lib/i18n/config";
import type { Product } from "@/types";

/** The catalog products behind order lines (for photos and links), keyed by product id. Missing ones are left out. */
export async function productsForOrders(orders: ApiOrder[], locale: Locale): Promise<Map<string, Product>> {
  const ids = Array.from(new Set(orders.flatMap((o) => o.items.map((i) => i.product_id))));
  const found = await Promise.all(
    ids.map((id) =>
      apiFetch<ApiProduct>(`/products/${encodeURIComponent(id)}`, { revalidate: 60 })
        .then((p) => toProduct(p, locale, ""))
        .catch(() => null),
    ),
  );
  const map = new Map<string, Product>();
  for (const p of found) if (p) map.set(p.id, p);
  return map;
}
