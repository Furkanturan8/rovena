import "server-only";
import { ApiError, apiFetch } from "@/lib/api/server";
import type { ApiAdminProduct, ApiCategory, ApiPage, ApiProductStatus, ApiVariant } from "@/lib/api/types";
import type { Locale } from "@/lib/i18n/config";

export interface AdminProductFilters {
  status?: ApiProductStatus;
  q?: string;
  category?: string;
  sort?: "newest" | "price_asc" | "price_desc" | "name";
  page?: number;
  pageSize?: number;
}

export interface CategoryOption {
  id: string;
  label: string;
}

/** Categories as a flat, tree-ordered select list; children are indented with a dash prefix. */
export async function getCategoryOptions(locale: Locale): Promise<CategoryOption[]> {
  const all = await apiFetch<ApiCategory[]>("/categories");
  const byParent = new Map<string | null, ApiCategory[]>();
  for (const c of all) {
    const list = byParent.get(c.parent_id) ?? [];
    list.push(c);
    byParent.set(c.parent_id, list);
  }
  const out: CategoryOption[] = [];
  const walk = (parent: string | null, depth: number, seen: Set<string>) => {
    for (const c of byParent.get(parent) ?? []) {
      if (seen.has(c.id)) continue;
      seen.add(c.id);
      out.push({ id: c.id, label: `${"— ".repeat(depth)}${c.translations?.[locale]?.name || c.name}` });
      walk(c.id, depth + 1, seen);
    }
  };
  walk(null, 0, new Set());
  return out;
}

export interface CreateProductBody {
  name: string;
  brand: string;
  description: string;
  category_id: string;
  status: ApiProductStatus;
  translations?: Record<string, { name?: string; description?: string }>;
  attributes?: Record<string, string | number>;
  images: { url: string; alt: string }[];
  variants: { sku: string; size: string; color: string; price_cents: number }[];
}

/** Creates a product together with its images and variants in one transaction. */
export async function createAdminProduct(token: string, body: CreateProductBody): Promise<ApiAdminProduct> {
  return apiFetch<ApiAdminProduct>("/admin/products", { method: "POST", token, body });
}

export type UpdateProductBody = Omit<CreateProductBody, "variants">;

/** One product of any status; null when it does not exist (or the id is malformed). */
export async function getAdminProduct(token: string, id: string): Promise<ApiAdminProduct | null> {
  try {
    return await apiFetch<ApiAdminProduct>(`/admin/products/${encodeURIComponent(id)}`, { token });
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 400)) return null;
    throw e;
  }
}

/** Updates a product's fields; `images`, when sent, replace the whole list. The slug never changes. */
export async function updateAdminProduct(token: string, id: string, body: UpdateProductBody): Promise<ApiAdminProduct> {
  return apiFetch<ApiAdminProduct>(`/admin/products/${encodeURIComponent(id)}`, { method: "PATCH", token, body });
}

export interface CreateVariantBody {
  sku: string;
  size: string;
  color: string;
  price_cents: number;
}

export interface UpdateVariantBody {
  size?: string;
  color?: string;
  price_cents?: number;
  active?: boolean;
}

/** Adds a variant to an existing product. */
export async function createAdminVariant(token: string, productId: string, body: CreateVariantBody): Promise<ApiVariant> {
  return apiFetch<ApiVariant>(`/admin/products/${encodeURIComponent(productId)}/variants`, { method: "POST", token, body });
}

/** Updates size, color, price or the active flag of a variant. The SKU never changes. */
export async function updateAdminVariant(token: string, productId: string, variantId: string, body: UpdateVariantBody): Promise<ApiVariant> {
  return apiFetch<ApiVariant>(`/admin/products/${encodeURIComponent(productId)}/variants/${encodeURIComponent(variantId)}`, { method: "PATCH", token, body });
}

/** One page of products of every status (draft and archived included). */
export async function getAdminProducts(token: string, filters: AdminProductFilters = {}): Promise<ApiPage<ApiAdminProduct>> {
  const qs = new URLSearchParams();
  if (filters.status) qs.set("status", filters.status);
  if (filters.q) qs.set("q", filters.q);
  if (filters.category) qs.set("category", filters.category);
  if (filters.sort) qs.set("sort", filters.sort);
  if (filters.page) qs.set("page", String(filters.page));
  qs.set("page_size", String(filters.pageSize ?? 20));
  return apiFetch<ApiPage<ApiAdminProduct>>(`/admin/products?${qs}`, { token });
}
