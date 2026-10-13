"use client";

import { usePathname, useRouter } from "next/navigation";
import { locales, splitLocale, type Locale } from "@/lib/i18n/config";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export function LanguageSwitcher({ className, tone = "inherit" }: { className?: string; tone?: "inherit" | "light" }) {
  const locale = useLocale();
  const d = useDict();
  const pathname = usePathname();
  const router = useRouter();

  function change(target: Locale) {
    if (target === locale) return;
    const { rest } = splitLocale(pathname);
    router.push(`/${target}${rest === "/" ? "" : rest}${window.location.search}`);
  }

  return (
    <div className={cn("flex items-center gap-2", className)} role="group" aria-label={d.common.language}>
      {locales.map((l, i) => (
        <span key={l} className="flex items-center gap-2">
          {i > 0 && <span aria-hidden className={cn("h-3 w-px", tone === "light" ? "bg-canvas/30" : "bg-current opacity-30")} />}
          <button
            onClick={() => change(l)}
            aria-pressed={l === locale}
            lang={l}
            className={cn("label-caps text-[10px] transition-opacity", l === locale ? "opacity-100" : "opacity-50 hover:opacity-100")}
          >
            {l.toUpperCase()}
          </button>
        </span>
      ))}
    </div>
  );
}
