"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { ApiError } from "@/lib/api/server";
import { requireAdmin } from "@/lib/auth/session";
import { createAdminProduct, type CreateProductBody } from "@/lib/data/admin-products";
import { defaultLocale, isLocale } from "@/lib/i18n/config";
import { getDictionary } from "@/lib/i18n/get-dictionary";
import { attributeValue, PRODUCT_STATUSES, validateProduct, type FieldErrors, type ProductPayload } from "./product-form";

/** `fields` carries the API's per-field validation messages (path -> message). */
export type AdminProductActionState = { status: "idle" } | { status: "error"; message: string; fields?: FieldErrors };

function str(form: FormData, key: string): string {
  const v = form.get(key);
  return typeof v === "string" ? v.trim() : "";
}

function text(v: unknown): string {
  return typeof v === "string" ? v : "";
}

/** Reads the JSON the form submits; null when it is not shaped like a product. */
function parsePayload(raw: string): ProductPayload | null {
  let data: unknown;
  try {
    data = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof data !== "object" || data === null) return null;
  const d = data as Record<string, unknown>;
  const rows = (v: unknown): Record<string, unknown>[] =>
    Array.isArray(v) ? v.filter((r): r is Record<string, unknown> => typeof r === "object" && r !== null) : [];
  const status = PRODUCT_STATUSES.find((s) => s === d.status);
  if (!status) return null;
  return {
    name: text(d.name).trim(),
    brand: text(d.brand).trim(),
    categoryId: text(d.categoryId).trim(),
    description: text(d.description).trim(),
    status,
    enName: text(d.enName).trim(),
    enDescription: text(d.enDescription).trim(),
    attributes: rows(d.attributes).map((r) => ({ key: text(r.key).trim(), value: text(r.value).trim() })),
    images: rows(d.images).map((r) => ({ url: text(r.url).trim(), alt: text(r.alt).trim() })),
    variants: rows(d.variants).map((r) => ({
      sku: text(r.sku).trim(),
      size: text(r.size).trim(),
      color: text(r.color).trim(),
      priceCents: typeof r.priceCents === "number" ? r.priceCents : NaN,
    })),
  };
}

function toBody(p: ProductPayload): CreateProductBody {
  const translations: NonNullable<CreateProductBody["translations"]> = {};
  if (p.enName || p.enDescription) {
    translations.en = {};
    if (p.enName) translations.en.name = p.enName;
    if (p.enDescription) translations.en.description = p.enDescription;
  }
  const attributes: Record<string, string | number> = {};
  for (const a of p.attributes) {
    if (a.key) attributes[a.key] = attributeValue(a.value);
  }
  return {
    name: p.name,
    brand: p.brand,
    description: p.description,
    category_id: p.categoryId,
    status: p.status,
    translations,
    attributes,
    images: p.images,
    variants: p.variants.map((v) => ({ sku: v.sku.toUpperCase(), size: v.size, color: v.color, price_cents: v.priceCents })),
  };
}

/**
 * Creates a product with its images and variants, then redirects to its page.
 * Re-checks the admin role on every call: server actions are public endpoints.
 */
export async function createProductAction(_prev: AdminProductActionState, form: FormData): Promise<AdminProductActionState> {
  const rawLocale = str(form, "locale");
  const locale = isLocale(rawLocale) ? rawLocale : defaultLocale;
  const { token } = await requireAdmin(locale, `/${locale}/admin/products/new`);
  const m = (await getDictionary(locale)).admin.productForm;

  const payload = parsePayload(str(form, "payload"));
  if (!payload) return { status: "error", message: m.messages.generic };
  const fields = validateProduct(payload, m.errors);
  if (Object.keys(fields).length > 0) return { status: "error", message: m.messages.fixFields, fields };

  let id: string;
  try {
    id = (await createAdminProduct(token, toBody(payload))).id;
  } catch (e) {
    if (e instanceof ApiError) {
      if (e.status === 422 && e.fields && Object.keys(e.fields).length > 0) {
        return { status: "error", message: m.messages.fixFields, fields: e.fields };
      }
      if (e.status === 409) {
        // The conflict response names no field; with a single variant the SKU is unambiguous.
        if (/sku/i.test(e.message) && payload.variants.length === 1) {
          return { status: "error", message: m.messages.fixFields, fields: { "variants[0].sku": m.messages.skuTaken } };
        }
        return { status: "error", message: /sku/i.test(e.message) ? m.messages.skuTakenSome : m.messages.conflict };
      }
      if (e.status === 404) return { status: "error", message: m.messages.categoryNotFound, fields: { category_id: m.messages.categoryNotFound } };
      if (e.status === 400) return { status: "error", message: m.messages.generic };
    }
    return { status: "error", message: m.messages.generic };
  }
  revalidatePath(`/${locale}/admin/products`);
  revalidatePath(`/${locale}/admin`);
  redirect(`/${locale}/admin/products/${id}`);
}
