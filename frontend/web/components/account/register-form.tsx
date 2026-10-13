"use client";

import { useRouter } from "next/navigation";
import { useDict, useLocale } from "@/lib/i18n/provider";

export function RegisterForm() {
  const d = useDict();
  const locale = useLocale();
  const t = d.auth.register;
  const router = useRouter();
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        router.push(`/${locale}/account`);
      }}
      className="space-y-5"
    >
      <div className="grid gap-5 sm:grid-cols-2">
        <div><label className="field-label" htmlFor="fn">{t.firstName}</label><input id="fn" required className="field" autoComplete="given-name" /></div>
        <div><label className="field-label" htmlFor="ln">{t.lastName}</label><input id="ln" required className="field" autoComplete="family-name" /></div>
      </div>
      <div><label className="field-label" htmlFor="em">{t.email}</label><input id="em" type="email" required className="field" autoComplete="email" /></div>
      <div><label className="field-label" htmlFor="ph">{t.phone}</label><input id="ph" type="tel" className="field" autoComplete="tel" placeholder="+90" /></div>
      <div>
        <label className="field-label" htmlFor="pw">{t.password}</label>
        <input id="pw" type="password" required minLength={8} className="field" autoComplete="new-password" />
        <p className="mt-2 text-[12px] text-umber">{t.passwordHint}</p>
      </div>
      <label className="flex cursor-pointer items-start gap-3 text-[13px] leading-5">
        <input type="checkbox" required className="mt-0.5 h-4 w-4 shrink-0 appearance-none border border-ink checked:bg-ink" />
        <span>{t.terms}</span>
      </label>
      <label className="flex cursor-pointer items-start gap-3 text-[13px] leading-5">
        <input type="checkbox" className="mt-0.5 h-4 w-4 shrink-0 appearance-none border border-ink checked:bg-ink" />
        <span>{t.newsletter}</span>
      </label>
      <button className="btn btn-primary w-full">{t.submit}</button>
    </form>
  );
}
