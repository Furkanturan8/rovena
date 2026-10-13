"use client";

import { useCartLines } from "./use-cart-lines";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn, formatPrice } from "@/lib/utils";

/** The order API charges the catalog price of the items; delivery is free, so total = subtotal. */
export function OrderSummary({ children, className }: { children?: React.ReactNode; className?: string }) {
  const d = useDict();
  const locale = useLocale();
  const s = d.summary;
  const { subtotal, ready } = useCartLines();
  const money = (v: number) => formatPrice(v, locale);

  return (
    <div className={cn("border border-travertine bg-paper p-6 md:p-8", className)}>
      <h2 className="headline-sm">{s.title}</h2>
      <dl className="mt-6 space-y-3 text-[14px]">
        <div className="flex justify-between"><dt className="text-umber">{s.subtotal}</dt><dd>{ready ? money(subtotal) : "—"}</dd></div>
        <div className="flex justify-between"><dt className="text-umber">{s.shipping}</dt><dd>{d.common.free}</dd></div>
        <div className="flex items-baseline justify-between border-t border-travertine pt-4">
          <dt className="label-caps">{s.total}</dt>
          <dd className="font-serif text-[26px] leading-none">{ready ? money(subtotal) : "—"}</dd>
        </div>
        <p className="text-[11px] text-umber">{s.note}</p>
      </dl>
      {children}
    </div>
  );
}
