"use client";

import { Link } from "@/components/ui/link";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export function Logo({ className, tone = "dark", motto = true }: { className?: string; tone?: "dark" | "light"; motto?: boolean }) {
  const d = useDict();
  return (
    <Link href="/" aria-label={`Rovena — ${d.brand.motto}`} className={cn("inline-flex flex-col items-center leading-none", className)}>
      <span className={cn("pl-[0.28em] font-serif text-[26px] tracking-[0.28em]", tone === "light" ? "text-canvas" : "text-ink")}>ROVENA</span>
      {motto && (
        <span className={cn("mt-1.5 pl-[0.34em] text-[8px] uppercase tracking-[0.34em] transition-colors", tone === "light" ? "text-canvas/65" : "text-umber")}>
          {d.brand.motto}
        </span>
      )}
    </Link>
  );
}
