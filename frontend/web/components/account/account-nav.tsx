"use client";

import { usePathname } from "next/navigation";
import { Link } from "@/components/ui/link";
import { splitLocale } from "@/lib/i18n/config";
import { useDict } from "@/lib/i18n/provider";
import { logoutAction } from "@/lib/auth/actions";
import { useLocale } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export function AccountNav() {
  const d = useDict();
  const locale = useLocale();
  const n = d.account.nav;
  const pathname = splitLocale(usePathname()).rest;
  const links = [
    { href: "/account", label: n.overview, exact: true },
    { href: "/account/orders", label: n.orders },
    { href: "/account/wishlist", label: n.wishlist },
    { href: "/account/addresses", label: n.addresses },
    { href: "/account/profile", label: n.profile },
  ];
  return (
    <nav aria-label={n.label} className="no-scrollbar -mx-5 flex gap-1 overflow-x-auto px-5 lg:mx-0 lg:flex-col lg:gap-0 lg:overflow-visible lg:px-0">
      {links.map((l) => {
        const active = l.exact ? pathname === l.href : pathname.startsWith(l.href);
        return (
          <Link
            key={l.href}
            href={l.href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "label-caps shrink-0 border px-4 py-3 text-[10px] transition-colors lg:border-0 lg:border-l-2 lg:px-5 lg:py-3.5",
              active ? "border-ink bg-ink text-canvas lg:border-ink lg:bg-transparent lg:text-ink" : "border-travertine text-umber hover:text-ink lg:border-transparent",
            )}
          >
            {l.label}
          </Link>
        );
      })}
      <form action={logoutAction} className="shrink-0 lg:mt-6">
        <input type="hidden" name="locale" value={locale} />
        <button className="label-caps border border-travertine px-4 py-3 text-[10px] text-umber hover:text-ink lg:border-0 lg:px-5">{n.logout}</button>
      </form>
    </nav>
  );
}
