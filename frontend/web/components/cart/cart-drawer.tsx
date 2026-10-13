"use client";

import { useEffect } from "react";
import { Minus, Plus, X } from "lucide-react";
import { Link } from "@/components/ui/link";
import { useStore } from "./store";
import { getProductById } from "@/lib/mock/catalog";
import { Photo } from "@/components/ui/photo";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn, formatPrice } from "@/lib/utils";

export const FREE_SHIPPING_THRESHOLD = 25000;

export function CartDrawer() {
  const d = useDict();
  const locale = useLocale();
  const { drawerOpen, closeDrawer, items, subtotal, setQty, remove } = useStore();

  useEffect(() => {
    if (!drawerOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && closeDrawer();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [drawerOpen, closeDrawer]);

  const remaining = Math.max(0, FREE_SHIPPING_THRESHOLD - subtotal);
  const progress = Math.min(100, (subtotal / FREE_SHIPPING_THRESHOLD) * 100);
  const c = d.cart;

  return (
    <div className={cn("fixed inset-0 z-[60]", drawerOpen ? "visible" : "invisible")} aria-hidden={!drawerOpen}>
      <button
        aria-label={c.closeCart}
        onClick={closeDrawer}
        className={cn("absolute inset-0 bg-ink/40 transition-opacity duration-300", drawerOpen ? "opacity-100" : "opacity-0")}
      />
      <aside
        role="dialog"
        aria-label={c.eyebrow}
        className={cn(
          "absolute right-0 top-0 flex h-full w-full max-w-[460px] flex-col border-l border-travertine bg-canvas text-ink transition-transform duration-500 ease-[cubic-bezier(0.2,0.7,0.2,1)]",
          drawerOpen ? "translate-x-0" : "translate-x-full",
        )}
      >
        <div className="flex h-[72px] items-center justify-between border-b border-travertine px-6">
          <h2 className="headline-sm">{c.count(items.reduce((n, i) => n + i.qty, 0))}</h2>
          <button onClick={closeDrawer} aria-label={d.common.close} className="p-2">
            <X size={20} strokeWidth={1.4} />
          </button>
        </div>

        {items.length === 0 ? (
          <div className="flex flex-1 flex-col items-center justify-center px-8 text-center">
            <p className="headline-md">{c.emptyTitle}</p>
            <p className="mt-3 max-w-xs text-umber">{c.emptyText}</p>
            <Link href="/collection" onClick={closeDrawer} className="btn btn-primary mt-8">
              {c.discoverCollection}
            </Link>
          </div>
        ) : (
          <>
            <div className="border-b border-travertine px-6 py-4">
              <p className="text-[12px] text-umber">{remaining > 0 ? c.freeShippingRemaining(formatPrice(remaining, locale)) : c.freeShippingEarned}</p>
              <div className="mt-2 h-px bg-travertine">
                <div className="h-px bg-ink transition-all duration-500" style={{ width: `${progress}%` }} />
              </div>
            </div>
            <ul className="flex-1 divide-y divide-travertine overflow-y-auto px-6">
              {items.map((i) => {
                const p = getProductById(i.productId, locale);
                if (!p) return null;
                return (
                  <li key={`${i.productId}-${i.size}-${i.color}`} className="flex gap-4 py-5">
                    <Link href={`/product/${p.slug}`} onClick={closeDrawer} className="w-24 shrink-0">
                      <Photo src={p.images[0]} alt={p.name} sizes="100px" imgClassName="object-top" />
                    </Link>
                    <div className="flex flex-1 flex-col">
                      <p className="label-caps text-[10px] text-umber">{p.kind}</p>
                      <Link href={`/product/${p.slug}`} onClick={closeDrawer} className="font-serif text-[16px] leading-5">
                        {p.name}
                      </Link>
                      <p className="mt-1 text-[12px] text-umber">
                        {d.colors[i.color] ?? i.color} · {c.sizeLabel(d.sizes[i.size] ?? i.size)}
                      </p>
                      <div className="mt-auto flex items-center justify-between pt-3">
                        <div className="flex items-center border border-travertine">
                          <button aria-label={c.decrease} className="p-2 hover:bg-sand" onClick={() => setQty(i.productId, i.size, i.color, i.qty - 1)}>
                            <Minus size={12} />
                          </button>
                          <span className="w-7 text-center text-[12px]">{i.qty}</span>
                          <button aria-label={c.increase} className="p-2 hover:bg-sand" onClick={() => setQty(i.productId, i.size, i.color, i.qty + 1)}>
                            <Plus size={12} />
                          </button>
                        </div>
                        <p className="label-mono">{formatPrice(p.price * i.qty, locale)}</p>
                      </div>
                      <button onClick={() => remove(i.productId, i.size, i.color)} className="mt-2 self-start text-[11px] text-umber underline underline-offset-4 hover:text-ink">
                        {d.common.remove}
                      </button>
                    </div>
                  </li>
                );
              })}
            </ul>
            <div className="border-t border-travertine bg-paper px-6 py-6">
              <div className="flex items-baseline justify-between">
                <span className="label-caps text-umber">{c.subtotal}</span>
                <span className="headline-sm">{formatPrice(subtotal, locale)}</span>
              </div>
              <p className="mt-1 text-[12px] text-umber">{c.vatNote}</p>
              <Link href="/checkout" onClick={closeDrawer} className="btn btn-primary mt-5 w-full">
                {c.goCheckout}
              </Link>
              <Link href="/cart" onClick={closeDrawer} className="btn btn-secondary mt-3 w-full">
                {c.viewCart}
              </Link>
            </div>
          </>
        )}
      </aside>
    </div>
  );
}
