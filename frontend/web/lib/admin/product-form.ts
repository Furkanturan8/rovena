// Pure helpers shared by the product form (client) and the product server actions.
import type { ApiProductStatus } from "@/lib/api/types";

export const MAX_IMAGES = 10;
export const MAX_VARIANTS = 100;
export const MAX_ATTRIBUTES = 30;
export const PRODUCT_STATUSES: ApiProductStatus[] = ["draft", "active", "archived"];

export interface ProductPayload {
  name: string;
  brand: string;
  categoryId: string;
  description: string;
  status: ApiProductStatus;
  enName: string;
  enDescription: string;
  attributes: { key: string; value: string }[];
  images: { url: string; alt: string }[];
  variants: { sku: string; size: string; color: string; priceCents: number }[];
}

export interface ProductErrorMessages {
  nameInvalid: string;
  categoryRequired: string;
  tooLong: string;
  attributeKeyRequired: string;
  attributeKeyDuplicate: string;
  imageLimit: string;
  urlInvalid: string;
  variantRequired: string;
  variantLimit: string;
  skuInvalid: string;
  skuDuplicate: string;
  priceInvalid: string;
}

/** Field path to message, using the API's own paths (`variants[0].sku`, `images[1].url`). */
export type FieldErrors = Record<string, string>;

const SKU_PATTERN = /^[A-Za-z0-9_-]{2,64}$/;
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function isValidSku(sku: string): boolean {
  return SKU_PATTERN.test(sku);
}

export function isUuid(v: string): boolean {
  return UUID_PATTERN.test(v);
}

export function isHttpUrl(raw: string): boolean {
  if (raw.length === 0 || raw.length > 2048) return false;
  try {
    const u = new URL(raw);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}

/**
 * Converts a price typed in TL ("1.299,90", "1299.90", "450") to kuruş.
 * Returns null for anything that is not a positive amount with at most two decimals.
 */
export function tlToCents(raw: string): number | null {
  let s = raw.trim().replace(/\s/g, "");
  if (!s) return null;
  if (s.includes(",")) {
    // Turkish style: dots group thousands, the comma is the decimal separator.
    s = s.replace(/\./g, "").replace(",", ".");
  } else if ((s.match(/\./g) ?? []).length > 1) {
    s = s.replace(/\./g, "");
  }
  if (!/^\d{1,9}(\.\d{1,2})?$/.test(s)) return null;
  const [whole, frac = ""] = s.split(".");
  const cents = Number(whole) * 100 + Number(frac.padEnd(2, "0"));
  return cents >= 1 ? cents : null;
}

/** Formats kuruş as a plain TL string for an input ("89900" -> "899,00"). */
export function centsToTl(cents: number): string {
  const whole = Math.floor(cents / 100);
  const frac = String(cents % 100).padStart(2, "0");
  return `${whole},${frac}`;
}

/** Attribute values that look like plain numbers are stored as numbers (rating, craft_hours, ...). */
export function attributeValue(raw: string): string | number {
  return /^-?\d{1,9}(\.\d{1,4})?$/.test(raw) ? Number(raw) : raw;
}

/** In edit mode variants are managed elsewhere, so they are not validated. */
export function validateProduct(p: ProductPayload, m: ProductErrorMessages, mode: "create" | "edit" = "create"): FieldErrors {
  const e: FieldErrors = {};
  const name = p.name.trim();
  if (name.length < 2 || name.length > 200) e.name = m.nameInvalid;
  if (p.brand.length > 100) e.brand = m.tooLong;
  if (!isUuid(p.categoryId)) e.category_id = m.categoryRequired;
  if (p.description.length > 10000) e.description = m.tooLong;
  if (p.enName.length > 200) e["translations.en.name"] = m.tooLong;
  if (p.enDescription.length > 10000) e["translations.en.description"] = m.tooLong;

  const seenKeys = new Set<string>();
  p.attributes.forEach((a, i) => {
    const key = a.key.trim();
    if (!key && !a.value.trim()) return;
    if (!key) e[`attributes[${i}].key`] = m.attributeKeyRequired;
    else if (seenKeys.has(key.toLowerCase())) e[`attributes[${i}].key`] = m.attributeKeyDuplicate;
    else if (key.length > 64) e[`attributes[${i}].key`] = m.tooLong;
    seenKeys.add(key.toLowerCase());
    if (a.value.length > 500) e[`attributes[${i}].value`] = m.tooLong;
  });

  if (p.images.length > MAX_IMAGES) e.images = m.imageLimit;
  p.images.forEach((img, i) => {
    if (!isHttpUrl(img.url.trim())) e[`images[${i}].url`] = m.urlInvalid;
    if (img.alt.length > 200) e[`images[${i}].alt`] = m.tooLong;
  });

  if (p.variants.length === 0) e.variants = m.variantRequired;
  else if (p.variants.length > MAX_VARIANTS) e.variants = m.variantLimit;
  const seenSkus = new Set<string>();
  p.variants.forEach((v, i) => {
    const sku = v.sku.trim().toUpperCase();
    if (!SKU_PATTERN.test(sku)) e[`variants[${i}].sku`] = m.skuInvalid;
    else if (seenSkus.has(sku)) e[`variants[${i}].sku`] = m.skuDuplicate;
    seenSkus.add(sku);
    if (v.size.length > 32) e[`variants[${i}].size`] = m.tooLong;
    if (v.color.length > 32) e[`variants[${i}].color`] = m.tooLong;
    if (!Number.isInteger(v.priceCents) || v.priceCents < 1) e[`variants[${i}].price_cents`] = m.priceInvalid;
  });
  if (mode === "edit") {
    for (const key of Object.keys(e)) {
      if (key === "variants" || key.startsWith("variants[")) delete e[key];
    }
  }
  return e;
}
