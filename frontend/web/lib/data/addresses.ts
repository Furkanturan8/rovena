import "server-only";
import { apiFetch } from "@/lib/api/server";
import type { ApiAddress } from "@/lib/api/types";

/** The signed-in customer's saved addresses, default first. */
export async function getMyAddresses(token: string): Promise<ApiAddress[]> {
  return apiFetch<ApiAddress[]>("/users/me/addresses", { token });
}
