import type { Locale } from "@/lib/i18n/config";
import type { Category, ColorOption, Product, ProductTag, SizeOption } from "@/types";
import type { ApiCategory, ApiProduct } from "./types";

const TAGS: ProductTag[] = ["limited", "new", "sealed", "custom"];

export function toCategory(api: ApiCategory, locale: Locale, image = ""): Category {
  return { id: api.id, slug: api.slug, label: api.translations?.[locale]?.name ?? api.name, image };
}

/**
 * Resolves the catalog's per-language texts. The main columns (name, description) are Turkish;
 * `translations.<lang>` carries the rest and the other languages. Missing English text falls back to Turkish.
 */
export function toProduct(api: ApiProduct, locale: Locale, categorySlug: string, stock: Record<string, number> = {}): Product {
  const tr = api.translations?.tr ?? {};
  const t = api.translations?.[locale] ?? {};
  const a = api.attributes ?? {};

  const variants = (api.variants ?? []).filter((v) => v.active);
  const colors: ColorOption[] =
    a.colors && a.colors.length > 0
      ? a.colors
      : Array.from(new Set(variants.map((v) => v.color))).map((name) => ({ name, hex: "#9a948b" }));

  const sizeOrder: string[] = [];
  for (const v of variants) if (!sizeOrder.includes(v.size)) sizeOrder.push(v.size);
  const hasStock = Object.keys(stock).length > 0;
  const sizes: SizeOption[] = sizeOrder.map((label) => ({
    label,
    // Without a stock lookup (search suggestions) sizes are shown as available.
    stock: hasStock ? variants.filter((v) => v.size === label).reduce((n, v) => n + (stock[v.id] ?? 0), 0) : 1,
  }));

  const price = api.price_from_cents ?? Math.min(...variants.map((v) => v.price_cents));
  const tag = TAGS.find((x) => x === a.tag);

  return {
    id: api.id,
    slug: api.slug,
    category: categorySlug,
    name: t.name ?? api.name,
    kind: t.kind ?? tr.kind ?? "",
    description: t.description ?? api.description ?? "",
    material: t.material ?? tr.material ?? "",
    composition: t.composition ?? tr.composition ?? [],
    price: Math.round(price / 100),
    craftHours: a.craft_hours ?? 0,
    edition: a.edition,
    tag,
    colors,
    sizes,
    variants: variants.map((v) => ({ id: v.id, sku: v.sku, size: v.size, color: v.color, price: Math.round(v.price_cents / 100) })),
    stock,
    images: (api.images ?? []).map((i) => i.url),
    sku: a.sku_base ?? variants[0]?.sku ?? "",
    rating: a.rating ?? 0,
    reviewCount: a.review_count ?? 0,
    releasedAt: a.released_at ?? "",
    leadTimeDays: a.lead_time_days,
    madeToOrderWeeks: a.made_to_order_weeks,
    video: a.video === "sewing" || a.video === "wool" ? a.video : undefined,
  };
}

/** Variant ids of the given products, for one stock lookup. */
export function variantIds(products: ApiProduct[]): string[] {
  return products.flatMap((p) => (p.variants ?? []).filter((v) => v.active).map((v) => v.id));
}
