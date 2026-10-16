import "server-only";
import { apiFetch } from "@/lib/api/server";
import type { ApiAdminProduct, ApiPage, ApiProductStatus } from "@/lib/api/types";

export interface AdminProductFilters {
  status?: ApiProductStatus;
  q?: string;
  category?: string;
  sort?: "newest" | "price_asc" | "price_desc" | "name";
  page?: number;
  pageSize?: number;
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
