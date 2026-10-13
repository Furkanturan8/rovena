"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { Link } from "@/components/ui/link";
import { useDict, useLocale } from "@/lib/i18n/provider";

export function LoginForm() {
  const d = useDict();
  const locale = useLocale();
  const t = d.auth.login;
  const router = useRouter();
  const [show, setShow] = useState(false);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        router.push(`/${locale}/account`);
      }}
      className="space-y-5"
    >
      <div>
        <label className="field-label" htmlFor="email">{t.email}</label>
        <input id="email" type="email" required autoComplete="email" className="field" placeholder={t.emailPlaceholder} />
      </div>
      <div>
        <div className="flex items-center justify-between">
          <label className="field-label" htmlFor="password">{t.password}</label>
          <Link href="#" className="mb-2 text-[12px] text-umber underline underline-offset-4 hover:text-ink">{t.forgot}</Link>
        </div>
        <div className="relative">
          <input id="password" type={show ? "text" : "password"} required autoComplete="current-password" className="field !pr-12" />
          <button type="button" onClick={() => setShow((s) => !s)} aria-label={show ? t.hidePassword : t.showPassword} className="absolute right-4 top-1/2 -translate-y-1/2 text-umber">
            {show ? <EyeOff size={16} /> : <Eye size={16} />}
          </button>
        </div>
      </div>
      <label className="flex cursor-pointer items-center gap-3 text-[13px]">
        <input type="checkbox" className="h-4 w-4 appearance-none border border-ink checked:bg-ink" /> {t.remember}
      </label>
      <button className="btn btn-primary w-full">{t.submit}</button>
    </form>
  );
}
