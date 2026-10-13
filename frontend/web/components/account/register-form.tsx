"use client";

import { useActionState } from "react";
import { registerAction, type FormState } from "@/lib/auth/actions";
import { useDict, useLocale } from "@/lib/i18n/provider";

export function RegisterForm({ next }: { next?: string }) {
  const d = useDict();
  const locale = useLocale();
  const t = d.auth.register;
  const [state, action, pending] = useActionState<FormState, FormData>(registerAction, {});

  return (
    <form action={action} className="space-y-5">
      <input type="hidden" name="locale" value={locale} />
      {next && <input type="hidden" name="next" value={next} />}
      <div className="grid gap-5 sm:grid-cols-2">
        <div><label className="field-label" htmlFor="fn">{t.firstName}</label><input id="fn" name="first_name" required className="field" autoComplete="given-name" /></div>
        <div><label className="field-label" htmlFor="ln">{t.lastName}</label><input id="ln" name="last_name" required className="field" autoComplete="family-name" /></div>
      </div>
      <div><label className="field-label" htmlFor="em">{t.email}</label><input id="em" name="email" type="email" required className="field" autoComplete="email" /></div>
      <div>
        <label className="field-label" htmlFor="pw">{t.password}</label>
        <input id="pw" name="password" type="password" required minLength={8} maxLength={72} className="field" autoComplete="new-password" />
        <p className="mt-2 text-[12px] text-umber">{t.passwordHint}</p>
      </div>
      <label className="flex cursor-pointer items-start gap-3 text-[13px] leading-5">
        <input type="checkbox" required className="mt-0.5 h-4 w-4 shrink-0 appearance-none border border-ink checked:bg-ink" />
        <span>{t.terms}</span>
      </label>
      {state.error && <p role="alert" className="border border-danger/40 bg-danger/5 p-3 text-[13px] text-danger">{state.error}</p>}
      <button disabled={pending} className="btn btn-primary w-full">{t.submit}</button>
    </form>
  );
}
