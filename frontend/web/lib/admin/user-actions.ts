"use server";

import { revalidatePath } from "next/cache";
import { ApiError } from "@/lib/api/server";
import type { ApiRole } from "@/lib/api/types";
import { requireAdmin } from "@/lib/auth/session";
import { changeUserRole } from "@/lib/data/admin-users";
import { defaultLocale, isLocale } from "@/lib/i18n/config";
import { getDictionary } from "@/lib/i18n/get-dictionary";

export type AdminUserActionState = { status: "idle" } | { status: "ok" } | { status: "error"; message: string };

const ROLES: ApiRole[] = ["customer", "admin"];

function str(form: FormData, key: string): string {
  const v = form.get(key);
  return typeof v === "string" ? v.trim() : "";
}

/** Changes a user's role. Re-checks the admin role on every call: server actions are public endpoints. */
export async function changeUserRoleAction(_prev: AdminUserActionState, form: FormData): Promise<AdminUserActionState> {
  const rawLocale = str(form, "locale");
  const locale = isLocale(rawLocale) ? rawLocale : defaultLocale;
  const { token } = await requireAdmin(locale, `/${locale}/admin/users`);
  const m = (await getDictionary(locale)).admin.users.messages;

  const userId = str(form, "userId");
  const role = ROLES.find((r) => r === str(form, "role"));
  if (!userId) return { status: "error", message: m.generic };
  if (!role) return { status: "error", message: m.invalid };

  try {
    await changeUserRole(token, userId, role);
  } catch (e) {
    if (e instanceof ApiError) {
      // 409: the backend refuses to demote the last remaining admin.
      if (e.status === 409) return { status: "error", message: m.lastAdmin };
      if (e.status === 404) return { status: "error", message: m.notFound };
      if (e.status === 400 || e.status === 422) return { status: "error", message: m.invalid };
    }
    return { status: "error", message: m.generic };
  }
  revalidatePath(`/${locale}/admin/users`);
  revalidatePath(`/${locale}/admin`);
  return { status: "ok" };
}
