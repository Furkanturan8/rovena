import "server-only";
import { cache } from "react";
import { apiFetch, ApiError } from "@/lib/api/server";
import { toCategory, toProduct, variantIds } from "@/lib/api/mappers";
import type { ApiAvailability, ApiCategory, ApiPage, ApiProduct } from "@/lib/api/types";
import type { Locale } from "@/lib/i18n/config";
import type { Category, Product } from "@/types";

const PAGE_SIZE = 100; // gateway maximum
const STOCK_CHUNK = 100; // /stock accepts at most 100 variant ids

/** Every active product, paged through the public listing endpoint. */
const fetchAllProducts = cache(async (): Promise<ApiProduct[]> => {
  const all: ApiProduct[] = [];
  for (let page = 1; ; page++) {
    const res = await apiFetch<ApiPage<ApiProduct>>(`/products?page=${page}&page_size=${PAGE_SIZE}&sort=newest`, { revalidate: 30 });
    all.push(...res.items);
    if (all.length >= res.total || res.items.length === 0) return all;
  }
});

const fetchCategories = cache(() => apiFetch<ApiCategory[]>("/categories", { revalidate: 60 }));

/** Sellable units per variant id. A failed stock lookup degrades to "unknown" (empty map). */
async function fetchStock(ids: string[], revalidate?: number): Promise<Record<string, number>> {
  const out: Record<string, number> = {};
  for (let i = 0; i < ids.length; i += STOCK_CHUNK) {
    const chunk = ids.slice(i, i + STOCK_CHUNK);
    const items = await apiFetch<ApiAvailability[]>(`/stock?variant_ids=${chunk.join(",")}`, { revalidate });
    for (const a of items) out[a.variant_id] = a.available;
  }
  return out;
}

const getStockForAll = cache(async (): Promise<Record<string, number>> => fetchStock(variantIds(await fetchAllProducts()), 10));

export interface Catalog {
  products: Product[];
  categories: Category[];
}

/** The whole storefront catalog in one language: products with stock, and categories with a cover photo. */
export const getCatalog = cache(async (locale: Locale): Promise<Catalog> => {
  const [apiProducts, apiCategories, stock] = await Promise.all([fetchAllProducts(), fetchCategories(), getStockForAll()]);
  const slugById = new Map(apiCategories.map((c) => [c.id, c.slug]));
  const products = apiProducts.map((p) => toProduct(p, locale, slugById.get(p.category_id) ?? "", stock));
  const categories = apiCategories.map((c) => toCategory(c, locale, products.find((p) => p.category === c.slug)?.images[0] ?? ""));
  return { products, categories };
});

/** Category list for navigation; the site stays usable when the catalog service is down. */
export async function getCategoriesSafe(locale: Locale): Promise<Category[]> {
  try {
    return (await getCatalog(locale)).categories;
  } catch {
    return [];
  }
}

/** One product by slug (or id) with live stock; null when it does not exist. */
export async function getProduct(slugOrId: string, locale: Locale): Promise<Product | null> {
  try {
    const [p, cats] = await Promise.all([apiFetch<ApiProduct>(`/products/${encodeURIComponent(slugOrId)}`, { revalidate: 30 }), fetchCategories()]);
    const stock = await fetchStock(variantIds([p]));
    return toProduct(p, locale, cats.find((c) => c.id === p.category_id)?.slug ?? "", stock);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

export async function getProductSlugs(): Promise<string[]> {
  try {
    return (await fetchAllProducts()).map((p) => p.slug);
  } catch {
    return [];
  }
}
