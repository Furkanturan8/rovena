import "server-only";
import { apiFetch, ApiError } from "@/lib/api/server";
import type {
  ApiAdjustStockRequest,
  ApiCreateStockRequest,
  ApiPage,
  ApiReceiveStockRequest,
  ApiStock,
  ApiStockMovement,
} from "@/lib/api/types";

export interface AdminStockFilters {
  /** Case-insensitive SKU prefix. */
  sku?: string;
  /** Low-stock report: only items whose available is less than this. */
  below?: number;
  sort?: "newest" | "sku" | "available_asc";
  page?: number;
  pageSize?: number;
}

export async function getAdminStock(token: string, filters: AdminStockFilters = {}): Promise<ApiPage<ApiStock>> {
  const qs = new URLSearchParams();
  if (filters.sku) qs.set("sku", filters.sku);
  if (filters.below) qs.set("below", String(filters.below));
  if (filters.sort) qs.set("sort", filters.sort);
  if (filters.page) qs.set("page", String(filters.page));
  qs.set("page_size", String(filters.pageSize ?? 20));
  return apiFetch<ApiPage<ApiStock>>(`/admin/stock?${qs}`, { token });
}

/** Stock of one variant; null when it is not tracked yet. */
export async function getAdminStockItem(token: string, variantId: string): Promise<ApiStock | null> {
  try {
    return await apiFetch<ApiStock>(`/admin/stock/${encodeURIComponent(variantId)}`, { token });
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 400)) return null;
    throw e;
  }
}

/** Ledger of on_hand changes, newest first; null when the variant is not tracked. */
export async function getAdminStockMovements(
  token: string,
  variantId: string,
  page = 1,
  pageSize = 20,
): Promise<ApiPage<ApiStockMovement> | null> {
  try {
    return await apiFetch<ApiPage<ApiStockMovement>>(
      `/admin/stock/${encodeURIComponent(variantId)}/movements?page=${page}&page_size=${pageSize}`,
      { token },
    );
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 400)) return null;
    throw e;
  }
}

/** Starts tracking a variant. Throws ApiError (409 already tracked, 422 invalid). */
export async function createAdminStock(token: string, input: ApiCreateStockRequest): Promise<ApiStock> {
  return apiFetch<ApiStock>("/admin/stock", { method: "POST", body: input, token });
}

/** Books stock that arrived at the warehouse (stock entry). */
export async function receiveAdminStock(token: string, variantId: string, input: ApiReceiveStockRequest): Promise<ApiStock> {
  return apiFetch<ApiStock>(`/admin/stock/${encodeURIComponent(variantId)}/receipts`, {
    method: "POST",
    body: input,
    token,
  });
}

/** Corrects on_hand by a signed delta with a mandatory reason. Throws ApiError 409 when it would drop below reserved. */
export async function adjustAdminStock(token: string, variantId: string, input: ApiAdjustStockRequest): Promise<ApiStock> {
  return apiFetch<ApiStock>(`/admin/stock/${encodeURIComponent(variantId)}/adjustments`, {
    method: "POST",
    body: input,
    token,
  });
}
