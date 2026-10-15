import "server-only";
import { apiFetch } from "@/lib/api/server";
import type { ApiPage, ApiRole, ApiUser } from "@/lib/api/types";

/** One page of users, newest first. */
export async function getAdminUsers(token: string, page = 1, pageSize = 20): Promise<ApiPage<ApiUser>> {
  return apiFetch<ApiPage<ApiUser>>(`/users?page=${page}&page_size=${pageSize}`, { token });
}

/** Throws ApiError 409 when it would demote the last admin. */
export async function changeUserRole(token: string, id: string, role: ApiRole): Promise<ApiUser> {
  return apiFetch<ApiUser>(`/users/${encodeURIComponent(id)}/role`, { method: "PATCH", body: { role }, token });
}
