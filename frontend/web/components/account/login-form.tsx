"use client";

import { useActionState, useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { Link } from "@/components/ui/link";
import { loginAction, type FormState } from "@/lib/auth/actions";
import { useDict, useLocale } from "@/lib/i18n/provider";

export function LoginForm({ next }: { next?: string }) {
  const d = useDict();
  const locale = useLocale();
  const t = d.auth.login;
  const [show, setShow] = useState(false);
  const [state, action, pending] = useActionState<FormState, FormData>(loginAction, {});

  return (
    <form action={action} className="space-y-5">
      <input type="hidden" name="locale" value={locale} />
      {next && <input type="hidden" name="next" value={next} />}
      <div>
        <label className="field-label" htmlFor="email">{t.email}</label>
        <input id="email" name="email" type="email" required autoComplete="email" className="field" placeholder={t.emailPlaceholder} />
      </div>
      <div>
        <div className="flex items-center justify-between">
          <label className="field-label" htmlFor="password">{t.password}</label>
          <Link href="#" className="mb-2 text-[12px] text-umber underline underline-offset-4 hover:text-ink">{t.forgot}</Link>
        </div>
        <div className="relative">
          <input id="password" name="password" type={show ? "text" : "password"} required autoComplete="current-password" className="field !pr-12" />
          <button type="button" onClick={() => setShow((s) => !s)} aria-label={show ? t.hidePassword : t.showPassword} className="absolute right-4 top-1/2 -translate-y-1/2 text-umber">
            {show ? <EyeOff size={16} /> : <Eye size={16} />}
          </button>
        </div>
      </div>
      {state.error && <p role="alert" className="border border-danger/40 bg-danger/5 p-3 text-[13px] text-danger">{state.error}</p>}
      <button disabled={pending} className="btn btn-primary w-full">{t.submit}</button>
    </form>
  );
}
