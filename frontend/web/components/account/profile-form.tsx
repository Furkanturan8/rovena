"use client";

import { useActionState } from "react";
import { updateProfileAction, type FormState } from "@/lib/auth/actions";
import { useDict, useLocale } from "@/lib/i18n/provider";

export function ProfileForm({ user }: { user: { firstName: string; lastName: string; email: string } }) {
  const d = useDict();
  const locale = useLocale();
  const t = d.account.profile;
  const [state, action, pending] = useActionState<FormState, FormData>(updateProfileAction, {});

  return (
    <form action={action}>
      <input type="hidden" name="locale" value={locale} />
      <h2 className="headline-md mb-8">{t.personal}</h2>
      <div className="grid gap-5 sm:grid-cols-2">
        <div><label className="field-label" htmlFor="fn">{t.firstName}</label><input id="fn" name="first_name" defaultValue={user.firstName} maxLength={100} className="field" /></div>
        <div><label className="field-label" htmlFor="ln">{t.lastName}</label><input id="ln" name="last_name" defaultValue={user.lastName} maxLength={100} className="field" /></div>
        <div className="sm:col-span-2">
          <label className="field-label" htmlFor="em">{t.email}</label>
          <input id="em" type="email" value={user.email} readOnly className="field bg-sand/60 text-umber" />
          <p className="mt-2 text-[12px] text-umber">{t.emailNote}</p>
        </div>
      </div>
      <button disabled={pending} className="btn btn-primary mt-8">{d.common.save}</button>
      {state.saved && <p role="status" className="mt-4 text-[13px] text-success">{t.saved}</p>}
      {state.error && <p role="alert" className="mt-4 text-[13px] text-danger">{state.error}</p>}
    </form>
  );
}
