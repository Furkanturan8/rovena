import "server-only";
import { apiFetch, ApiError } from "@/lib/api/server";
import type { ApiOrder, ApiPage } from "@/lib/api/types";

export async function getMyOrders(token: string): Promise<ApiOrder[]> {
  const page = await apiFetch<ApiPage<ApiOrder>>("/orders?page_size=50", { token });
  return page.items;
}

/** One of the customer's orders with its status history; null when it does not exist (or is someone else's). */
export async function getMyOrder(token: string, id: string): Promise<ApiOrder | null> {
  try {
    return await apiFetch<ApiOrder>(`/orders/${encodeURIComponent(id)}`, { token });
  } catch (e) {
    if (e instanceof ApiError && (e.status === 404 || e.status === 422 || e.status === 400)) return null;
    throw e;
  }
}
