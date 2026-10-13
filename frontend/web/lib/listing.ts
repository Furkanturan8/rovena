import { getProducts, priceRange } from "@/lib/mock/catalog";
import type { Locale } from "@/lib/i18n/config";
import type { Product } from "@/types";

export type SortKey = "featured" | "newest" | "craft" | "price-desc" | "price-asc";
export const sortKeys: SortKey[] = ["featured", "newest", "craft", "price-desc", "price-asc"];

export interface ListingParams {
  category?: string;
  sort: SortKey;
  q?: string;
  tag?: string;
  size: string[];
  color: string[];
  min?: number;
  max?: number;
  page: number;
}

type Raw = Record<string, string | string[] | undefined>;

const list = (v: string | string[] | undefined) =>
  (Array.isArray(v) ? v.join(",") : (v ?? "")).split(",").map((s) => s.trim()).filter(Boolean);
const one = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v);
const num = (v: string | string[] | undefined) => {
  const raw = one(v);
  if (raw === undefined || raw === "") return undefined;
  const n = Number(raw);
  return Number.isFinite(n) ? n : undefined;
};

export function parseListingParams(raw: Raw): ListingParams {
  const sort = one(raw.sort) as SortKey;
  return {
    category: one(raw.category),
    sort: sortKeys.includes(sort) ? sort : "featured",
    q: one(raw.q),
    tag: one(raw.tag),
    size: list(raw.size),
    color: list(raw.color),
    min: num(raw.min),
    max: num(raw.max),
    page: Math.max(1, num(raw.page) ?? 1),
  };
}

export const PAGE_SIZE = 9;

export function filterProducts(p: ListingParams, locale: Locale, ignore?: "category"): Product[] {
  const q = p.q?.trim().toLocaleLowerCase(locale);
  const out = getProducts(locale).filter((x) => {
    if (ignore !== "category" && p.category && x.category !== p.category) return false;
    if (q && !`${x.name} ${x.kind} ${x.material} ${x.description}`.toLocaleLowerCase(locale).includes(q)) return false;
    if (p.tag === "limited" && x.tag !== "limited") return false;
    if (p.size.length && !x.sizes.some((s) => p.size.includes(s.label) && s.stock > 0)) return false;
    if (p.color.length && !x.colors.some((c) => p.color.includes(c.name))) return false;
    if (p.min !== undefined && x.price < p.min) return false;
    if (p.max !== undefined && x.price > p.max) return false;
    return true;
  });

  const by: Record<SortKey, (a: Product, b: Product) => number> = {
    featured: (a, b) => b.rating * b.reviewCount ** 0.3 - a.rating * a.reviewCount ** 0.3,
    newest: (a, b) => b.releasedAt.localeCompare(a.releasedAt),
    craft: (a, b) => b.craftHours - a.craftHours,
    "price-desc": (a, b) => b.price - a.price,
    "price-asc": (a, b) => a.price - b.price,
  };
  return [...out].sort(by[p.sort]);
}

export const allSizes = ["XS", "S", "M", "L", "XL", "34", "36", "38", "40", "42", "44", "46", "OS"];
export { priceRange };
