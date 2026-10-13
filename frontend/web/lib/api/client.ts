import type { ApiPage, ApiProduct, ApiEnvelope } from "./types";

// Browser-side reads of the public catalog API. Next proxies /api/v1/(products|categories|stock) to the gateway.

async function get<T>(path: string): Promise<T | null> {
  try {
    const res = await fetch(`/api/v1${path}`, { headers: { Accept: "application/json" } });
    if (!res.ok) return null;
    const json = (await res.json()) as ApiEnvelope<T>;
    return json.success ? json.data : null;
  } catch {
    return null;
  }
}

const byId = new Map<string, Promise<ApiProduct | null>>();

/** One product by id or slug; answers are remembered for the page's lifetime. */
export function fetchProduct(idOrSlug: string): Promise<ApiProduct | null> {
  let hit = byId.get(idOrSlug);
  if (!hit) {
    hit = get<ApiProduct>(`/products/${encodeURIComponent(idOrSlug)}`);
    // A failed lookup must not stick: allow a retry on the next render.
    hit.then((p) => !p && byId.delete(idOrSlug));
    byId.set(idOrSlug, hit);
  }
  return hit;
}

export async function searchProducts(q: string, limit = 4): Promise<ApiProduct[]> {
  const page = await get<ApiPage<ApiProduct>>(`/products?q=${encodeURIComponent(q)}&page_size=${limit}`);
  return page?.items ?? [];
}
