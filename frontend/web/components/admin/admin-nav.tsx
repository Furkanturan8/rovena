"use client";

import { usePathname } from "next/navigation";
import { Link } from "@/components/ui/link";
import { splitLocale } from "@/lib/i18n/config";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export function AdminNav() {
  const n = useDict().admin.nav;
  const pathname = splitLocale(usePathname()).rest;
  const links = [
    { href: "/admin", label: n.overview, exact: true },
    { href: "/admin/orders", label: n.orders },
    { href: "/admin/products", label: n.products },
    { href: "/admin/stock", label: n.stock },
    { href: "/admin/users", label: n.users },
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
    </nav>
  );
}
