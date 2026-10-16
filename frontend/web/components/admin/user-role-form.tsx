"use client";

import { useActionState } from "react";
import { changeUserRoleAction, type AdminUserActionState } from "@/lib/admin/user-actions";
import type { ApiRole } from "@/lib/api/types";
import { useDict, useLocale } from "@/lib/i18n/provider";

const idle: AdminUserActionState = { status: "idle" };

export function UserRoleForm({ userId, role }: { userId: string; role: ApiRole }) {
  const t = useDict().admin.users;
  const locale = useLocale();
  const [state, action, pending] = useActionState(changeUserRoleAction, idle);
  const next: ApiRole = role === "admin" ? "customer" : "admin";

  return (
    <form action={action} className="inline-flex flex-col items-end gap-1">
      <input type="hidden" name="locale" value={locale} />
      <input type="hidden" name="userId" value={userId} />
      <input type="hidden" name="role" value={next} />
      <button type="submit" disabled={pending} className="btn btn-secondary">
        {pending ? t.saving : next === "admin" ? t.makeAdmin : t.makeCustomer}
      </button>
      {state.status === "error" ? (
        <p role="alert" className="max-w-[260px] text-right text-[13px] text-red-700">{state.message}</p>
      ) : null}
      {state.status === "ok" ? <p role="status" className="text-[13px] text-bronze-deep">{t.saved}</p> : null}
    </form>
  );
}
