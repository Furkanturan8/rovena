"use client";

import { Link } from "@/components/ui/link";
import { useStore } from "@/components/cart/store";
import { ProductCard } from "@/components/product/product-card";
import { useProducts } from "@/hooks/use-products";
import { useDict } from "@/lib/i18n/provider";

export function WishlistGrid() {
  const d = useDict();
  const { wishlist, hydrated } = useStore();
  const { products, ready } = useProducts(wishlist);
  const items = wishlist.flatMap((id) => products.get(id) ?? []);

  if (!hydrated || !ready) return <div className="h-80 animate-pulse bg-sand" />;
  if (items.length === 0) {
    return (
      <div className="border border-travertine py-20 text-center">
        <p className="headline-md">{d.account.wishlist.emptyTitle}</p>
        <p className="mx-auto mt-3 max-w-sm text-umber">{d.account.wishlist.emptyText}</p>
        <Link href="/collection" className="btn btn-primary mt-8">{d.cart.discoverCollection}</Link>
      </div>
    );
  }
  return (
    <div className="grid grid-cols-2 gap-x-4 gap-y-10 md:grid-cols-3 md:gap-x-6">
      {items.map((p) => (
        <ProductCard key={p.id} product={p} sizes="(min-width:768px) 25vw, 50vw" />
      ))}
    </div>
  );
}
