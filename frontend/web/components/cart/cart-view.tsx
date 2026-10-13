"use client";

import { Lock, RefreshCw, ShieldCheck } from "lucide-react";
import { Link } from "@/components/ui/link";
import { useStore } from "./store";
import { CartLines } from "./cart-lines";
import { OrderSummary } from "./order-summary";
import { useDict } from "@/lib/i18n/provider";

export function CartView() {
  const d = useDict();
  const c = d.cart;
  const { items, hydrated } = useStore();

  if (hydrated && items.length === 0) {
    return (
      <div className="py-24 text-center">
        <p className="headline-lg">{c.emptyPageTitle}</p>
        <p className="mx-auto mt-4 max-w-md text-umber">{c.emptyText}</p>
        <div className="mt-10 flex flex-col justify-center gap-3 sm:flex-row">
          <Link href="/collection" className="btn btn-primary">{c.discoverCollection}</Link>
          <Link href="/account/wishlist" className="btn btn-secondary">{c.myWishlist}</Link>
        </div>
      </div>
    );
  }

  const icons = [ShieldCheck, RefreshCw, Lock];

  return (
    <div className="grid gap-12 lg:grid-cols-12">
      <div className="lg:col-span-8">
        <CartLines />
        <div className="mt-8 flex flex-wrap items-center justify-between gap-4">
          <Link href="/collection" className="link-underline">{c.continueShopping}</Link>
          <ul className="flex flex-wrap gap-x-6 gap-y-2 text-[12px] text-umber">
            {c.trust.map((t, i) => {
              const Icon = icons[i];
              return (
                <li key={t} className="flex items-center gap-2"><Icon size={14} strokeWidth={1.4} /> {t}</li>
              );
            })}
          </ul>
        </div>
      </div>
      <aside className="lg:col-span-4">
        <OrderSummary className="lg:sticky lg:top-24">
          <Link href="/checkout" className="btn btn-primary mt-6 w-full">{c.secureCheckout}</Link>
          <p className="mt-4 text-center text-[12px] text-umber">{c.deliveryNote}</p>
        </OrderSummary>
      </aside>
    </div>
  );
}
