import "server-only";
import { apiFetch, ApiError } from "@/lib/api/server";
import type { ApiAdminOrder, ApiAdminOrderDetail, ApiOrderStatus, ApiPage, ApiShipRequest } from "@/lib/api/types";

export interface AdminOrderFilters {
  status?: ApiOrderStatus;
  customerId?: string;
  page?: number;
  pageSize?: number;
}

/** One page of every customer's orders, newest first. */
export async function getAdminOrders(token: string, filters: AdminOrderFilters = {}): Promise<ApiPage<ApiAdminOrder>> {
  const qs = new URLSearchParams();
  if (filters.status) qs.set("status", filters.status);
  if (filters.customerId) qs.set("customer_id", filters.customerId);
  if (filters.page) qs.set("page", String(filters.page));
  qs.set("page_size", String(filters.pageSize ?? 20));
  return apiFetch<ApiPage<ApiAdminOrder>>(`/admin/orders?${qs}`, { token });
}

/** Any order with its saga bookkeeping and full history; null when it does not exist. */
export async function getAdminOrder(token: string, id: string): Promise<ApiAdminOrderDetail | null> {
  try {
    return await apiFetch<ApiAdminOrderDetail>(`/admin/orders/${encodeURIComponent(id)}`, { token });
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 400)) return null;
    throw e;
  }
}

/** confirmed -> shipped. Throws ApiError (409 wrong state, 422 invalid carrier/tracking code). */
export async function shipAdminOrder(token: string, id: string, input: ApiShipRequest): Promise<ApiAdminOrder> {
  return apiFetch<ApiAdminOrder>(`/admin/orders/${encodeURIComponent(id)}/ship`, {
    method: "POST",
    body: input,
    token,
  });
}

/** shipped -> delivered. Throws ApiError (409 wrong state). */
export async function deliverAdminOrder(token: string, id: string): Promise<ApiAdminOrder> {
  return apiFetch<ApiAdminOrder>(`/admin/orders/${encodeURIComponent(id)}/deliver`, { method: "POST", token });
}
