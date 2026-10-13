"use client";

import { useActionState, useEffect, useRef } from "react";
import { changeEmailAction, changePasswordAction } from "@/lib/account/actions";
import type { FormState } from "@/lib/auth/actions";
import { useDict, useLocale } from "@/lib/i18n/provider";

export function CredentialsForms() {
  const d = useDict();
  const locale = useLocale();
  const t = d.account.profile;
  const [emailState, emailAction, emailPending] = useActionState<FormState, FormData>(changeEmailAction, {});
  const [pwState, pwAction, pwPending] = useActionState<FormState, FormData>(changePasswordAction, {});
  const emailForm = useRef<HTMLFormElement>(null);
  const pwForm = useRef<HTMLFormElement>(null);

  useEffect(() => {
    if (emailState.saved) emailForm.current?.reset();
  }, [emailState]);
  useEffect(() => {
    if (pwState.saved) pwForm.current?.reset();
  }, [pwState]);

  return (
    <>
      <form ref={emailForm} action={emailAction}>
        <input type="hidden" name="locale" value={locale} />
        <h2 className="headline-md mb-8">{t.emailTitle}</h2>
        <div className="grid gap-5 sm:grid-cols-2">
          <div><label className="field-label" htmlFor="new-email">{t.newEmail}</label><input id="new-email" name="email" type="email" required maxLength={254} autoComplete="email" className="field" /></div>
          <div><label className="field-label" htmlFor="email-pw">{t.currentPassword}</label><input id="email-pw" name="password" type="password" required autoComplete="current-password" className="field" /></div>
        </div>
        <button disabled={emailPending} className="btn btn-secondary mt-8">{t.updateEmail}</button>
        {emailState.saved && <p role="status" className="mt-4 text-[13px] text-success">{t.emailChanged}</p>}
        {emailState.error && <p role="alert" className="mt-4 text-[13px] text-danger">{emailState.error}</p>}
      </form>

      <form ref={pwForm} action={pwAction}>
        <input type="hidden" name="locale" value={locale} />
        <h2 className="headline-md mb-8">{t.passwordTitle}</h2>
        <div className="grid gap-5 sm:grid-cols-2">
          <div><label className="field-label" htmlFor="cur-pw">{t.currentPassword}</label><input id="cur-pw" name="current_password" type="password" required autoComplete="current-password" className="field" /></div>
          <div>
            <label className="field-label" htmlFor="new-pw">{t.newPassword}</label>
            <input id="new-pw" name="new_password" type="password" required minLength={8} maxLength={72} autoComplete="new-password" className="field" />
            <p className="mt-2 text-[12px] text-umber">{t.passwordHint}</p>
          </div>
        </div>
        <button disabled={pwPending} className="btn btn-secondary mt-8">{t.updatePassword}</button>
        {pwState.saved && <p role="status" className="mt-4 text-[13px] text-success">{t.passwordChanged}</p>}
        {pwState.error && <p role="alert" className="mt-4 text-[13px] text-danger">{pwState.error}</p>}
      </form>
    </>
  );
}
