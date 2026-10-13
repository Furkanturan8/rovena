"use client";

import { Minus, Plus } from "lucide-react";
import { Link } from "@/components/ui/link";
import { useStore } from "./store";
import { useCartLines } from "./use-cart-lines";
import { Photo } from "@/components/ui/photo";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { formatPrice } from "@/lib/utils";

export function CartLines() {
  const d = useDict();
  const locale = useLocale();
  const { setQty, remove } = useStore();
  const { lines, ready } = useCartLines();
  if (!ready) return <div className="h-64 animate-pulse bg-sand" />;

  return (
    <ul className="divide-y divide-travertine border-y border-travertine">
      {lines.map(({ item: i, product: p, lineTotal }) => {
        const lead = p.madeToOrderWeeks
          ? d.product.madeToOrder(p.madeToOrderWeeks)
          : d.product.leadTime(p.leadTimeDays?.[0] ?? 2, p.leadTimeDays?.[1] ?? 4);
        return (
          <li key={i.variantId} className="grid grid-cols-[96px_1fr] gap-5 py-6 md:grid-cols-[140px_1fr_auto]">
            <Link href={`/product/${p.slug}`}>
              <Photo src={p.images[0]} alt={p.name} sizes="140px" imgClassName="object-top" />
            </Link>
            <div className="flex flex-col">
              <p className="label-caps text-[10px] text-umber">{p.kind}</p>
              <Link href={`/product/${p.slug}`} className="headline-sm mt-1 hover:text-bronze-deep">{p.name}</Link>
              <p className="mt-2 text-[13px] text-umber">{d.colors[i.color] ?? i.color} · {d.cart.sizeLabel(d.sizes[i.size] ?? i.size)}</p>
              <p className="mt-1 text-[12px] text-umber">{d.cart.delivery(lead)}</p>
              <div className="mt-auto flex items-center gap-5 pt-4">
                <div className="flex items-center border border-travertine">
                  <button aria-label={d.cart.decrease} className="p-2.5 hover:bg-sand" onClick={() => setQty(i.variantId, i.qty - 1)}><Minus size={13} /></button>
                  <span className="w-8 text-center text-[13px]">{i.qty}</span>
                  <button aria-label={d.cart.increase} className="p-2.5 hover:bg-sand" onClick={() => setQty(i.variantId, i.qty + 1)}><Plus size={13} /></button>
                </div>
                <button onClick={() => remove(i.variantId)} className="text-[12px] text-umber underline underline-offset-4 hover:text-ink">{d.common.remove}</button>
              </div>
            </div>
            <p className="label-mono col-span-2 text-right md:col-span-1">{formatPrice(lineTotal, locale)}</p>
          </li>
        );
      })}
    </ul>
  );
}
